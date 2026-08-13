package app

import (
	"net/http"
	"strings"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/integrations"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
)

// authenticateCredential keeps browser sessions and API keys on the same
// resource URLs without weakening CSRF. A request that presents Authorization
// is never allowed to fall back to a cookie, and only an explicitly mapped
// route can be reached with an API key.
func (application *Application) authenticateCredential(next http.Handler) http.Handler {
	sessionHandler := application.authenticate(next)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.TrimSpace(request.Header.Get("Authorization")) == "" {
			sessionHandler.ServeHTTP(writer, request)
			return
		}
		scope, allowed := apiKeyScopeForRequest(request.Method, request.URL.Path)
		if !allowed || application.apiKeyAuth == nil {
			httpx.WriteProblem(writer, request, application.logger, errx.ErrForbidden)
			return
		}
		application.apiKeyAuth.Require(scope, next).ServeHTTP(writer, request)
	})
}

func apiKeyScopeForRequest(method, path string) (integrations.Scope, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "workspaces" || parts[3] == "" {
		return "", false
	}
	resource := parts[4]
	read := method == http.MethodGet || method == http.MethodHead
	switch resource {
	case "contacts":
		if read {
			return integrations.ScopeContactsRead, true
		}
		return integrations.ScopeContactsWrite, isMutation(method)
	case "companies":
		if read {
			return integrations.ScopeCompaniesRead, true
		}
		return integrations.ScopeCompaniesWrite, isMutation(method)
	case "deals", "pipelines":
		if read {
			return integrations.ScopeDealsRead, true
		}
		return integrations.ScopeDealsWrite, isMutation(method)
	case "activities":
		if read {
			return integrations.ScopeActivitiesRead, true
		}
		return integrations.ScopeActivitiesWrite, isMutation(method)
	case "dashboard", "reports":
		return integrations.ScopeReportsRead, read
	case "webhooks", "webhook-deliveries":
		return integrations.ScopeWebhooksWrite, method == http.MethodGet || isMutation(method)
	default:
		return "", false
	}
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
