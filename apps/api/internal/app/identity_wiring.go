package app

import (
	"fmt"
	"net"
	"net/smtp"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/brand"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
)

func newIdentityService(
	cfg config.Config,
	pool *pgxpool.Pool,
	hasher *identity.PasswordHasher,
) (*identity.Service, error) {
	var secretCipher identity.SecretCipher
	if cfg.IdentityKeyBase64 != "" {
		keyring, err := identity.NewAESGCMKeyringFromBase64(cfg.IdentityKeyID, cfg.IdentityKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("configure identity encryption: %w", err)
		}
		secretCipher = keyring
	}

	var resetSender identity.PasswordResetSender
	if cfg.SMTPAddress != "" {
		host, _, err := net.SplitHostPort(cfg.SMTPAddress)
		if err != nil {
			return nil, fmt.Errorf("SMTP_ADDR must include host and port: %w", err)
		}
		var auth smtp.Auth
		if cfg.SMTPUsername != "" {
			auth = smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)
		}
		resetSender = &identity.SMTPPasswordResetSender{
			Address: cfg.SMTPAddress, From: cfg.SMTPFrom, Auth: auth,
			Renderer:   localization.CatalogEmailRenderer{ProductName: brand.Config.ProductName},
			RequireTLS: cfg.SMTPRequireTLS, ImplicitTLS: cfg.SMTPImplicitTLS,
			WriteTimeout: cfg.SMTPWriteTimeout,
		}
	}

	return identity.NewServiceWithOptions(pool, hasher, identity.ServiceOptions{
		SessionTTL: cfg.SessionTTL, PasswordResetTTL: cfg.PasswordResetTTL,
		MFAChallengeTTL: cfg.MFAChallengeTTL, MFASetupTTL: cfg.MFASetupTTL,
		RegistrationEnabled: cfg.Environment == "development",
		SecretCipher:        secretCipher, ResetSender: resetSender,
		PublicURL: cfg.PublicURL, MFAIssuer: brand.Config.ProductName,
		SupportedLocales: cfg.SupportedLocales,
	})
}
