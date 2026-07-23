package httpx

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func RequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID, err := ids.NewV7()
		if err != nil {
			http.Error(writer, "request id unavailable", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("X-Request-ID", requestID.String())
		next.ServeHTTP(writer, request.WithContext(WithRequestID(request.Context(), requestID.String())))
	})
}

func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "request_id", RequestID(request.Context()), "panic", recovered, "stack", string(debug.Stack()))
				WriteProblem(writer, request, logger, errInternal)
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

var errInternal = &internalError{}

const baseContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; font-src 'self'; connect-src 'self'%s; worker-src 'self' blob:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; manifest-src 'self'"

type internalError struct{}

func (*internalError) Error() string { return "internal error" }

func SecurityHeaders(productionTLS bool, callsOrigin string, next http.Handler) http.Handler {
	connectSource := ""
	permissionsPolicy := "camera=(self), microphone=(self), geolocation=()"
	if callsOrigin != "" {
		connectSource = " " + callsOrigin
	}
	contentSecurityPolicy := fmt.Sprintf(baseContentSecurityPolicy, connectSource)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		headers := writer.Header()
		headers.Set("Content-Security-Policy", contentSecurityPolicy)
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("Permissions-Policy", permissionsPolicy)
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		if productionTLS {
			headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(writer, request)
	})
}

func SameOrigin(publicURL string, logger *slog.Logger, next http.Handler) http.Handler {
	expected, _ := url.Parse(publicURL)
	expectedOrigin := expected.Scheme + "://" + expected.Host
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.EqualFold(request.Header.Get("Sec-Fetch-Site"), "cross-site") {
			WriteProblem(writer, request, logger, errx.ErrSecurityRejected)
			return
		}
		if origin := request.Header.Get("Origin"); origin != "" && origin != expectedOrigin {
			WriteProblem(writer, request, logger, errx.ErrSecurityRejected)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Flush() {
	if flusher, ok := recorder.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func AccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		logger.Info("http request",
			"request_id", RequestID(request.Context()),
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}
