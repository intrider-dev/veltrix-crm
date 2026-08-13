package identity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

var (
	ErrMFARequired          = errors.New("MFA required")
	ErrInvalidMFA           = errors.New("invalid MFA code")
	ErrMFAUnavailable       = errors.New("MFA encryption is not configured")
	ErrInvalidResetToken    = errors.New("invalid or expired password reset token")
	ErrResetDeliveryMissing = errors.New("password reset delivery is not configured")
	ErrRegistrationDisabled = errors.New("development registration is disabled")
)

type MFAChallenge struct {
	Token     string
	ExpiresAt time.Time
}

type LoginResult struct {
	Session      *Session
	MFAChallenge *MFAChallenge
}

type DevelopmentRegistration struct {
	Email       string
	DisplayName string
	Password    string
	Locale      string
}

type MFASetup struct {
	Secret          string
	ProvisioningURI string
}

func (service *Service) RegisterDevelopmentUser(
	ctx context.Context,
	request DevelopmentRegistration,
) (dbgen.IdentityUser, error) {
	if !service.registrationEnabled {
		return dbgen.IdentityUser{}, ErrRegistrationDisabled
	}
	normalized, err := normalizeEmail(request.Email)
	if err != nil {
		return dbgen.IdentityUser{}, validation("/email", "validation.email")
	}
	displayName := strings.TrimSpace(request.DisplayName)
	if count := utf8.RuneCountInString(displayName); count < 1 || count > 160 {
		return dbgen.IdentityUser{}, validation("/displayName", "validation.length")
	}
	locale := strings.ToLower(strings.TrimSpace(request.Locale))
	if _, ok := service.supportedLocales[locale]; !ok {
		return dbgen.IdentityUser{}, validation("/locale", "validation.locale.unsupported")
	}
	if !passwordInputWithinBounds(request.Password) {
		return dbgen.IdentityUser{}, validation("/password", "validation.password.policy")
	}
	passwordHash, err := service.hasher.Hash(request.Password)
	if err != nil {
		return dbgen.IdentityUser{}, fmt.Errorf("hash registration password: %w", err)
	}
	userID, err := ids.NewV7()
	if err != nil {
		return dbgen.IdentityUser{}, err
	}
	user, err := service.queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID: userID.PG(), Email: strings.TrimSpace(request.Email), EmailNormalized: normalized,
		DisplayName: displayName, PasswordHash: passwordHash, PreferredLocale: locale,
	})
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return dbgen.IdentityUser{}, validation("/email", "validation.email.alreadyUsed")
		}
		return dbgen.IdentityUser{}, fmt.Errorf("create development user: %w", err)
	}
	return user, nil
}

func (service *Service) LogoutAll(ctx context.Context, userID ids.UUID) error {
	if err := service.queries.RevokeAllUserSessions(ctx, userID.PG()); err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	return nil
}

// PruneExpiredArtifacts is safe for a bounded periodic worker job.
func (service *Service) PruneExpiredArtifacts(ctx context.Context) error {
	if err := service.queries.DeleteExpiredMFAChallenges(ctx); err != nil {
		return fmt.Errorf("prune MFA challenges: %w", err)
	}
	if err := service.queries.DeleteExpiredPasswordResetTokens(ctx); err != nil {
		return fmt.Errorf("prune password reset tokens: %w", err)
	}
	if err := service.queries.DeleteExpiredSessions(ctx); err != nil {
		return fmt.Errorf("prune sessions: %w", err)
	}
	return nil
}

