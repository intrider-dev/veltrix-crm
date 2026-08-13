package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
)

func TestSessionProbeTreatsMissingCookieAsNormalAnonymousState(t *testing.T) {
	application := &Application{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	response := httptest.NewRecorder()

	application.sessionProbe(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var probe apigen.SessionProbe
	if err := json.NewDecoder(response.Body).Decode(&probe); err != nil {
		t.Fatal(err)
	}
	if probe.Authenticated || probe.Session != nil {
		t.Fatalf("probe = %+v, want anonymous state", probe)
	}
}
