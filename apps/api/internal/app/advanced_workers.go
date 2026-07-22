package app

import (
	"fmt"
	"log/slog"
	"net"
	"net/smtp"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/automation"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/integrations"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/notifications"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/brand"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/search"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

// BuildAdvancedComponents centralizes optional-secret wiring without placing
// secret material in the managers or worker configuration. Callers mount the
// returned HTTP handlers under the already authenticated workspace router and
// merge the worker handlers into worker.Config.Handlers.
func BuildAdvancedComponents(
	cfg config.Config,
	logger *slog.Logger,
	pool *pgxpool.Pool,
	tenancyService *tenancy.Service,
	actionExecutor automation.ActionExecutor,
) (*AdvancedHandlers, map[string]worker.Handler, error) {
	cipher, err := integrationCipher(cfg)
	if err != nil {
		return nil, nil, err
	}
	urlValidator := integrations.URLValidator{AllowHTTP: cfg.Environment != "production"}
	integrationManager := integrations.NewManager(cipher, urlValidator)
	httpHandlers := NewAdvancedHandlers(logger, tenancyService, automation.NewManager(), integrationManager)
	workerHandlers, err := BuildWorkerHandlers(cfg, logger, pool, actionExecutor)
	if err != nil {
		return nil, nil, err
	}
	return httpHandlers, workerHandlers, nil
}

// BuildWorkerHandlers is shared by the integrated and standalone worker
// commands. Registries are merged by job kind; duplicate kinds fail closed so
// adding a second implementation cannot silently replace the active handler.
func BuildWorkerHandlers(
	cfg config.Config,
	logger *slog.Logger,
	pool *pgxpool.Pool,
	actionExecutor automation.ActionExecutor,
) (map[string]worker.Handler, error) {
	cipher, err := integrationCipher(cfg)
	if err != nil {
		return nil, err
	}
	urlValidator := integrations.URLValidator{AllowHTTP: cfg.Environment != "production"}
	repository := integrations.NewPostgresRepository(pool)
	sender, err := notificationSender(cfg)
	if err != nil {
		return nil, err
	}
	workerHandlers := map[string]worker.Handler{
		"automation.dispatch":   automation.NewDispatchWorkerHandler(pool),
		"automation.email.send": automation.NewEmailWorkerHandler(automation.NewPostgresAutomationEmailResolver(pool), sender),
		"notification.dispatch": notifications.NewDispatchWorkerHandler(notifications.NewPostgresDispatchRepository(pool)),
		"search.sync":           search.NewWorkerHandler(search.NewPostgresReconciler(pool)),
		"webhook.dispatch":      integrations.NewWebhookDispatchHandler(repository),
		"webhook.deliver": integrations.NewWebhookDeliveryHandler(
			repository,
			cipher,
			integrations.SafeWebhookClient{
				Validator: urlValidator, MaxResponseBody: 4096,
				UserAgent: brand.Config.ShortName + "/webhook-v1",
			},
			nil,
		),
	}
	if actionExecutor == nil {
		actionExecutor = automation.NewTypedActionExecutor(automation.NewPostgresActionPorts(pool))
	}
	workerHandlers["automation.execute"] = automation.NewWorkerHandler(
		automation.NewExecutionPostgres(pool), actionExecutor,
	)
	if err := mergeWorkerHandlers(workerHandlers, map[string]worker.Handler{
		"customers.import.contacts": customers.NewService().ContactImportJobHandler,
	}); err != nil {
		return nil, err
	}
	if err := mergeWorkerHandlers(workerHandlers, notifications.WorkerHandlers(
		sender, notifications.CatalogRenderer{ProductName: brand.Config.ProductName},
	)); err != nil {
		return nil, err
	}
	return workerHandlers, nil
}

func integrationCipher(cfg config.Config) (identity.SecretCipher, error) {
	if cfg.IdentityKeyBase64 == "" {
		return nil, nil
	}
	keyring, err := identity.NewAESGCMKeyringFromBase64(cfg.IdentityKeyID, cfg.IdentityKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("configure integrations encryption: %w", err)
	}
	return keyring, nil
}

func notificationSender(cfg config.Config) (notifications.EmailSender, error) {
	if cfg.SMTPAddress == "" {
		return nil, nil
	}
	host, _, err := net.SplitHostPort(cfg.SMTPAddress)
	if err != nil {
		return nil, fmt.Errorf("SMTP_ADDR must include host and port: %w", err)
	}
	var auth smtp.Auth
	if cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)
	}
	return &notifications.SMTPSender{
		Address: cfg.SMTPAddress, From: cfg.SMTPFrom, Auth: auth,
		RequireTLS: cfg.SMTPRequireTLS, ImplicitTLS: cfg.SMTPImplicitTLS,
		WriteTimeout: cfg.SMTPWriteTimeout,
	}, nil
}

func mergeWorkerHandlers(target, source map[string]worker.Handler) error {
	for kind, handler := range source {
		if _, exists := target[kind]; exists {
			return fmt.Errorf("worker handler %q is registered more than once", kind)
		}
		target[kind] = handler
	}
	return nil
}
