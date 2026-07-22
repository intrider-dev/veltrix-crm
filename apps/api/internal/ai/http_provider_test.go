package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOllamaAdapterUsesBoundedNonStreamingChatRequest(t *testing.T) {
	type capturedRequest struct {
		path          string
		authorization string
		body          string
	}
	captured := make(chan capturedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		captured <- capturedRequest{request.URL.Path, request.Header.Get("Authorization"), string(body)}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"message":{"role":"assistant","content":"Local summary"}}`))
	}))
	defer server.Close()
	provider, err := NewOllamaProvider(AdapterOptions{
		BaseURL: server.URL, Model: "local-model", APIKey: "local-secret",
		Timeout: time.Second, MaxOutputBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	detail := "person@example.test"
	content, err := provider.TimelineSummary(context.Background(), TimelineSummaryRequest{
		Locale: "en", Items: []ContextItem{{Kind: "email", Subject: "Follow up", Detail: &detail}},
		Consent: &Consent{ExternalPIITransfer: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if content != "Local summary" {
		t.Fatalf("content = %q", content)
	}
	request := <-captured
	if request.path != "/api/chat" || request.authorization != "Bearer local-secret" {
		t.Fatalf("unexpected request path/auth: %+v", request)
	}
	if !strings.Contains(request.body, `"stream":false`) || !strings.Contains(request.body, `"num_predict":256`) {
		t.Fatalf("request is not bounded/non-streaming: %s", request.body)
	}
	if !strings.Contains(request.body, detail) || !strings.Contains(request.body, "untrusted data") {
		t.Fatalf("request did not contain bounded CRM data and safety instruction: %s", request.body)
	}
	if strings.Contains(request.body, "externalPiiTransfer") {
		t.Fatalf("consent metadata was sent to provider: %s", request.body)
	}
}

func TestOpenAIAdapterUsesChatCompletionsAndBearerKey(t *testing.T) {
	type capturedRequest struct {
		path          string
		authorization string
		body          []byte
	}
	captured := make(chan capturedRequest, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		captured <- capturedRequest{request.URL.Path, request.Header.Get("Authorization"), body}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Draft response"}}]}`))
	}))
	defer server.Close()
	provider, err := NewOpenAIProvider(AdapterOptions{
		BaseURL: server.URL + "/v1", Model: "compatible-model", APIKey: "external-secret",
		Timeout: time.Second, MaxOutputBytes: 2048, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := provider.FollowUpDraft(context.Background(), FollowUpDraftRequest{
		Locale: "ru", Channel: "email", Objective: "Prepare a concise reply",
		Context: []ContextItem{{Kind: "note", Subject: "Asked for pricing"}},
		Consent: &Consent{ExternalPIITransfer: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if content != "Draft response" {
		t.Fatalf("content = %q", content)
	}
	request := <-captured
	if request.path != "/v1/chat/completions" || request.authorization != "Bearer external-secret" {
		t.Fatalf("unexpected request path/auth: path=%q authorization=%q", request.path, request.authorization)
	}
	var payload struct {
		Model     string        `json:"model"`
		Messages  []chatMessage `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
	}
	if err := json.Unmarshal(request.body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Model != "compatible-model" || payload.MaxTokens != 512 || len(payload.Messages) != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if strings.Contains(string(request.body), "externalPiiTransfer") {
		t.Fatalf("consent metadata was sent to provider: %s", request.body)
	}
}

func TestAdapterErrorsNeverIncludeProviderBodyOrAPIKey(t *testing.T) {
	const sensitive = "person@example.test external-secret"
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, sensitive, http.StatusBadRequest)
	}))
	defer server.Close()
	provider, err := NewOpenAIProvider(AdapterOptions{
		BaseURL: server.URL, Model: "compatible-model", APIKey: "external-secret",
		Timeout: time.Second, MaxOutputBytes: 1024, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.NextAction(context.Background(), NextActionRequest{Locale: "en", EntityType: "contact"})
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("error = %v, want provider unavailable", err)
	}
	if strings.Contains(err.Error(), "person@example.test") || strings.Contains(err.Error(), "external-secret") {
		t.Fatalf("provider error leaked sensitive data: %v", err)
	}
}

func TestAdapterRejectsOversizedOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"message":{"content":"123456789"}}`))
	}))
	defer server.Close()
	provider, err := NewOllamaProvider(AdapterOptions{
		BaseURL: server.URL, Model: "local-model", Timeout: time.Second, MaxOutputBytes: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.NextAction(context.Background(), NextActionRequest{Locale: "en", EntityType: "contact"})
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("error = %v, want output too large", err)
	}
}

func TestAdaptersRejectUnsafeProviderConfigurationAndRedirects(t *testing.T) {
	if _, err := NewOpenAIProvider(AdapterOptions{
		BaseURL: "http://api.example.test/v1", Model: "model", APIKey: "secret",
		Timeout: time.Second, MaxOutputBytes: 1024,
	}); err == nil {
		t.Fatal("expected external provider HTTP URL to be rejected")
	}
	if _, err := NewOllamaProvider(AdapterOptions{
		BaseURL: "https://public.example.test", Model: "model",
		Timeout: time.Second, MaxOutputBytes: 1024,
	}); err == nil {
		t.Fatal("expected non-local Ollama provider URL to be rejected")
	}
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Store(true)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	provider, err := NewOllamaProvider(AdapterOptions{
		BaseURL: source.URL, Model: "model", APIKey: "secret",
		Timeout: time.Second, MaxOutputBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.NextAction(context.Background(), NextActionRequest{Locale: "en", EntityType: "contact"})
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("error = %v, want provider unavailable", err)
	}
	if redirected.Load() {
		t.Fatal("provider request followed a redirect")
	}
}