func (service *Service) ChangePassword(ctx context.Context, userID ids.UUID, currentPassword, newPassword string) error {
	if !passwordInputWithinBounds(currentPassword) {
		return errx.ErrInvalidCredentials
	}
	if !passwordInputWithinBounds(newPassword) {
		return validation("/newPassword", "validation.password.policy")
	}
	newHash, err := service.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	user, err := queries.GetUserByIDForPasswordChange(ctx, userID.PG())
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Status != "active") {
		return errx.ErrUnauthenticated
	}
	if err != nil {
		return fmt.Errorf("lock user for password change: %w", err)
	}
	valid, err := service.hasher.Verify(currentPassword, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify current password: %w", err)
	}
	if !valid {
		return errx.ErrInvalidCredentials
	}
	if _, err := queries.ReplacePasswordAndRevokeSessions(ctx, dbgen.ReplacePasswordAndRevokeSessionsParams{
		PasswordHash: newHash, TargetUserID: userID.PG(),
	}); err != nil {
		return fmt.Errorf("replace password: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}

// RequestPasswordReset intentionally returns nil for unknown or disabled users
// so the HTTP boundary can always return the same generic response.
func (service *Service) RequestPasswordReset(ctx context.Context, email string) error {
	if service.resetSender == nil || service.publicURL == "" {
		return ErrResetDeliveryMissing
	}
	normalized, normalizeErr := normalizeEmail(email)
	if normalizeErr != nil {
		_, _ = service.hasher.Verify("invalid-reset-request", service.dummyHash)
		return nil
	}
	user, err := service.queries.GetUserByNormalizedEmail(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		_, _ = service.hasher.Verify("unknown-reset-request", service.dummyHash)
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup reset user: %w", err)
	}
	if user.Status != "active" {
		return nil
	}
	rawToken, tokenHash, err := randomToken()
	if err != nil {
		return err
	}
	tokenID, err := ids.NewV7()
	if err != nil {
		return err
	}
	expiresAt := service.now().Add(service.passwordResetTTL)
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.DeleteActivePasswordResetTokens(ctx, user.ID); err != nil {
		return fmt.Errorf("invalidate reset tokens: %w", err)
	}
	if err := queries.CreatePasswordResetToken(ctx, dbgen.CreatePasswordResetTokenParams{
		ID: tokenID.PG(), UserID: user.ID, TokenHash: tokenHash[:],
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("create reset token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reset token: %w", err)
	}
	resetURL := service.publicURL + "/reset-password?token=" + url.QueryEscape(rawToken)
	deliveryErr := service.resetSender.SendPasswordReset(ctx, PasswordResetDelivery{
		Recipient: user.Email, Locale: user.PreferredLocale, TemplateKey: passwordResetTemplateKey,
		Params: map[string]string{
			"resetUrl":       resetURL,
			"expiresMinutes": strconv.FormatInt(int64(service.passwordResetTTL.Minutes()), 10),
		},
	})
	if deliveryErr != nil {
		// Do not leave an undisclosed usable token after a failed synchronous send.
		_ = service.queries.DeletePasswordResetToken(ctx, tokenID.PG())
		return fmt.Errorf("deliver password reset: %w", deliveryErr)
	}
	return nil
}

func (service *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	tokenHash, ok := decodeTokenHash(rawToken)
	if !ok {
		return ErrInvalidResetToken
	}
	if !passwordInputWithinBounds(newPassword) {
		return validation("/newPassword", "validation.password.policy")
	}
	newHash, err := service.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash reset password: %w", err)
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	reset, err := queries.LockPasswordResetToken(ctx, tokenHash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("lock reset token: %w", err)
	}
	consumed, err := queries.ConsumePasswordResetToken(ctx, reset.ID)
	if err != nil {
		return fmt.Errorf("consume reset token: %w", err)
	}
	if consumed != 1 {
		return ErrInvalidResetToken
	}
	if _, err := queries.ReplacePasswordAndRevokeSessions(ctx, dbgen.ReplacePasswordAndRevokeSessionsParams{
		PasswordHash: newHash, TargetUserID: reset.UserID,
	}); err != nil {
		return fmt.Errorf("replace reset password: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset: %w", err)
	}
	return nil
}

func (service *Service) BeginMFASetup(ctx context.Context, userID ids.UUID, currentPassword, account string) (MFASetup, error) {
	if service.secretCipher == nil {
		return MFASetup{}, ErrMFAUnavailable
	}
	if !passwordInputWithinBounds(currentPassword) {
		return MFASetup{}, errx.ErrInvalidCredentials
	}
	user, err := service.queries.GetUserByID(ctx, userID.PG())
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Status != "active") {
		return MFASetup{}, errx.ErrUnauthenticated
	}
	if err != nil {
		return MFASetup{}, fmt.Errorf("load user for MFA setup: %w", err)
	}
	valid, err := service.hasher.Verify(currentPassword, user.PasswordHash)
	if err != nil {
		return MFASetup{}, fmt.Errorf("verify password for MFA setup: %w", err)
	}
	if !valid {
		return MFASetup{}, errx.ErrInvalidCredentials
	}
	rawSecret, encodedSecret, err := newTOTPSecret()
	if err != nil {
		return MFASetup{}, err
	}
	defer clear(rawSecret)
	envelope, err := service.secretCipher.Encrypt(ctx, "totp", userID.String(), rawSecret)
	if err != nil {
		return MFASetup{}, fmt.Errorf("encrypt TOTP secret: %w", err)
	}
	if account == "" {
		account = user.Email
	}
	provisioningURI, err := buildTOTPUri(service.mfaIssuer, account, encodedSecret)
	if err != nil {
		return MFASetup{}, err
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MFASetup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	lockedUser, err := queries.GetUserByIDForPasswordChange(ctx, userID.PG())
	if err != nil || lockedUser.Status != "active" || lockedUser.PasswordHash != user.PasswordHash {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return MFASetup{}, fmt.Errorf("lock user for MFA setup: %w", err)
		}
		return MFASetup{}, errx.ErrUnauthenticated
	}
	if err := queries.UpsertPendingMFAConfiguration(ctx, dbgen.UpsertPendingMFAConfigurationParams{
		UserID: userID.PG(), SecretCiphertext: envelope.Ciphertext,
		SecretNonce: envelope.Nonce, KeyID: envelope.KeyID,
	}); err != nil {
		return MFASetup{}, fmt.Errorf("store pending MFA setup: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return MFASetup{}, fmt.Errorf("commit MFA setup: %w", err)
	}
	return MFASetup{Secret: encodedSecret, ProvisioningURI: provisioningURI}, nil
}

func (service *Service) ConfirmMFASetup(ctx context.Context, userID ids.UUID, code string) ([]string, error) {
	if service.secretCipher == nil {
		return nil, ErrMFAUnavailable
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	configuration, err := queries.LockMFAConfiguration(ctx, userID.PG())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidMFA
	}
	if err != nil {
		return nil, fmt.Errorf("lock pending MFA setup: %w", err)
	}
	if configuration.PendingKeyID == nil {
		return nil, ErrInvalidMFA
	}
	if !configuration.PendingCreatedAt.Valid ||
		configuration.PendingCreatedAt.Time.Add(service.mfaSetupTTL).Before(service.now()) {
		return nil, ErrInvalidMFA
	}
	secret, err := service.secretCipher.Decrypt(ctx, "totp", userID.String(), SecretEnvelope{
		Ciphertext: configuration.PendingSecretCiphertext,
		Nonce:      configuration.PendingSecretNonce,
		KeyID:      *configuration.PendingKeyID,
	})
	if err != nil {
		return nil, fmt.Errorf("decrypt pending TOTP secret: %w", err)
	}
	defer clear(secret)
	step, valid := matchTOTP(secret, strings.TrimSpace(code), service.now(), 1)
	if !valid {
		return nil, ErrInvalidMFA
	}
	codes, err := replaceRecoveryCodes(ctx, queries, userID)
	if err != nil {
		return nil, err
	}
	updated, err := queries.EnableMFAConfiguration(ctx, dbgen.EnableMFAConfigurationParams{
		UserID: userID.PG(), LastAcceptedStep: &step,
	})
	if err != nil {
		return nil, fmt.Errorf("enable MFA: %w", err)
	}
	if updated != 1 {
		return nil, ErrInvalidMFA
	}
	if err := queries.RevokeAllUserSessions(ctx, userID.PG()); err != nil {
		return nil, fmt.Errorf("revoke sessions after MFA enable: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit MFA setup: %w", err)
	}
	return codes, nil
}

func (service *Service) MFAEnabled(ctx context.Context, userID ids.UUID) (bool, error) {
	enabled, err := service.queries.IsMFAEnabled(ctx, userID.PG())
	if err != nil {
		return false, fmt.Errorf("load MFA status: %w", err)
	}
	return enabled, nil
}

func (service *Service) RegenerateRecoveryCodes(
	ctx context.Context,
	userID ids.UUID,
	currentPassword,
	secondFactor string,
) ([]string, error) {
	if !passwordInputWithinBounds(currentPassword) {
		return nil, errx.ErrInvalidCredentials
	}
	user, err := service.queries.GetUserByID(ctx, userID.PG())
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Status != "active") {
		return nil, errx.ErrUnauthenticated
	}
	if err != nil {
		return nil, fmt.Errorf("load user for recovery regeneration: %w", err)
	}
	validPassword, err := service.hasher.Verify(currentPassword, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verify password for recovery regeneration: %w", err)
	}
	if !validPassword {
		return nil, errx.ErrInvalidCredentials
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	lockedUser, err := queries.GetUserByIDForPasswordChange(ctx, userID.PG())
	if err != nil || lockedUser.PasswordHash != user.PasswordHash || lockedUser.Status != "active" {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("lock user for recovery regeneration: %w", err)
		}
		return nil, errx.ErrUnauthenticated
	}
	configuration, err := queries.LockMFAConfiguration(ctx, userID.PG())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidMFA
	}
	if err != nil {
		return nil, fmt.Errorf("lock MFA for recovery regeneration: %w", err)
	}
	if !configuration.EnabledAt.Valid {
		return nil, ErrInvalidMFA
	}
	validFactor, err := service.verifySecondFactor(ctx, queries, configuration, secondFactor)
	if err != nil {
		return nil, err
	}
	if !validFactor {
		return nil, ErrInvalidMFA
	}
	codes, err := replaceRecoveryCodes(ctx, queries, userID)
	if err != nil {
		return nil, err
	}
	if err := queries.RevokeAllUserSessions(ctx, userID.PG()); err != nil {
		return nil, fmt.Errorf("revoke sessions after recovery regeneration: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit recovery regeneration: %w", err)
	}
	return codes, nil
}

func (service *Service) CompleteMFALogin(ctx context.Context, challengeToken, code string) (Session, error) {
	tokenHash, ok := decodeTokenHash(challengeToken)
	if !ok {
		return Session{}, ErrInvalidMFA
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	challenge, err := queries.LockMFAChallenge(ctx, tokenHash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidMFA
	}
	if err != nil {
		return Session{}, fmt.Errorf("lock MFA challenge: %w", err)
	}
	configuration, err := queries.LockMFAConfiguration(ctx, challenge.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidMFA
	}
	if err != nil {
		return Session{}, fmt.Errorf("lock MFA configuration: %w", err)
	}
	if !configuration.EnabledAt.Valid {
		return Session{}, ErrInvalidMFA
	}
	valid, err := service.verifySecondFactor(ctx, queries, configuration, code)
	if err != nil {
		return Session{}, err
	}
	if !valid {
		if err := queries.RecordMFAChallengeFailure(ctx, challenge.ID); err != nil {
			return Session{}, fmt.Errorf("record MFA failure: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Session{}, fmt.Errorf("commit MFA failure: %w", err)
		}
		return Session{}, ErrInvalidMFA
	}
	consumed, err := queries.ConsumeMFAChallenge(ctx, challenge.ID)
	if err != nil || consumed != 1 {
		if err != nil {
			return Session{}, fmt.Errorf("consume MFA challenge: %w", err)
		}
		return Session{}, ErrInvalidMFA
	}
	user, err := queries.GetUserByID(ctx, challenge.UserID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Status != "active") {
		return Session{}, errx.ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("load MFA user: %w", err)
	}
	session, err := service.createSession(ctx, queries, user, challenge.UserAgent, challenge.IpAddress)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit MFA login: %w", err)
	}
	return session, nil
}

func (service *Service) DisableMFA(ctx context.Context, userID ids.UUID, currentPassword, code string) error {
	if !passwordInputWithinBounds(currentPassword) {
		return errx.ErrInvalidCredentials
	}
	user, err := service.queries.GetUserByID(ctx, userID.PG())
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Status != "active") {
		return errx.ErrUnauthenticated
	}
	if err != nil {
		return fmt.Errorf("load user to disable MFA: %w", err)
	}
	validPassword, err := service.hasher.Verify(currentPassword, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify password to disable MFA: %w", err)
	}
	if !validPassword {
		return errx.ErrInvalidCredentials
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	lockedUser, err := queries.GetUserByIDForPasswordChange(ctx, userID.PG())
	if err != nil || lockedUser.PasswordHash != user.PasswordHash || lockedUser.Status != "active" {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lock user to disable MFA: %w", err)
		}
		return errx.ErrUnauthenticated
	}
	configuration, err := queries.LockMFAConfiguration(ctx, userID.PG())
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidMFA
	}
	if err != nil {
		return fmt.Errorf("lock MFA to disable: %w", err)
	}
	if !configuration.EnabledAt.Valid {
		return ErrInvalidMFA
	}
	validFactor, err := service.verifySecondFactor(ctx, queries, configuration, code)
	if err != nil {
		return err
	}
	if !validFactor {
		return ErrInvalidMFA
	}
	if err := queries.DeleteRecoveryCodes(ctx, userID.PG()); err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	if err := queries.DisableMFAConfiguration(ctx, userID.PG()); err != nil {
		return fmt.Errorf("disable MFA: %w", err)
	}
	if err := queries.RevokeAllUserSessions(ctx, userID.PG()); err != nil {
		return fmt.Errorf("revoke sessions after MFA disable: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit MFA disable: %w", err)
	}
	return nil
}

