package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/activities"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/ai"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/audit"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/calls"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/collaboration"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/files"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/integrations"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/mailbox"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/notifications"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/projects"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/reporting"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/search"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

// Application wires the modular-monolith services at the HTTP boundary. Domain
// services stay transport-agnostic and receive only an already-scoped tenant
// transaction.
type Application struct {
	cfg          config.Config
	logger       *slog.Logger
	pool         *pgxpool.Pool
	identity     *identity.Service
	tenancy      *tenancy.Service
	customers    *customers.Service
	sales        *sales.Service
	activities   *activities.Service
	projects     *projects.Service
	chat         *collaboration.Service
	calls        *calls.Service
	ai           *ai.Service
	reporting    *reporting.Service
	search       *search.Service
	audit        *audit.Service
	translations *localization.ContentService
	mailbox      *mailbox.Service
	attachments  *files.Service
	advanced     *AdvancedHandlers
	apiKeyAuth   *integrations.APIKeyHTTPAuthenticator
	workers      map[string]worker.Handler
	fileCloser   io.Closer
	hub          *notifications.Hub
	spa          http.Handler
	loginLimits  *httpx.RateLimiter
	mfaLimits    *httpx.RateLimiter
	resetLimits  *httpx.RateLimiter
	aiRateLimits *httpx.RateLimiter
	callLimits   *httpx.RateLimiter
	handler      http.Handler
}

func New(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, spa http.Handler) (*Application, error) {
	hasher := identity.NewPasswordHasher(2)
	identityService, err := newIdentityService(cfg, pool, hasher)
	if err != nil {
		return nil, err
	}
	if spa == nil {
		spa = http.NotFoundHandler()
	}
	aiService, err := buildAIService(cfg)
	if err != nil {
		return nil, err
	}
	mailboxService, err := buildMailboxService(cfg)
	if err != nil {
		return nil, err
	}
	var callProvider calls.Provider = calls.DisabledProvider{}
	if cfg.CallsProvider == "livekit" {
		callProvider, err = calls.NewLiveKitProvider(
			cfg.LiveKitPublicURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret, cfg.LiveKitTokenTTL,
		)
		if err != nil {
			return nil, err
		}
	}
	aiRequestsPerMinute := cfg.AIRequestsPerMinute
	if aiRequestsPerMinute < 1 {
		aiRequestsPerMinute = 10
	}
	application := &Application{
		cfg:      cfg,
		logger:   logger,
		pool:     pool,
		identity: identityService,
		tenancy: tenancy.NewServiceWithOptions(pool, tenancy.ServiceOptions{
			SupportedLocales: cfg.SupportedLocales, DefaultLocale: cfg.DefaultLocale,
		}),
		customers:    customers.NewService(),
		sales:        sales.NewService(),
		projects:     projects.NewService(),
		chat:         collaboration.NewService(),
		calls:        calls.NewService(callProvider),
		reporting:    reporting.NewService(),
		search:       search.NewService(),
		ai:           aiService,
		audit:        audit.NewService(),
		translations: localization.NewContentService(cfg.SupportedLocales),
		mailbox:      mailboxService,
		hub:          notifications.NewHub(logger),
		spa:          spa,
		loginLimits:  httpx.NewRateLimiter(8, 1.0/30.0, 10_000),
		mfaLimits:    httpx.NewRateLimiter(10, 1.0/30.0, 10_000),
		resetLimits:  httpx.NewRateLimiter(4, 1.0/60.0, 10_000),
		aiRateLimits: httpx.NewRateLimiter(aiRequestsPerMinute, float64(aiRequestsPerMinute)/60.0, 10_000),
		callLimits:   httpx.NewRateLimiter(10, 10.0/60.0, 10_000),
	}
	application.activities = activities.NewService(application)
	application.advanced, application.workers, err = BuildAdvancedComponents(
		cfg, logger, pool, application.tenancy, nil, mailboxService,
	)
	if err != nil {
		return nil, err
	}
	application.apiKeyAuth = integrations.NewAPIKeyHTTPAuthenticator(
		integrations.NewAPIKeyService(integrations.NewPostgresRepository(pool)), logger,
	)
	application.attachments, application.fileCloser, err = newAttachmentService(cfg, application)
	if err != nil {
		return nil, err
	}
	application.handler = application.routes()
	return application, nil
}

func (application *Application) Handler() http.Handler { return application.handler }

func (application *Application) Hub() *notifications.Hub { return application.hub }

// WorkerHandlers returns an isolated copy so callers can merge additional
// bounded handlers without mutating the application's registry.
func (application *Application) WorkerHandlers() map[string]worker.Handler {
	handlers := make(map[string]worker.Handler, len(application.workers))
	for kind, handler := range application.workers {
		handlers[kind] = handler
	}
	return handlers
}

func (application *Application) Close() error {
	if application.fileCloser == nil {
		return nil
	}
	return application.fileCloser.Close()
}

func (application *Application) Exists(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	entityID ids.UUID,
) (bool, error) {
	switch entityType {
	case "contact":
		_, err := application.customers.GetContact(ctx, workspace, workspaceID, entityID)
		return err == nil, ignoreNotFound(err)
	case "company":
		_, err := application.customers.GetCompany(ctx, workspace, workspaceID, entityID)
		return err == nil, ignoreNotFound(err)
	case "deal":
		_, err := application.sales.GetDeal(ctx, workspace, workspaceID, entityID)
		return err == nil, ignoreNotFound(err)
	case "activity":
		_, err := application.activities.Get(ctx, workspace, workspaceID, entityID)
		return err == nil, ignoreNotFound(err)
	case "project":
		return application.projects.Exists(ctx, workspace, workspaceID, entityID)
	case "chat_message":
		userID, ok := ids.FromPG(workspace.Membership.UserID)
		if !ok {
			return false, nil
		}
		return application.chat.MessageVisible(ctx, workspace, workspaceID, entityID, userID)
	default:
		return false, nil
	}
}

func (application *Application) readinessContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, time.Second)
}
