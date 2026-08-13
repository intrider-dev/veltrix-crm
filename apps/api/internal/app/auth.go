package app

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/integrations"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/brand"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
)

const csrfCookieName = "XSRF-TOKEN"

func (application *Application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie(application.sessionCookieName())
		if err != nil || cookie.Value == "" {
			httpx.WriteProblem(writer, request, application.logger, errx.ErrUnauthenticated)
			return
		}
		principal, err := application.identity.Authenticate(request.Context(), cookie.Value)
		if err != nil {
			application.clearAuthCookies(writer)
			httpx.WriteProblem(writer, request, application.logger, err)
			return
		}
		next.ServeHTTP(writer, request.WithContext(httpx.WithPrincipal(request.Context(), principal)))
	})
}

func (application *Application) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// The API-key marker is added only after token hash, workspace, scope,
		// expiry, revocation and rate-limit checks have succeeded.
		if _, ok := integrations.APIKeyFromContext(request.Context()); ok {
			next.ServeHTTP(writer, request)
			return
		}
		switch request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(writer, request)
			return
		}
		principal, ok := httpx.Principal(request.Context())
		if !ok {
			httpx.WriteProblem(writer, request, application.logger, errx.ErrUnauthenticated)
			return
		}
		headerToken := request.Header.Get("X-CSRF-Token")
		if headerToken == "" {
			headerToken = request.Header.Get("X-XSRF-TOKEN")
		}
		cookie, err := request.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" || len(cookie.Value) != len(headerToken) ||
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 ||
			!application.identity.VerifyCSRF(principal, headerToken) {
			httpx.WriteProblem(writer, request, application.logger, errx.ErrForbidden)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (application *Application) login(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[apigen.LoginRequest](writer, request, 16<<10)
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	remoteAddress := remoteIP(request)
	limitKey := "unknown"
	if remoteAddress != nil {
		limitKey = remoteAddress.String()
	}
	if !application.loginLimits.Allow(limitKey, time.Now()) {
		writer.Header().Set("Retry-After", "30")
		httpx.WriteProblem(writer, request, application.logger, errx.ErrRateLimited)
		return
	}
	result, err := application.identity.BeginLogin(
		request.Context(),
		string(body.Email),
		body.Password,
		request.UserAgent(),
		remoteAddress,
	)
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	if result.MFAChallenge != nil {
		writer.Header().Set("Cache-Control", "no-store")
		httpx.WriteJSON(writer, http.StatusAccepted, map[string]any{
			"mfaRequired": true, "challengeToken": result.MFAChallenge.Token,
			"expiresAt": result.MFAChallenge.ExpiresAt,
		})
		return
	}
	if result.Session == nil {
		httpx.WriteProblem(writer, request, application.logger, errx.ErrInvalidCredentials)
		return
	}
	session := *result.Session
	view, err := application.sessionView(request, session.Principal)
	if err != nil {
		_ = application.identity.Logout(request.Context(), session.Token)
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	application.setAuthCookies(writer, session)
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, view)
}

func (application *Application) logout(writer http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie(application.sessionCookieName()); err == nil {
		if err := application.identity.Logout(request.Context(), cookie.Value); err != nil {
			httpx.WriteProblem(writer, request, application.logger, err)
			return
		}
	}
	application.clearAuthCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

// sessionProbe treats the absence of a browser session as normal state. It is
// intentionally distinct from the protected /me resource so anonymous routes
// can preserve authenticated-user redirects without producing a failed network
// request in the browser console.
func (application *Application) sessionProbe(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	response := apigen.SessionProbe{Authenticated: false}
	cookie, err := request.Cookie(application.sessionCookieName())
	if err != nil || cookie.Value == "" {
		httpx.WriteJSON(writer, http.StatusOK, response)
		return
	}
	principal, err := application.identity.Authenticate(request.Context(), cookie.Value)
	if errors.Is(err, errx.ErrUnauthenticated) {
		application.clearAuthCookies(writer)
		httpx.WriteJSON(writer, http.StatusOK, response)
		return
	}
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	view, err := application.sessionView(request, principal)
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	response.Authenticated = true
	response.Session = &view
	httpx.WriteJSON(writer, http.StatusOK, response)
}

func (application *Application) me(writer http.ResponseWriter, request *http.Request) {
	principal, _ := httpx.Principal(request.Context())
	view, err := application.sessionView(request, principal)
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, view)
}

func (application *Application) updatePreferences(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[apigen.UpdateUserPreferences](writer, request, 16<<10)
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	if body.PreferredLocale == nil {
		httpx.WriteProblem(writer, request, application.logger, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/preferredLocale", Code: "validation.required",
		}}})
		return
	}
	principal, _ := httpx.Principal(request.Context())
	row, err := application.identity.UpdateLocale(request.Context(), principal.UserID, string(*body.PreferredLocale))
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, apigen.User{
		Id: uuid.UUID(row.ID.Bytes), Email: openapi_types.Email(row.Email),
		DisplayName: row.DisplayName, PreferredLocale: row.PreferredLocale,
	})
}

