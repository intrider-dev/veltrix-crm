package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

func WriteProblem(writer http.ResponseWriter, request *http.Request, logger *slog.Logger, err error) {
	requestID := RequestID(request.Context())
	locale := localization.Resolve("", request.Header.Get("Accept-Language"))
	if principal, ok := Principal(request.Context()); ok {
		locale = localization.Resolve(principal.PreferredLocale, request.Header.Get("Accept-Language"))
	}

	problem := apigen.Problem{
		Type:      "/api/v1/problems/internal",
		Title:     localization.Translate(locale, "problems.problem.generic", map[string]any{"requestId": requestID}),
		Status:    http.StatusInternalServerError,
		Code:      "internal.error",
		RequestId: requestID,
	}
	var validation *errx.ValidationError
	switch {
	case errors.Is(err, errx.ErrInvalidCredentials):
		problem.Type = "/api/v1/problems/authentication"
		problem.Title = localization.Translate(locale, "auth.problem.auth.invalidCredentials", nil)
		problem.Status = http.StatusUnauthorized
		problem.Code = "auth.invalid_credentials"
	case errors.Is(err, errx.ErrUnauthenticated):
		problem.Type = "/api/v1/problems/authentication"
		problem.Title = localization.Translate(locale, "problems.problem.auth.required", nil)
		problem.Status = http.StatusUnauthorized
		problem.Code = "auth.required"
	case errors.Is(err, errx.ErrForbidden):
		problem.Type = "/api/v1/problems/forbidden"
		problem.Title = localization.Translate(locale, "problems.problem.auth.permissionDenied", nil)
		problem.Status = http.StatusForbidden
		problem.Code = "auth.permission_denied"
	case errors.Is(err, errx.ErrNotFound):
		problem.Type = "/api/v1/problems/not-found"
		problem.Title = localization.Translate(locale, "problems.problem.notFound", nil)
		problem.Status = http.StatusNotFound
		problem.Code = "resource.not_found"
	case errors.Is(err, errx.ErrVersionConflict):
		problem.Type = "/api/v1/problems/version-conflict"
		problem.Title = localization.Translate(locale, "problems.problem.versionConflict", nil)
		problem.Status = http.StatusPreconditionFailed
		problem.Code = "record.version_conflict"
	case errors.Is(err, errx.ErrIdempotencyConflict):
		problem.Type = "/api/v1/problems/idempotency-conflict"
		problem.Title = localization.Translate(locale, "problems.problem.idempotencyConflict", nil)
		problem.Status = http.StatusConflict
		problem.Code = "request.idempotency_conflict"
	case errors.Is(err, errx.ErrRateLimited):
		problem.Type = "/api/v1/problems/rate-limited"
		problem.Title = localization.Translate(locale, "problems.problem.rateLimited", nil)
		problem.Status = http.StatusTooManyRequests
		problem.Code = "request.rate_limited"
	case errors.Is(err, errx.ErrSecurityRejected):
		problem.Type = "/api/v1/problems/security-policy"
		problem.Title = localization.Translate(locale, "problems.problem.securityRejected", nil)
		problem.Status = http.StatusForbidden
		problem.Code = "request.security_rejected"
	case errors.Is(err, errx.ErrConflict):
		problem.Type = "/api/v1/problems/conflict"
		problem.Title = localization.Translate(locale, "problems.problem.conflict", nil)
		problem.Status = http.StatusConflict
		problem.Code = "resource.conflict"
	case errors.Is(err, errx.ErrUnavailable):
		problem.Type = "/api/v1/problems/unavailable"
		problem.Title = localization.Translate(locale, "problems.problem.unavailable", nil)
		problem.Status = http.StatusServiceUnavailable
		problem.Code = "service.unavailable"
	case errors.As(err, &validation):
		problem.Type = "/api/v1/problems/validation"
		problem.Title = localization.Translate(locale, "problems.problem.validation", nil)
		problem.Status = http.StatusBadRequest
		problem.Code = "validation.failed"
		fieldErrors := make([]struct {
			Code    string                  `json:"code"`
			Params  *map[string]interface{} `json:"params,omitempty"`
			Pointer string                  `json:"pointer"`
		}, 0, len(validation.Fields))
		for _, field := range validation.Fields {
			var params *map[string]interface{}
			if field.Params != nil {
				converted := make(map[string]interface{}, len(field.Params))
				for key, value := range field.Params {
					converted[key] = value
				}
				params = &converted
			}
			fieldErrors = append(fieldErrors, struct {
				Code    string                  `json:"code"`
				Params  *map[string]interface{} `json:"params,omitempty"`
				Pointer string                  `json:"pointer"`
			}{Code: field.Code, Params: params, Pointer: field.Pointer})
		}
		problem.FieldErrors = &fieldErrors
	default:
		logger.Error("request failed", "request_id", requestID, "method", request.Method, "path", request.URL.Path, "error", err)
	}
	writer.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(problem.Status)
	_ = json.NewEncoder(writer).Encode(problem)
}
