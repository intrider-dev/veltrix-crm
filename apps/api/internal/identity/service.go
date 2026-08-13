package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type Principal struct {
	SessionID       ids.UUID
	UserID          ids.UUID
	Email           string
	DisplayName     string
	PreferredLocale string
	CSRFHash        [32]byte
	ExpiresAt       time.Time
}

var localeTagPattern = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})*$`)

type Session struct {
	Principal Principal
	Token     string
	CSRFToken string
}

type Service struct {
	pool                *pgxpool.Pool
	queries             *dbgen.Queries
	hasher              *PasswordHasher
	dummyHash           string
	sessionTTL          time.Duration
	passwordResetTTL    time.Duration
	mfaChallengeTTL     time.Duration
	mfaSetupTTL         time.Duration
	registrationEnabled bool
	secretCipher        SecretCipher
	resetSender         PasswordResetSender
	publicURL           string
	mfaIssuer           string
	supportedLocales    map[string]struct{}
	now                 func() time.Time
}

func NewService(pool *pgxpool.Pool, hasher *PasswordHasher, sessionTTL time.Duration) (*Service, error) {
	return NewServiceWithOptions(pool, hasher, ServiceOptions{SessionTTL: sessionTTL})
}

type ServiceOptions struct {
	SessionTTL          time.Duration
	PasswordResetTTL    time.Duration
	MFAChallengeTTL     time.Duration
	MFASetupTTL         time.Duration
	RegistrationEnabled bool
	SecretCipher        SecretCipher
	ResetSender         PasswordResetSender
	PublicURL           string
	MFAIssuer           string
	SupportedLocales    []string
	Clock               func() time.Time
}

func NewServiceWithOptions(pool *pgxpool.Pool, hasher *PasswordHasher, options ServiceOptions) (*Service, error) {
	if pool == nil {
		return nil, errors.New("identity database pool is required")
	}
	if hasher == nil {
		return nil, errors.New("password hasher is required")
	}
	if options.SessionTTL <= 0 {
		options.SessionTTL = 7 * 24 * time.Hour
	}
	if options.PasswordResetTTL <= 0 {
		options.PasswordResetTTL = time.Hour
	}
	if options.MFAChallengeTTL <= 0 {
		options.MFAChallengeTTL = 5 * time.Minute
	}
	if options.MFASetupTTL <= 0 {
		options.MFASetupTTL = 10 * time.Minute
	}
	if options.MFAIssuer == "" {
		options.MFAIssuer = "CRM"
	}
	if options.Clock == nil {
		options.Clock = func() time.Time { return time.Now().UTC() }
	}
	if len(options.SupportedLocales) == 0 {
		options.SupportedLocales = []string{"en", "ru"}
	}
	supportedLocales := make(map[string]struct{}, len(options.SupportedLocales))
	for _, locale := range options.SupportedLocales {
		normalized := strings.ToLower(strings.TrimSpace(locale))
		if localeTagPattern.MatchString(normalized) {
			supportedLocales[normalized] = struct{}{}
		}
	}
	if len(supportedLocales) == 0 {
		supportedLocales = map[string]struct{}{"en": {}, "ru": {}}
	}
	dummyHash, err := hasher.Hash("not-a-real-user-password")
	if err != nil {
		return nil, err
	}
	return &Service{
		pool: pool, queries: dbgen.New(pool), hasher: hasher, dummyHash: dummyHash,
		sessionTTL: options.SessionTTL, passwordResetTTL: options.PasswordResetTTL,
		mfaChallengeTTL: options.MFAChallengeTTL, registrationEnabled: options.RegistrationEnabled,
		mfaSetupTTL:  options.MFASetupTTL,
		secretCipher: options.SecretCipher, resetSender: options.ResetSender,
		publicURL: strings.TrimRight(options.PublicURL, "/"), mfaIssuer: options.MFAIssuer,
		supportedLocales: supportedLocales, now: options.Clock,
	}, nil
}

func (service *Service) Login(ctx context.Context, email, password, userAgent string, address *netip.Addr) (Session, error) {
	result, err := service.BeginLogin(ctx, email, password, userAgent, address)
	if err != nil {
		return Session{}, err
	}
	if result.Session != nil {
		return *result.Session, nil
	}
	return Session{}, ErrMFARequired
}

func (service *Service) BeginLogin(
	ctx context.Context,
	email,
	password,
	userAgent string,
	address *netip.Addr,
) (LoginResult, error) {
	normalized, err := normalizeEmail(email)
	if err != nil || len(password) < 8 || len(password) > 1024 {
		service.performDummyPasswordCheck(password)
		return LoginResult{}, errx.ErrInvalidCredentials
	}
	user, err := service.queries.GetUserByNormalizedEmail(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		service.performDummyPasswordCheck(password)
		return LoginResult{}, errx.ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("lookup user: %w", err)
	}
	valid, verifyErr := service.hasher.Verify(password, user.PasswordHash)
	if verifyErr != nil {
		return LoginResult{}, fmt.Errorf("verify password: %w", verifyErr)
	}
	locked := user.LockedUntil.Valid && user.LockedUntil.Time.After(service.now())
	if !valid || locked || user.Status != "active" {
		if !valid && !locked {
			_ = service.queries.RecordLoginFailure(ctx, user.ID)
		}
		return LoginResult{}, errx.ErrInvalidCredentials
	}
	_ = service.queries.ClearLoginFailures(ctx, user.ID)

	mfaEnabled, err := service.queries.IsMFAEnabled(ctx, user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("check MFA configuration: %w", err)
	}
	if mfaEnabled {
		challenge, challengeErr := service.createMFAChallenge(ctx, service.queries, user, userAgent, address)
		if challengeErr != nil {
			return LoginResult{}, challengeErr
		}
		return LoginResult{MFAChallenge: &challenge}, nil
	}

	session, err := service.createSession(ctx, service.queries, user, userAgent, address)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Session: &session}, nil
}

func (service *Service) performDummyPasswordCheck(candidate string) {
	if !passwordInputWithinBounds(candidate) {
		candidate = "bounded-invalid-password"
	}
	_, _ = service.hasher.Verify(candidate, service.dummyHash)
}

func (service *Service) createSession(
	ctx context.Context,
	queries *dbgen.Queries,
	user dbgen.IdentityUser,
	userAgent string,
	address *netip.Addr,
) (Session, error) {

	token, tokenHash, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, csrfHash, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	sessionID, err := ids.NewV7()
	if err != nil {
		return Session{}, err
	}
	expiresAt := service.now().Add(service.sessionTTL)
	userAgent = boundedUserAgent(userAgent)
	if err := queries.CreateSession(ctx, dbgen.CreateSessionParams{
		ID:             sessionID.PG(),
		UserID:         user.ID,
		TokenHash:      tokenHash[:],
		CsrfHash:       csrfHash[:],
		SessionVersion: user.SessionVersion,
		UserAgent:      userAgent,
		IpAddress:      address,
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	userID, _ := ids.FromPG(user.ID)
	return Session{
		Principal: Principal{
			SessionID:       sessionID,
			UserID:          userID,
			Email:           user.Email,
			DisplayName:     user.DisplayName,
			PreferredLocale: user.PreferredLocale,
			CSRFHash:        csrfHash,
			ExpiresAt:       expiresAt,
		},
		Token:     token,
		CSRFToken: csrf,
	}, nil
}

func (service *Service) Authenticate(ctx context.Context, rawToken string) (Principal, error) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return Principal{}, errx.ErrUnauthenticated
	}
	digest := sha256.Sum256(decoded)
	row, err := service.queries.GetSessionPrincipal(ctx, digest[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, errx.ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("load session: %w", err)
	}
	sessionID, sessionOK := ids.FromPG(row.SessionID)
	userID, userOK := ids.FromPG(row.UserID)
	if !sessionOK || !userOK || len(row.CsrfHash) != 32 || row.Status != "active" || !row.ExpiresAt.Valid {
		return Principal{}, errx.ErrUnauthenticated
	}
	var csrfHash [32]byte
	copy(csrfHash[:], row.CsrfHash)
	// Avoid a write-query roundtrip on every authenticated request. The SQL
	// predicate remains defense in depth for concurrent stale readers.
	if !row.LastSeenAt.Valid || row.LastSeenAt.Time.Before(time.Now().Add(-5*time.Minute)) {
		_ = service.queries.TouchSession(ctx, row.SessionID)
	}
	return Principal{
		SessionID:       sessionID,
		UserID:          userID,
		Email:           row.Email,
		DisplayName:     row.DisplayName,
		PreferredLocale: row.PreferredLocale,
		CSRFHash:        csrfHash,
		ExpiresAt:       row.ExpiresAt.Time,
	}, nil
}

func (service *Service) VerifyCSRF(principal Principal, rawToken string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return false
	}
	digest := sha256.Sum256(decoded)
	return subtle.ConstantTimeCompare(digest[:], principal.CSRFHash[:]) == 1
}

func (service *Service) Logout(ctx context.Context, rawToken string) error {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return nil
	}
	digest := sha256.Sum256(decoded)
	if err := service.queries.RevokeSessionByToken(ctx, digest[:]); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (service *Service) UpdateLocale(ctx context.Context, userID ids.UUID, locale string) (dbgen.UpdateUserLocaleRow, error) {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if _, supported := service.supportedLocales[locale]; !supported {
		return dbgen.UpdateUserLocaleRow{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/preferredLocale",
			Code:    "validation.locale.unsupported",
		}}}
	}
	row, err := service.queries.UpdateUserLocale(ctx, dbgen.UpdateUserLocaleParams{
		ID:              userID.PG(),
		PreferredLocale: locale,
	})
	if err != nil {
		return dbgen.UpdateUserLocaleRow{}, fmt.Errorf("update locale: %w", err)
	}
	return row, nil
}

func normalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 || len(trimmed) > 254 {
		return "", errors.New("invalid email")
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed {
		return "", errors.New("invalid email")
	}
	return strings.ToLower(trimmed), nil
}

func randomToken() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate secure token: %w", err)
	}
	digest := sha256.Sum256(raw[:])
	return base64.RawURLEncoding.EncodeToString(raw[:]), digest, nil
}

func boundedUserAgent(value string) string {
	value = strings.ToValidUTF8(value, "")
	if utf8.RuneCountInString(value) <= 512 {
		return value
	}
	runes := 0
	for index := range value {
		if runes == 512 {
			return value[:index]
		}
		runes++
	}
	return value
}