func (application *Application) sessionView(request *http.Request, principal identity.Principal) (apigen.SessionView, error) {
	workspaces, err := application.tenancy.ListWorkspaces(request.Context(), principal)
	if err != nil {
		return apigen.SessionView{}, err
	}
	items := make([]apigen.Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		permissions := make([]apigen.Permission, len(workspace.Permissions))
		for index, permission := range workspace.Permissions {
			permissions[index] = apigen.Permission(permission)
		}
		items = append(items, apigen.Workspace{
			Id: uuid.UUID(workspace.ID.Bytes), Name: workspace.Name,
			DefaultLocale: workspace.DefaultLocale, Timezone: workspace.Timezone,
			Role: apigen.WorkspaceRole(workspace.Role), RoleId: uuid.UUID(workspace.RoleID.Bytes),
			RoleName: workspace.RoleName, Permissions: permissions,
		})
	}
	return apigen.SessionView{
		User: apigen.User{
			Id: uuid.UUID(principal.UserID), Email: openapi_types.Email(principal.Email),
			DisplayName: principal.DisplayName, PreferredLocale: principal.PreferredLocale,
		},
		Workspaces: items,
	}, nil
}

func (application *Application) sessionCookieName() string {
	if application.cfg.CookieSecure {
		return "__Host-" + brand.Config.CookiePrefix + "_session"
	}
	return brand.Config.CookiePrefix + "_session"
}

func (application *Application) setAuthCookies(writer http.ResponseWriter, session identity.Session) {
	maxAge := int(time.Until(session.Principal.ExpiresAt).Seconds())
	http.SetCookie(writer, &http.Cookie{
		Name: application.sessionCookieName(), Value: session.Token, Path: "/",
		HttpOnly: true, Secure: application.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
		MaxAge: maxAge, Expires: session.Principal.ExpiresAt,
	})
	http.SetCookie(writer, &http.Cookie{
		Name: csrfCookieName, Value: session.CSRFToken, Path: "/",
		HttpOnly: false, Secure: application.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
		MaxAge: maxAge, Expires: session.Principal.ExpiresAt,
	})
}

func (application *Application) clearAuthCookies(writer http.ResponseWriter) {
	expired := time.Unix(1, 0).UTC()
	for _, cookie := range []http.Cookie{
		{Name: application.sessionCookieName(), HttpOnly: true},
		{Name: csrfCookieName, HttpOnly: false},
	} {
		cookie.Value = ""
		cookie.Path = "/"
		cookie.Secure = application.cfg.CookieSecure
		cookie.SameSite = http.SameSiteLaxMode
		cookie.MaxAge = -1
		cookie.Expires = expired
		http.SetCookie(writer, &cookie)
	}
}

func remoteIP(request *http.Request) *netip.Addr {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(request.RemoteAddr)
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return nil
	}
	address = address.Unmap()
	return &address
}
