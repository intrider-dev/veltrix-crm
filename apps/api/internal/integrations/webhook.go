package integrations

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/pagination"
)

const (
	WebhookSignatureVersion = "v1"
	WebhookSecretPurpose    = "webhook-signing"
	maxWebhookEvents        = 50
)

var eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_]*){1,7}$`)

type WebhookSubscription struct {
	WorkspaceID   ids.UUID
	ID            ids.UUID
	URL           string
	EventTypes    []string
	Enabled       bool
	Version       int64
	SecretVersion int
	Timeout       time.Duration
	MaxAttempts   int
	CreatedBy     ids.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type WebhookCreate struct {
	WorkspaceID ids.UUID
	CreatedBy   ids.UUID
	URL         string
	EventTypes  []string
	Enabled     bool
	Timeout     time.Duration
	MaxAttempts int
}

type GeneratedWebhook struct {
	Subscription  WebhookSubscription `json:"subscription"`
	SigningSecret string              `json:"signingSecret"`
}

type WebhookSecretRecord struct {
	Subscription  WebhookSubscription
	Current       identity.SecretEnvelope
	Previous      *identity.SecretEnvelope
	PreviousUntil *time.Time
}

type WebhookDeliveryLog struct {
	ID               ids.UUID   `json:"id"`
	SubscriptionID   ids.UUID   `json:"subscriptionId"`
	EventID          ids.UUID   `json:"eventId"`
	Status           string     `json:"status"`
	Attempts         int        `json:"attempts"`
	NextAttemptAt    *time.Time `json:"nextAttemptAt,omitempty"`
	ResponseStatus   *int32     `json:"responseStatus,omitempty"`
	ResponseSummary  *string    `json:"responseSummary,omitempty"`
	DeliveredAt      *time.Time `json:"deliveredAt,omitempty"`
	RequestTimestamp *int64     `json:"requestTimestamp,omitempty"`
	SignatureVersion int32      `json:"signatureVersion"`
	LastErrorCode    *string    `json:"lastErrorCode,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type WebhookDeliveryPage struct {
	Items      []WebhookDeliveryLog `json:"items"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

type WebhookRepository interface {
	CreateWebhook(context.Context, WebhookSubscription, identity.SecretEnvelope) (WebhookSubscription, error)
	ListWebhooks(context.Context, ids.UUID, int) ([]WebhookSubscription, error)
	RotateWebhookSecret(context.Context, ids.UUID, ids.UUID, int64, identity.SecretEnvelope, time.Duration) (WebhookSubscription, error)
	SetWebhookEnabled(context.Context, ids.UUID, ids.UUID, int64, bool) (WebhookSubscription, bool, error)
	RetryWebhookDelivery(context.Context, ids.UUID, ids.UUID) (bool, error)
	ListWebhookDeliveries(context.Context, ids.UUID, *ids.UUID, time.Time, ids.UUID, int) ([]WebhookDeliveryLog, error)
}

func (service *WebhookService) List(ctx context.Context, workspaceID ids.UUID, limit int) ([]WebhookSubscription, error) {
	if service == nil || service.repository == nil {
		return nil, errors.New("webhook repository is required")
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	return service.repository.ListWebhooks(ctx, workspaceID, limit)
}

func (service *WebhookService) ListDeliveries(
	ctx context.Context, workspaceID ids.UUID, subscriptionID *ids.UUID, cursor string, limit int,
) (WebhookDeliveryPage, error) {
	if service == nil || service.repository == nil {
		return WebhookDeliveryPage{}, errors.New("webhook repository is required")
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	filter := "webhook-deliveries:all"
	if subscriptionID != nil {
		filter = "webhook-deliveries:" + subscriptionID.String()
	}
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return WebhookDeliveryPage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := service.repository.ListWebhookDeliveries(
		ctx, workspaceID, subscriptionID, cursorTime, cursorID, limit+1,
	)
	if err != nil {
		return WebhookDeliveryPage{}, err
	}
	page := WebhookDeliveryPage{Items: make([]WebhookDeliveryLog, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			break
		}
		page.Items = append(page.Items, row)
	}
	if len(rows) > limit {
		last := rows[limit-1]
		page.NextCursor, err = pagination.Encode(last.CreatedAt, last.ID, filter)
		if err != nil {
			return WebhookDeliveryPage{}, err
		}
	}
	return page, nil
}

func (service *WebhookService) SetEnabled(
	ctx context.Context, workspaceID, subscriptionID ids.UUID, version int64, enabled bool,
) (WebhookSubscription, error) {
	if service == nil || service.repository == nil {
		return WebhookSubscription{}, errors.New("webhook repository is required")
	}
	updated, found, err := service.repository.SetWebhookEnabled(ctx, workspaceID, subscriptionID, version, enabled)
	if err != nil {
		return WebhookSubscription{}, err
	}
	if !found {
		return WebhookSubscription{}, errx.ErrVersionConflict
	}
	return updated, nil
}

func (service *WebhookService) RetryDelivery(ctx context.Context, workspaceID, deliveryID ids.UUID) error {
	if service == nil || service.repository == nil {
		return errors.New("webhook repository is required")
	}
	retried, err := service.repository.RetryWebhookDelivery(ctx, workspaceID, deliveryID)
	if err != nil {
		return err
	}
	if !retried {
		return errx.ErrNotFound
	}
	return nil
}

type WebhookService struct {
	repository WebhookRepository
	cipher     identity.SecretCipher
	validator  URLValidator
}

func NewWebhookService(repository WebhookRepository, cipher identity.SecretCipher, validator URLValidator) *WebhookService {
	return &WebhookService{repository: repository, cipher: cipher, validator: validator}
}

func (service *WebhookService) Create(ctx context.Context, input WebhookCreate) (GeneratedWebhook, error) {
	if service == nil || service.repository == nil || service.cipher == nil {
		return GeneratedWebhook{}, errors.New("webhook service is not configured")
	}
	if _, err := service.validator.Validate(ctx, input.URL); err != nil {
		return GeneratedWebhook{}, validation("/url", "webhook.url.rejected")
	}
	events, err := normalizeEventTypes(input.EventTypes)
	if err != nil {
		return GeneratedWebhook{}, err
	}
	if input.Timeout == 0 {
		input.Timeout = 10 * time.Second
	}
	if input.Timeout < time.Second || input.Timeout > 30*time.Second {
		return GeneratedWebhook{}, validation("/timeoutSeconds", "validation.range")
	}
	if input.MaxAttempts == 0 {
		input.MaxAttempts = 8
	}
	if input.MaxAttempts < 1 || input.MaxAttempts > 20 {
		return GeneratedWebhook{}, validation("/maxAttempts", "validation.range")
	}
	id, err := ids.NewV7()
	if err != nil {
		return GeneratedWebhook{}, err
	}
	secret, encoded, err := generateWebhookSecret()
	if err != nil {
		return GeneratedWebhook{}, err
	}
	defer clear(secret)
	envelope, err := service.cipher.Encrypt(ctx, WebhookSecretPurpose, webhookSubject(input.WorkspaceID, id), secret)
	if err != nil {
		return GeneratedWebhook{}, err
	}
	now := time.Now().UTC()
	subscription := WebhookSubscription{
		WorkspaceID: input.WorkspaceID, ID: id, URL: input.URL, EventTypes: events,
		Enabled: input.Enabled, Version: 1, SecretVersion: 1, Timeout: input.Timeout,
		MaxAttempts: input.MaxAttempts, CreatedBy: input.CreatedBy, CreatedAt: now, UpdatedAt: now,
	}
	created, err := service.repository.CreateWebhook(ctx, subscription, envelope)
	if err != nil {
		return GeneratedWebhook{}, err
	}
	return GeneratedWebhook{Subscription: created, SigningSecret: encoded}, nil
}

func (service *WebhookService) RotateSecret(
	ctx context.Context, workspaceID, subscriptionID ids.UUID, version int64, overlap time.Duration,
) (GeneratedWebhook, error) {
	if service == nil || service.repository == nil || service.cipher == nil {
		return GeneratedWebhook{}, errors.New("webhook service is not configured")
	}
	if overlap < 0 || overlap > 24*time.Hour {
		return GeneratedWebhook{}, validation("/overlapSeconds", "validation.range")
	}
	secret, encoded, err := generateWebhookSecret()
	if err != nil {
		return GeneratedWebhook{}, err
	}
	defer clear(secret)
	envelope, err := service.cipher.Encrypt(ctx, WebhookSecretPurpose, webhookSubject(workspaceID, subscriptionID), secret)
	if err != nil {
		return GeneratedWebhook{}, err
	}
	updated, err := service.repository.RotateWebhookSecret(ctx, workspaceID, subscriptionID, version, envelope, overlap)
	if err != nil {
		return GeneratedWebhook{}, err
	}
	return GeneratedWebhook{Subscription: updated, SigningSecret: encoded}, nil
}

func SignWebhook(secret []byte, timestamp time.Time, eventID string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp.UTC().Unix(), 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(eventID))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return WebhookSignatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))
}

type ReplayStore interface {
	Reserve(context.Context, string, time.Time) (bool, error)
}

func VerifyWebhook(
	ctx context.Context,
	secret []byte,
	now time.Time,
	tolerance time.Duration,
	eventID, timestampHeader, signatureHeader string,
	body []byte,
	replay ReplayStore,
) error {
	if len(eventID) < 1 || len(eventID) > 200 || len(body) > 262144 {
		return errors.New("invalid webhook envelope")
	}
	seconds, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return errors.New("invalid webhook timestamp")
	}
	timestamp := time.Unix(seconds, 0).UTC()
	if tolerance <= 0 || tolerance > 15*time.Minute {
		tolerance = 5 * time.Minute
	}
	delta := now.UTC().Sub(timestamp)
	if delta < -tolerance || delta > tolerance {
		return errors.New("webhook timestamp outside tolerance")
	}
	wanted := SignWebhook(secret, timestamp, eventID, body)
	if !signatureHeaderContains(signatureHeader, wanted) {
		return errors.New("invalid webhook signature")
	}
	if replay != nil {
		reserved, err := replay.Reserve(ctx, eventID, timestamp.Add(tolerance))
		if err != nil {
			return fmt.Errorf("reserve webhook replay key: %w", err)
		}
		if !reserved {
			return errors.New("webhook event was already accepted")
		}
	}
	return nil
}

func signatureHeaderContains(header, wanted string) bool {
	parts := strings.Split(header, ",")
	if len(parts) > 4 {
		return false
	}
	matched := 0
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if len(candidate) == len(wanted) {
			matched |= subtle.ConstantTimeCompare([]byte(candidate), []byte(wanted))
		} else {
			dummy := make([]byte, len(wanted))
			copy(dummy, candidate)
			matched |= subtle.ConstantTimeCompare(dummy, []byte(wanted)) & 0
		}
	}
	return matched == 1
}

func DecryptCurrentWebhookSecret(ctx context.Context, cipher identity.SecretCipher, record WebhookSecretRecord) ([]byte, error) {
	if cipher == nil {
		return nil, errors.New("webhook secret cipher is required")
	}
	return cipher.Decrypt(ctx, WebhookSecretPurpose, webhookSubject(record.Subscription.WorkspaceID, record.Subscription.ID), record.Current)
}

func generateWebhookSecret() ([]byte, string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, "", fmt.Errorf("generate webhook secret: %w", err)
	}
	return secret, "whsec_" + base64.RawURLEncoding.EncodeToString(secret), nil
}

func DecodeWebhookSecret(value string) ([]byte, error) {
	if !strings.HasPrefix(value, "whsec_") {
		return nil, errors.New("invalid webhook secret")
	}
	secret, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(value, "whsec_"))
	if err != nil || len(secret) != 32 {
		clear(secret)
		return nil, errors.New("invalid webhook secret")
	}
	return secret, nil
}

func normalizeEventTypes(values []string) ([]string, error) {
	if len(values) < 1 || len(values) > maxWebhookEvents {
		return nil, validation("/eventTypes", "validation.array.size")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) < 3 || len(value) > 120 || !eventTypePattern.MatchString(value) {
			return nil, validation("/eventTypes", "validation.format")
		}
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func webhookSubject(workspaceID, subscriptionID ids.UUID) string {
	return workspaceID.String() + "/" + subscriptionID.String()
}
