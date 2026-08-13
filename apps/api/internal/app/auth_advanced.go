package app

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
)

type mfaLoginRequest struct {
	ChallengeToken string `json:"challengeToken"`
	Code           string `json:"code"`
}

type developmentRegistrationRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Locale      string `json:"locale"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmation struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type mfaSetupRequest struct {
	CurrentPassword string `json:"currentPassword"`
}

type mfaCodeRequest struct {
	Code string `json:"code"`
}

type mfaProtectedRequest struct {
	CurrentPassword string `json:"currentPassword"`
	Code            string `json:"code"`
}

func (application *Application) registerDevelopmentUser(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[developmentRegistrationRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	row, err := application.identity.RegisterDevelopmentUser(request.Context(), identity.DevelopmentRegistration{
		Email: body.Email, DisplayName: body.DisplayName, Password: body.Password, Locale: body.Locale,
	})
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusCreated, map[string]any{
		"id": apiID(row.ID), "email": row.Email, "displayName": row.DisplayName,
		"preferredLocale": row.PreferredLocale,
	})
}

func (application *Application) completeMFALogin(writer http.ResponseWriter, request *http.Request) {
	if !application.allowAuthAttempt(application.mfaLimits, writer, request, 30) {
		return
	}
	body, _, err := httpx.DecodeJSON[mfaLoginRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	session, err := application.identity.CompleteMFALogin(request.Context(), body.ChallengeToken, body.Code)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
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

func (application *Application) requestPasswordReset(writer http.ResponseWriter, request *http.Request) {
	if !application.allowAuthAttempt(application.resetLimits, writer, request, 60) {
		return
	}
	body, _, err := httpx.DecodeJSON[passwordResetRequest](writer, request, 8<<10)
	if writeError(application, writer, request, err) {
		return
	}
	err = application.identity.RequestPasswordReset(request.Context(), body.Email)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusAccepted)
}

func (application *Application) confirmPasswordReset(writer http.ResponseWriter, request *http.Request) {
	if !application.allowAuthAttempt(application.resetLimits, writer, request, 60) {
		return
	}
	body, _, err := httpx.DecodeJSON[passwordResetConfirmation](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	err = application.identity.ResetPassword(request.Context(), body.Token, body.NewPassword)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	application.clearAuthCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) changePassword(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[passwordChangeRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.identity.ChangePassword(request.Context(), principal.UserID, body.CurrentPassword, body.NewPassword)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	application.clearAuthCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) logoutAllSessions(writer http.ResponseWriter, request *http.Request) {
	principal, _ := httpx.Principal(request.Context())
	if writeError(application, writer, request, application.identity.LogoutAll(request.Context(), principal.UserID)) {
		return
	}
	application.clearAuthCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) mfaStatus(writer http.ResponseWriter, request *http.Request) {
	principal, _ := httpx.Principal(request.Context())
	enabled, err := application.identity.MFAEnabled(request.Context(), principal.UserID)
	if writeError(application, writer, request, err) {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, map[string]bool{"enabled": enabled})
}

func (application *Application) beginMFASetup(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[mfaSetupRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	setup, err := application.identity.BeginMFASetup(
		request.Context(), principal.UserID, body.CurrentPassword, principal.Email,
	)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, map[string]string{
		"secret": setup.Secret, "provisioningUri": setup.ProvisioningURI,
	})
}

func (application *Application) confirmMFASetup(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[mfaCodeRequest](writer, request, 8<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	codes, err := application.identity.ConfirmMFASetup(request.Context(), principal.UserID, body.Code)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	application.clearAuthCookies(writer)
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, map[string]any{"recoveryCodes": codes, "sessionsRevoked": true})
}

func (application *Application) regenerateRecoveryCodes(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[mfaProtectedRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	codes, err := application.identity.RegenerateRecoveryCodes(
		request.Context(), principal.UserID, body.CurrentPassword, body.Code,
	)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	application.clearAuthCookies(writer)
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, map[string]any{"recoveryCodes": codes, "sessionsRevoked": true})
}

func (application *Application) disableMFA(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[mfaProtectedRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.identity.DisableMFA(request.Context(), principal.UserID, body.CurrentPassword, body.Code)
	if writeError(application, writer, request, mapIdentityError(err)) {
		return
	}
	application.clearAuthCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) allowAuthAttempt(
	limiter *httpx.RateLimiter,
	writer http.ResponseWriter,
	request *http.Request,
	retryAfter int,
) bool {
	key := "unknown"
	if address := remoteIP(request); address != nil {
		key = address.String()
	}
	if limiter.Allow(key, time.Now()) {
		return true
	}
	writer.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	httpx.WriteProblem(writer, request, application.logger, errx.ErrRateLimited)
	return false
}

func mapIdentityError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, identity.ErrInvalidMFA):
		return errx.ErrInvalidCredentials
	case errors.Is(err, identity.ErrInvalidResetToken):
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/token", Code: "auth.reset.invalid"}}}
	case errors.Is(err, identity.ErrMFAUnavailable), errors.Is(err, identity.ErrResetDeliveryMissing):
		return errx.ErrUnavailable
	case errors.Is(err, identity.ErrRegistrationDisabled):
		return errx.ErrNotFound
	default:
		return err
	}
}
