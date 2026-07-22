package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

const (
	maxContextKindBytes    = 80
	maxContextSubjectBytes = 500
	maxContextDetailBytes  = 10_000
	maxObjectiveBytes      = 2_000
	maxRecordFields        = 12
	maxFieldNameBytes      = 64
	maxFieldValueBytes     = 1 << 10
)

type Options struct {
	Provider               Provider
	Timeout                time.Duration
	MaxInputBytes          int64
	MaxOutputBytes         int64
	MaxContextItems        int
	MaxDuplicateCandidates int
	MaxConcurrency         int
	SupportedLocales       []string
	DefaultLocale          string
}

type Service struct {
	provider         Provider
	timeout          time.Duration
	limits           Limits
	defaultLocale    string
	supportedLocales map[string]struct{}
	slots            chan struct{}
}

func NewService(options Options) (*Service, error) {
	if options.Provider == nil {
		return nil, errors.New("AI provider is required")
	}
	if options.Timeout <= 0 || options.MaxInputBytes <= 0 || options.MaxOutputBytes <= 0 ||
		options.MaxContextItems <= 0 || options.MaxDuplicateCandidates <= 0 || options.MaxConcurrency <= 0 {
		return nil, errors.New("AI limits must be positive")
	}
	if options.Timeout > time.Minute || options.MaxInputBytes > 64<<10 || options.MaxOutputBytes > 32<<10 ||
		options.MaxContextItems > 200 || options.MaxDuplicateCandidates > 100 || options.MaxConcurrency > 16 {
		return nil, errors.New("AI limits exceed hard safety bounds")
	}
	locales := make(map[string]struct{}, len(options.SupportedLocales))
	for _, locale := range options.SupportedLocales {
		normalized := strings.ToLower(strings.TrimSpace(locale))
		if normalized != "" {
			locales[normalized] = struct{}{}
		}
	}
	defaultLocale := strings.ToLower(strings.TrimSpace(options.DefaultLocale))
	if _, ok := locales[defaultLocale]; !ok {
		return nil, errors.New("AI default locale must be supported")
	}
	return &Service{
		provider: options.Provider,
		timeout:  options.Timeout,
		limits: Limits{
			MaxInputBytes:          options.MaxInputBytes,
			MaxOutputBytes:         options.MaxOutputBytes,
			MaxContextItems:        options.MaxContextItems,
			MaxDuplicateCandidates: options.MaxDuplicateCandidates,
		},
		defaultLocale:    defaultLocale,
		supportedLocales: locales,
		slots:            make(chan struct{}, options.MaxConcurrency),
	}, nil
}

func DisabledStatus(limits Limits) Status {
	return Status{
		Enabled:      false,
		Capabilities: append([]Capability(nil), supportedCapabilities...),
		Limits:       limits,
	}
}

func (service *Service) Status() Status {
	info := service.provider.Info()
	provider := info.Name
	providerClass := info.Class
	return Status{
		Enabled:                    true,
		Provider:                   &provider,
		ProviderClass:              &providerClass,
		RequiresExternalPIIConsent: info.Class == ProviderClassExternal,
		Capabilities:               append([]Capability(nil), supportedCapabilities...),
		Limits:                     service.limits,
	}
}

func (service *Service) ProviderInfo() ProviderInfo { return service.provider.Info() }

func (service *Service) TimelineSummary(ctx context.Context, request TimelineSummaryRequest) (Result, error) {
	request.Locale = service.locale(request.Locale)
	if request.Locale == "" {
		return Result{}, invalid("/locale", "validation.enum")
	}
	if request.EntityType != "" && !validEntityType(request.EntityType) {
		return Result{}, invalid("/entityType", "validation.enum")
	}
	if err := service.validateContext(request.Items, "/items"); err != nil {
		return Result{}, err
	}
	if err := service.validateInputSize(request); err != nil {
		return Result{}, err
	}
	content, err := service.execute(ctx, request.Consent, func(callCtx context.Context) (string, error) {
		return service.provider.TimelineSummary(callCtx, request)
	})
	return result(CapabilityTimelineSummary, content), err
}

func (service *Service) FollowUpDraft(ctx context.Context, request FollowUpDraftRequest) (Result, error) {
	request.Locale = service.locale(request.Locale)
	if request.Locale == "" {
		return Result{}, invalid("/locale", "validation.enum")
	}
	if !oneOf(request.Channel, "email", "message", "call_script") {
		return Result{}, invalid("/channel", "validation.enum")
	}
	if !boundedText(request.Tone, 80, true) {
		return Result{}, invalid("/tone", "validation.length")
	}
	if !boundedText(request.Recipient, maxContextSubjectBytes, true) {
		return Result{}, invalid("/recipient", "validation.length")
	}
	if !boundedText(request.Objective, maxObjectiveBytes, false) {
		return Result{}, invalid("/objective", "validation.length")
	}
	if err := service.validateContext(request.Context, "/context"); err != nil {
		return Result{}, err
	}
	if err := service.validateInputSize(request); err != nil {
		return Result{}, err
	}
	content, err := service.execute(ctx, request.Consent, func(callCtx context.Context) (string, error) {
		return service.provider.FollowUpDraft(callCtx, request)
	})
	return result(CapabilityFollowUpDraft, content), err
}

