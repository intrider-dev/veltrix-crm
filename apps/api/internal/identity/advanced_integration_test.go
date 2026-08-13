//go:build integration

package identity

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestAdvancedIdentityMFAAndSessionRevocation(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, adminURL, appPassword); err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(ctx, adminURL, 1, "identity-advanced-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	pool, err := database.Open(ctx, appURL, 2, "identity-advanced-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	keyring, err := NewAESGCMKeyringFromBase64("integration-v1", key)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(30 * time.Second).Add(5 * time.Second)
	service, err := NewServiceWithOptions(pool, NewPasswordHasher(1), ServiceOptions{
		SessionTTL: 24 * time.Hour, RegistrationEnabled: true, SecretCipher: keyring,
		MFAIssuer: "CRM Integration", Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	uniqueID, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	email := "mfa-" + strings.ReplaceAll(uniqueID.String(), "-", "") + "@example.invalid"
	user, err := service.RegisterDevelopmentUser(ctx, DevelopmentRegistration{
		Email: email, DisplayName: "MFA Integration", Password: "Demo123!", Locale: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id=$1`, user.ID) }()
	userID, ok := ids.FromPG(user.ID)
	if !ok {
		t.Fatal("invalid generated user ID")
	}

	setup, err := service.BeginMFASetup(ctx, userID, "Demo123!", "")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	if err != nil {
		t.Fatal(err)
	}
	setupCode := hmacOneTimePassword(secret, uint64(now.Unix()/totpPeriodSeconds), totpDigits)
	recoveryCodes, err := service.ConfirmMFASetup(ctx, userID, setupCode)
	if err != nil {
		t.Fatal(err)
	}
	if len(recoveryCodes) != recoveryCodeCount {
		t.Fatalf("got %d recovery codes", len(recoveryCodes))
	}

	now = now.Add(30 * time.Second)
	login, err := service.BeginLogin(ctx, email, "Demo123!", "integration", nil)
	if err != nil {
		t.Fatal(err)
	}
	if login.Session != nil || login.MFAChallenge == nil {
		t.Fatal("MFA-enabled password login issued a session")
	}
	loginCode := hmacOneTimePassword(secret, uint64(now.Unix()/totpPeriodSeconds), totpDigits)
	session, err := service.CompleteMFALogin(ctx, login.MFAChallenge.Token, loginCode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, session.Token); err != nil {
		t.Fatalf("new MFA session did not authenticate: %v", err)
	}
	if err := service.LogoutAll(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, session.Token); !errors.Is(err, errx.ErrUnauthenticated) {
		t.Fatalf("session survived logout-all: %v", err)
	}

	recoveryLogin, err := service.BeginLogin(ctx, email, "Demo123!", "integration", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteMFALogin(ctx, recoveryLogin.MFAChallenge.Token, recoveryCodes[0]); err != nil {
		t.Fatalf("recovery code rejected: %v", err)
	}
	replayLogin, err := service.BeginLogin(ctx, email, "Demo123!", "integration", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteMFALogin(ctx, replayLogin.MFAChallenge.Token, recoveryCodes[0]); !errors.Is(err, ErrInvalidMFA) {
		t.Fatalf("replayed recovery code error = %v", err)
	}
}