func (service *Service) verifySecondFactor(
	ctx context.Context,
	queries *dbgen.Queries,
	configuration dbgen.IdentityMfaConfiguration,
	code string,
) (bool, error) {
	userID, ok := ids.FromPG(configuration.UserID)
	if !ok {
		return false, errors.New("invalid MFA user id")
	}
	trimmedCode := strings.TrimSpace(code)
	if len(trimmedCode) == totpDigits {
		if service.secretCipher == nil {
			return false, ErrMFAUnavailable
		}
		secret, err := service.secretCipher.Decrypt(ctx, "totp", userID.String(), SecretEnvelope{
			Ciphertext: configuration.SecretCiphertext,
			Nonce:      configuration.SecretNonce,
			KeyID:      configuration.KeyID,
		})
		if err != nil {
			return false, fmt.Errorf("decrypt TOTP secret: %w", err)
		}
		defer clear(secret)
		step, matches := matchTOTP(secret, trimmedCode, service.now(), 1)
		if !matches || (configuration.LastAcceptedStep != nil && step <= *configuration.LastAcceptedStep) {
			return false, nil
		}
		advanced, err := queries.AdvanceMFATimeStep(ctx, dbgen.AdvanceMFATimeStepParams{
			UserID: configuration.UserID, LastAcceptedStep: &step,
		})
		if err != nil {
			return false, fmt.Errorf("advance MFA replay step: %w", err)
		}
		return advanced == 1, nil
	}
	if len(normalizeRecoveryCode(trimmedCode)) != 16 {
		return false, nil
	}
	codeHash := hashRecoveryCode(trimmedCode)
	consumed, err := queries.ConsumeRecoveryCode(ctx, dbgen.ConsumeRecoveryCodeParams{
		UserID: configuration.UserID, CodeHash: codeHash[:],
	})
	if err != nil {
		return false, fmt.Errorf("consume recovery code: %w", err)
	}
	return consumed == 1, nil
}

