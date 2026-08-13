package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAllowOnlySameOriginDocumentBase(t *testing.T) {
	handler := SecurityHeaders(false, "", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	policy := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "base-uri 'self'") {
		t.Fatalf("CSP does not allow the fixed same-origin <base>: %q", policy)
	}
	if strings.Contains(policy, "base-uri 'none'") {
		t.Fatalf("CSP still blocks the application's <base> element: %q", policy)
	}
	for _, directive := range strings.Split(policy, ";") {
		if strings.HasPrefix(strings.TrimSpace(directive), "script-src ") && strings.Contains(directive, "'unsafe-inline'") {
			t.Fatalf("script-src permits inline scripts or event handlers: %q", directive)
		}
	}
	if response.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS was emitted for a deployment not configured for production TLS")
	}
}

func TestSecurityHeadersEmitHSTSOnlyForProductionTLS(t *testing.T) {
	handler := SecurityHeaders(true, "", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := response.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("Strict-Transport-Security = %q", got)
	}
}

func TestSecurityHeadersAllowOnlyConfiguredCallOrigin(t *testing.T) {
	handler := SecurityHeaders(false, "wss://calls.example.test", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	policy := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "connect-src 'self' wss://calls.example.test") {
		t.Fatalf("call origin missing from CSP: %q", policy)
	}
	if got := response.Header().Get("Permissions-Policy"); got != "camera=(self), microphone=(self), geolocation=()" {
		t.Fatalf("Permissions-Policy = %q", got)
	}
}