func (service *Service) NextAction(ctx context.Context, request NextActionRequest) (Result, error) {
	request.Locale = service.locale(request.Locale)
	if request.Locale == "" {
		return Result{}, invalid("/locale", "validation.enum")
	}
	if !validEntityType(request.EntityType) {
		return Result{}, invalid("/entityType", "validation.enum")
	}
	if !boundedText(request.Goal, maxObjectiveBytes, true) {
		return Result{}, invalid("/goal", "validation.length")
	}
	if err := service.validateContext(request.Context, "/context"); err != nil {
		return Result{}, err
	}
	if err := service.validateInputSize(request); err != nil {
		return Result{}, err
	}
	content, err := service.execute(ctx, request.Consent, func(callCtx context.Context) (string, error) {
		return service.provider.NextAction(callCtx, request)
	})
	return result(CapabilityNextAction, content), err
}

func (service *Service) DuplicateCandidates(ctx context.Context, request DuplicateCandidatesRequest) (Result, error) {
	request.Locale = service.locale(request.Locale)
	if request.Locale == "" {
		return Result{}, invalid("/locale", "validation.enum")
	}
	if !oneOf(request.EntityType, "contact", "company") {
		return Result{}, invalid("/entityType", "validation.enum")
	}
	if err := validateFields(request.Subject.Fields, "/subject/fields"); err != nil {
		return Result{}, err
	}
	if len(request.Candidates) < 1 || len(request.Candidates) > service.limits.MaxDuplicateCandidates {
		return Result{}, invalid("/candidates", "validation.range")
	}
	for index, candidate := range request.Candidates {
		if candidate.ID == [16]byte{} {
			return Result{}, invalid(fmt.Sprintf("/candidates/%d/id", index), "validation.required")
		}
		if err := validateFields(candidate.Fields, fmt.Sprintf("/candidates/%d/fields", index)); err != nil {
			return Result{}, err
		}
	}
	if err := service.validateInputSize(request); err != nil {
		return Result{}, err
	}
	content, err := service.execute(ctx, request.Consent, func(callCtx context.Context) (string, error) {
		return service.provider.DuplicateCandidates(callCtx, request)
	})
	return result(CapabilityDuplicateCandidates, content), err
}

func (service *Service) execute(ctx context.Context, consent *Consent, call func(context.Context) (string, error)) (string, error) {
	if service.provider.Info().Class == ProviderClassExternal && (consent == nil || !consent.ExternalPIITransfer) {
		return "", ErrConsentRequired
	}
	select {
	case service.slots <- struct{}{}:
		defer func() { <-service.slots }()
	case <-ctx.Done():
		return "", fmt.Errorf("%w: request canceled", ErrProviderUnavailable)
	default:
		return "", ErrConcurrencyLimited
	}
	callCtx, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	content, err := call(callCtx)
	if err != nil {
		if errors.Is(err, ErrOutputTooLarge) {
			return "", err
		}
		return "", fmt.Errorf("%w: request failed", ErrProviderUnavailable)
	}
	content = strings.TrimSpace(content)
	if content == "" || !utf8.ValidString(content) {
		return "", fmt.Errorf("%w: invalid response", ErrProviderUnavailable)
	}
	if int64(len(content)) > service.limits.MaxOutputBytes {
		return "", ErrOutputTooLarge
	}
	return content, nil
}

func (service *Service) validateContext(items []ContextItem, pointer string) error {
	if len(items) < 1 || len(items) > service.limits.MaxContextItems {
		return invalid(pointer, "validation.range")
	}
	for index, item := range items {
		base := fmt.Sprintf("%s/%d", pointer, index)
		if !boundedText(item.Kind, maxContextKindBytes, false) {
			return invalid(base+"/kind", "validation.length")
		}
		if !boundedText(item.Subject, maxContextSubjectBytes, false) {
			return invalid(base+"/subject", "validation.length")
		}
		if item.Detail != nil && !boundedText(*item.Detail, maxContextDetailBytes, true) {
			return invalid(base+"/detail", "validation.length")
		}
	}
	return nil
}

func (service *Service) validateInputSize(input any) error {
	encoded, err := json.Marshal(input)
	if err != nil {
		return invalid("/", "validation.json.invalid")
	}
	if int64(len(encoded)) > service.limits.MaxInputBytes {
		return invalid("/", "validation.body.too_large")
	}
	return nil
}

func (service *Service) locale(candidate string) string {
	if candidate == "" {
		return service.defaultLocale
	}
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if _, ok := service.supportedLocales[candidate]; !ok {
		return ""
	}
	return candidate
}

func validateFields(fields map[string]string, pointer string) error {
	if len(fields) < 1 || len(fields) > maxRecordFields {
		return invalid(pointer, "validation.range")
	}
	for name, value := range fields {
		if !boundedText(name, maxFieldNameBytes, false) {
			return invalid(pointer, "validation.length")
		}
		if !boundedText(value, maxFieldValueBytes, true) {
			return invalid(pointer+"/"+name, "validation.length")
		}
	}
	return nil
}

func validEntityType(value string) bool {
	return oneOf(value, "contact", "company", "lead", "deal", "activity")
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func boundedText(value string, maximum int, allowEmpty bool) bool {
	if !utf8.ValidString(value) || len(value) > maximum {
		return false
	}
	if !allowEmpty && strings.TrimSpace(value) == "" {
		return false
	}
	return !strings.ContainsRune(value, '\x00')
}

func invalid(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}

func result(capability Capability, content string) Result {
	if content == "" {
		return Result{}
	}
	return Result{Capability: capability, Content: content, Advisory: true}
}