func (service *Service) createMFAChallenge(
	ctx context.Context,
	queries *dbgen.Queries,
	user dbgen.IdentityUser,
	userAgent string,
	address *netip.Addr,
) (MFAChallenge, error) {
	rawToken, tokenHash, err := randomToken()
	if err != nil {
		return MFAChallenge{}, err
	}
	challengeID, err := ids.NewV7()
	if err != nil {
		return MFAChallenge{}, err
	}
	userAgent = boundedUserAgent(userAgent)
	expiresAt := service.now().Add(service.mfaChallengeTTL)
	if err := queries.CreateMFAChallenge(ctx, dbgen.CreateMFAChallengeParams{
		ID: challengeID.PG(), UserID: user.ID, TokenHash: tokenHash[:],
		UserAgent: userAgent, IpAddress: address,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return MFAChallenge{}, fmt.Errorf("create MFA challenge: %w", err)
	}
	return MFAChallenge{Token: rawToken, ExpiresAt: expiresAt}, nil
}

func decodeTokenHash(rawToken string) ([32]byte, bool) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256(decoded), true
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}

func passwordInputWithinBounds(password string) bool {
	return len(password) >= 8 && len(password) <= 1024
}

func replaceRecoveryCodes(ctx context.Context, queries *dbgen.Queries, userID ids.UUID) ([]string, error) {
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	if err := queries.DeleteRecoveryCodes(ctx, userID.PG()); err != nil {
		return nil, fmt.Errorf("delete prior recovery codes: %w", err)
	}
	for _, codeHash := range hashes {
		codeID, err := ids.NewV7()
		if err != nil {
			return nil, err
		}
		if err := queries.CreateRecoveryCode(ctx, dbgen.CreateRecoveryCodeParams{
			ID: codeID.PG(), UserID: userID.PG(), CodeHash: codeHash[:],
		}); err != nil {
			return nil, fmt.Errorf("store recovery code: %w", err)
		}
	}
	return codes, nil
}
