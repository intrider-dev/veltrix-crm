package ai

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Capability string

const (
	CapabilityTimelineSummary     Capability = "timeline_summary"
	CapabilityFollowUpDraft       Capability = "follow_up_draft"
	CapabilityNextAction          Capability = "next_action"
	CapabilityDuplicateCandidates Capability = "duplicate_candidates"
)

var supportedCapabilities = []Capability{
	CapabilityTimelineSummary,
	CapabilityFollowUpDraft,
	CapabilityNextAction,
	CapabilityDuplicateCandidates,
}

type ProviderClass string

const (
	ProviderClassLocal    ProviderClass = "local"
	ProviderClassExternal ProviderClass = "external"
)

var (
	ErrConsentRequired     = errors.New("explicit external PII transfer consent is required")
	ErrConcurrencyLimited  = errors.New("AI concurrency limit reached")
	ErrProviderUnavailable = errors.New("AI provider unavailable")
	ErrOutputTooLarge      = errors.New("AI provider output exceeded the configured limit")
)

type Consent struct {
	ExternalPIITransfer bool `json:"externalPiiTransfer"`
}

type ContextItem struct {
	Kind       string     `json:"kind"`
	OccurredAt *time.Time `json:"occurredAt,omitempty"`
	Subject    string     `json:"subject"`
	Detail     *string    `json:"detail,omitempty"`
}

type TimelineSummaryRequest struct {
	Locale     string        `json:"locale,omitempty"`
	EntityType string        `json:"entityType,omitempty"`
	EntityID   *uuid.UUID    `json:"entityId,omitempty"`
	Items      []ContextItem `json:"items"`
	Consent    *Consent      `json:"consent,omitempty"`
}

type FollowUpDraftRequest struct {
	Locale    string        `json:"locale,omitempty"`
	Channel   string        `json:"channel"`
	Tone      string        `json:"tone,omitempty"`
	Recipient string        `json:"recipient,omitempty"`
	Objective string        `json:"objective"`
	Context   []ContextItem `json:"context"`
	Consent   *Consent      `json:"consent,omitempty"`
}

type NextActionRequest struct {
	Locale     string        `json:"locale,omitempty"`
	EntityType string        `json:"entityType"`
	EntityID   *uuid.UUID    `json:"entityId,omitempty"`
	Goal       string        `json:"goal,omitempty"`
	Context    []ContextItem `json:"context"`
	Consent    *Consent      `json:"consent,omitempty"`
}

type DuplicateRecord struct {
	ID     *uuid.UUID        `json:"id,omitempty"`
	Fields map[string]string `json:"fields"`
}

type DuplicateCandidate struct {
	ID     uuid.UUID         `json:"id"`
	Fields map[string]string `json:"fields"`
}

type DuplicateCandidatesRequest struct {
	Locale     string               `json:"locale,omitempty"`
	EntityType string               `json:"entityType"`
	Subject    DuplicateRecord      `json:"subject"`
	Candidates []DuplicateCandidate `json:"candidates"`
	Consent    *Consent             `json:"consent,omitempty"`
}

type ProviderInfo struct {
	Name  string
	Class ProviderClass
	Model string
}

// Provider is the narrow optional boundary around the four supported,
// advisory-only CRM assistance operations.
type Provider interface {
	Info() ProviderInfo
	TimelineSummary(context.Context, TimelineSummaryRequest) (string, error)
	FollowUpDraft(context.Context, FollowUpDraftRequest) (string, error)
	NextAction(context.Context, NextActionRequest) (string, error)
	DuplicateCandidates(context.Context, DuplicateCandidatesRequest) (string, error)
}

type Result struct {
	Capability Capability `json:"capability"`
	Content    string     `json:"content"`
	Advisory   bool       `json:"advisory"`
}

type Limits struct {
	MaxInputBytes          int64 `json:"maxInputBytes"`
	MaxOutputBytes         int64 `json:"maxOutputBytes"`
	MaxContextItems        int   `json:"maxContextItems"`
	MaxDuplicateCandidates int   `json:"maxDuplicateCandidates"`
}

type Status struct {
	Enabled                    bool           `json:"enabled"`
	Provider                   *string        `json:"provider,omitempty"`
	ProviderClass              *ProviderClass `json:"providerClass,omitempty"`
	RequiresExternalPIIConsent bool           `json:"requiresExternalPiiConsent"`
	Capabilities               []Capability   `json:"capabilities"`
	Limits                     Limits         `json:"limits"`
}
