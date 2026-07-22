package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

type stubProvider struct {
	info     ProviderInfo
	output   string
	err      error
	calls    atomic.Int32
	started  chan struct{}
	release  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func (provider *stubProvider) Info() ProviderInfo { return provider.info }

func (provider *stubProvider) generate(ctx context.Context) (string, error) {
	provider.calls.Add(1)
	if provider.started != nil {
		provider.once.Do(func() { close(provider.started) })
	}
	if provider.release != nil {
		select {
		case <-provider.release:
		case <-ctx.Done():
			if provider.canceled != nil {
				close(provider.canceled)
			}
			return "", ctx.Err()
		}
	}
	return provider.output, provider.err
}

func (provider *stubProvider) TimelineSummary(ctx context.Context, _ TimelineSummaryRequest) (string, error) {
	return provider.generate(ctx)
}

func (provider *stubProvider) FollowUpDraft(ctx context.Context, _ FollowUpDraftRequest) (string, error) {
	return provider.generate(ctx)
}

func (provider *stubProvider) NextAction(ctx context.Context, _ NextActionRequest) (string, error) {
	return provider.generate(ctx)
}

func (provider *stubProvider) DuplicateCandidates(ctx context.Context, _ DuplicateCandidatesRequest) (string, error) {
	return provider.generate(ctx)
}

func TestExternalProviderRequiresPerRequestConsent(t *testing.T) {
	provider := &stubProvider{
		info:   ProviderInfo{Name: "openai", Class: ProviderClassExternal, Model: "private-model"},
		output: "summary",
	}
	service := newTestService(t, provider, testOptions{})
	request := validTimelineRequest()
	if _, err := service.TimelineSummary(context.Background(), request); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("error = %v, want consent required", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatal("provider was called before external transfer consent")
	}
	request.Consent = &Consent{ExternalPIITransfer: true}
	result, err := service.TimelineSummary(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "summary" || !result.Advisory || result.Capability != CapabilityTimelineSummary {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLocalProviderDoesNotRequireExternalConsent(t *testing.T) {
	provider := &stubProvider{
		info:   ProviderInfo{Name: "ollama", Class: ProviderClassLocal, Model: "local-model"},
		output: "  local summary  ",
	}
	service := newTestService(t, provider, testOptions{})
	result, err := service.TimelineSummary(context.Background(), validTimelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "local summary" {
		t.Fatalf("content = %q", result.Content)
	}
}

func TestServiceCancelsProviderAtTimeout(t *testing.T) {
	provider := &stubProvider{
		info:    ProviderInfo{Name: "ollama", Class: ProviderClassLocal},
		started: make(chan struct{}), release: make(chan struct{}), canceled: make(chan struct{}),
	}
	service := newTestService(t, provider, testOptions{timeout: 20 * time.Millisecond})
	startedAt := time.Now()
	_, err := service.TimelineSummary(context.Background(), validTimelineRequest())
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("error = %v, want provider unavailable", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("timeout was not bounded: %s", elapsed)
	}
	select {
	case <-provider.canceled:
	case <-time.After(time.Second):
		t.Fatal("provider context was not canceled")
	}
}

func TestServiceRejectsConcurrentRequestWithoutQueueing(t *testing.T) {
	provider := &stubProvider{
		info: ProviderInfo{Name: "ollama", Class: ProviderClassLocal}, output: "done",
		started: make(chan struct{}), release: make(chan struct{}),
	}
	service := newTestService(t, provider, testOptions{timeout: time.Second, maxConcurrency: 1})
	firstDone := make(chan error, 1)
	go func() {
		_, err := service.TimelineSummary(context.Background(), validTimelineRequest())
		firstDone <- err
	}()
	<-provider.started
	if _, err := service.TimelineSummary(context.Background(), validTimelineRequest()); !errors.Is(err, ErrConcurrencyLimited) {
		t.Fatalf("error = %v, want concurrency limit", err)
	}
	close(provider.release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestServiceEnforcesInputAndOutputBounds(t *testing.T) {
	provider := &stubProvider{
		info: ProviderInfo{Name: "ollama", Class: ProviderClassLocal}, output: strings.Repeat("x", 17),
	}
	service := newTestService(t, provider, testOptions{maxInputBytes: 180, maxOutputBytes: 16})
	request := validTimelineRequest()
	detail := strings.Repeat("context", 40)
	request.Items[0].Detail = &detail
	var validation *errx.ValidationError
	if _, err := service.TimelineSummary(context.Background(), request); !errors.As(err, &validation) {
		t.Fatalf("error = %v, want validation error", err)
	}
	request = validTimelineRequest()
	if _, err := service.TimelineSummary(context.Background(), request); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("error = %v, want output too large", err)
	}
}

func TestDuplicateCandidatesAreBoundedAndAdvisory(t *testing.T) {
	provider := &stubProvider{
		info: ProviderInfo{Name: "ollama", Class: ProviderClassLocal}, output: "candidate-id: likely match",
	}
	service := newTestService(t, provider, testOptions{maxDuplicateCandidates: 1})
	request := DuplicateCandidatesRequest{
		EntityType: "contact",
		Subject:    DuplicateRecord{Fields: map[string]string{"email": "person@example.test"}},
		Candidates: []DuplicateCandidate{{ID: uuid.New(), Fields: map[string]string{"email": "person@example.test"}}},
	}
	result, err := service.DuplicateCandidates(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Capability != CapabilityDuplicateCandidates || !result.Advisory {
		t.Fatalf("unexpected result: %+v", result)
	}
	request.Candidates = append(request.Candidates, DuplicateCandidate{ID: uuid.New(), Fields: map[string]string{"email": "other@example.test"}})
	if _, err := service.DuplicateCandidates(context.Background(), request); err == nil {
		t.Fatal("expected duplicate candidate bound to be enforced")
	}
}

func TestStatusAndAuditSummaryExcludeConfiguredModel(t *testing.T) {
	provider := &stubProvider{
		info: ProviderInfo{Name: "openai", Class: ProviderClassExternal, Model: "model-name-must-not-leak"},
	}
	service := newTestService(t, provider, testOptions{})
	statusJSON, err := json.Marshal(service.Status())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(statusJSON), provider.info.Model) {
		t.Fatalf("status exposed configured model: %s", statusJSON)
	}
	auditJSON, err := auditSummary(CapabilityFollowUpDraft, provider.info, true, 123)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(auditJSON), provider.info.Model) {
		t.Fatalf("audit summary exposed configured model: %s", auditJSON)
	}
	if strings.Contains(string(auditJSON), "prompt") || strings.Contains(string(auditJSON), "response") {
		t.Fatalf("audit summary contains prompt/response fields: %s", auditJSON)
	}
}

type testOptions struct {
	timeout                time.Duration
	maxInputBytes          int64
	maxOutputBytes         int64
	maxConcurrency         int
	maxDuplicateCandidates int
}

func newTestService(t *testing.T, provider Provider, override testOptions) *Service {
	t.Helper()
	if override.timeout == 0 {
		override.timeout = time.Second
	}
	if override.maxInputBytes == 0 {
		override.maxInputBytes = 32 << 10
	}
	if override.maxOutputBytes == 0 {
		override.maxOutputBytes = 8 << 10
	}
	if override.maxConcurrency == 0 {
		override.maxConcurrency = 2
	}
	if override.maxDuplicateCandidates == 0 {
		override.maxDuplicateCandidates = 25
	}
	service, err := NewService(Options{
		Provider: provider, Timeout: override.timeout,
		MaxInputBytes: override.maxInputBytes, MaxOutputBytes: override.maxOutputBytes,
		MaxContextItems: 10, MaxDuplicateCandidates: override.maxDuplicateCandidates,
		MaxConcurrency: override.maxConcurrency, SupportedLocales: []string{"en", "ru"}, DefaultLocale: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func validTimelineRequest() TimelineSummaryRequest {
	return TimelineSummaryRequest{
		Items: []ContextItem{{Kind: "note", Subject: "Customer asked for a proposal"}},
	}
}
