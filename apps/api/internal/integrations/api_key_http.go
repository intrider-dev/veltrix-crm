package integrations

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type apiKeyContextKey uint8

const authenticatedAPIKeyContext apiKeyContextKey = iota

func APIKeyFromContext(ctx context.Context) (AuthenticatedAPIKey, bool) {
	key, ok := ctx.Value(authenticatedAPIKeyContext).(AuthenticatedAPIKey)
	return key, ok
}

type APIKeyHTTPAuthenticator struct {
	service  *APIKeyService
	logger   *slog.Logger
	preAuth  *httpx.RateLimiter
	postAuth *httpx.RateLimiter
}

func NewAPIKeyHTTPAuthenticator(service *APIKeyService, logger *slog.Logger) *APIKeyHTTPAuthenticator {
	if logger == nil {
		logger = slog.Default()
	}
	return &APIKeyHTTPAuthenticator{
		service: service, logger: logger,
		preAuth:  httpx.NewRateLimiter(30, 1, 20000),
		postAuth: httpx.NewRateLimiter(120, 2, 20000),
	}
}

// Require authenticates only a Bearer API key, verifies that the workspace
// embedded in the key equals the route workspace, and checks the route scope.
// Mount this middleware outside cookie-session/CSRF middleware: CSRF is bypassed
// only after this function has cryptographically verified a non-cookie key.
func (authenticator *APIKeyHTTPAuthenticator) Require(scope Scope, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if authenticator == nil {
			httpx.WriteProblem(writer, request, slog.Default(), errx.ErrUnauthenticated)
			return
		}
		if authenticator.service == nil || !validScope(scope) {
			httpx.WriteProblem(writer, request, authenticator.logger, errx.ErrUnauthenticated)
			return
		}
		clientKey := request.RemoteAddr
		if host, _, err := net.SplitHostPort(clientKey); err == nil {
			clientKey = host
		}
		if !authenticator.preAuth.Allow(clientKey, time.Now()) {
			writer.Header().Set("Retry-After", "1")
			httpx.WriteProblem(writer, request, authenticator.logger, errx.ErrRateLimited)
			return
		}
		token, ok := bearerToken(request.Header.Get("Authorization"))
		if !ok {
			httpx.WriteProblem(writer, request, authenticator.logger, errx.ErrUnauthenticated)
			return
		}
		key, err := authenticator.service.Authenticate(request.Context(), token, time.Now().UTC())
		if err != nil {
			httpx.WriteProblem(writer, request, authenticator.logger, err)
			return
		}
		workspaceID, err := ids.Parse(requestWorkspaceID(request))
		if err != nil || workspaceID != key.WorkspaceID {
			httpx.WriteProblem(writer, request, authenticator.logger, errx.ErrForbidden)
			return
		}
		if !HasScope(key.Scopes, scope) {
			httpx.WriteProblem(writer, request, authenticator.logger, errx.ErrForbidden)
			return
		}
		if !authenticator.postAuth.Allow(key.KeyID.String(), time.Now()) {
			writer.Header().Set("Retry-After", "1")
			httpx.WriteProblem(writer, request, authenticator.logger, errx.ErrRateLimited)
			return
		}
		expiresAt := time.Now().UTC().Add(5 * time.Minute)
		if key.ExpiresAt != nil && key.ExpiresAt.Before(expiresAt) {
			expiresAt = key.ExpiresAt.UTC()
		}
		principal := identity.Principal{UserID: key.CreatedBy, ExpiresAt: expiresAt}
		ctx := context.WithValue(request.Context(), authenticatedAPIKeyContext, key)
		ctx = httpx.WithPrincipal(ctx, principal)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func requestWorkspaceID(request *http.Request) string {
	if value := chi.URLParam(request, "workspaceId"); value != "" {
		return value
	}
	// A parent router's middleware can run before the nested route has added
	// URL params. The public contract has one canonical workspace position, so
	// use the escaped-path-independent decoded URL path as a strict fallback.
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) >= 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "workspaces" {
		return parts[3]
	}
	return ""
}

func bearerToken(header string) (string, bool) {
	if len(header) < 8 || len(header) > 240 || strings.ContainsAny(header, "\r\n\t") {
		return "", false
	}
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
