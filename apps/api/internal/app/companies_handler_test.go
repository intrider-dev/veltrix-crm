package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
)

func TestListCompaniesRejectsInvalidLimitBeforeDatabaseAccess(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/workspaces/{workspaceId}/companies", (&Application{}).listCompanies)
	request := httptest.NewRequest(
		http.MethodGet,
		"/workspaces/01900000-0000-7000-8000-000000000001/companies?limit=101&q=ignored",
		nil,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
		t.Fatalf("content type = %q", contentType)
	}
	var problem apigen.Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "validation.failed" || problem.FieldErrors == nil || len(*problem.FieldErrors) != 1 {
		t.Fatalf("unexpected problem: %#v", problem)
	}
	if (*problem.FieldErrors)[0].Pointer != "/query/limit" {
		t.Fatalf("field pointer = %q", (*problem.FieldErrors)[0].Pointer)
	}
}

func TestCompanyPageJSONContractIsAnObject(t *testing.T) {
	next := "opaque-cursor"
	payload, err := json.Marshal(apigen.CompanyPage{Items: []apigen.Company{}, NextCursor: &next})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded["items"]) != "[]" || string(decoded["nextCursor"]) != `"opaque-cursor"` {
		t.Fatalf("unexpected company page JSON: %s", payload)
	}
}
