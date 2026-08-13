package integrations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const apiKeyVersion = "crm1"

type Scope string

const (
	ScopeContactsRead    Scope = "contacts.read"
	ScopeContactsWrite   Scope = "contacts.write"
	ScopeCompaniesRead   Scope = "companies.read"
	ScopeCompaniesWrite  Scope = "companies.write"
	ScopeDealsRead       Scope = "deals.read"
	ScopeDealsWrite      Scope = "deals.write"
	ScopeActivitiesRead  Scope = "activities.read"
	ScopeActivitiesWrite Scope = "activities.write"
	ScopeReportsRead     Scope = "reports.read"
	ScopeWebhooksWrite   Scope = "webhooks.write"
)

var apiKeyNamePattern = regexp.MustCompile(`^[^\p{Cc}\p{Cf}]{1,120}$`)

type APIKey struct {
	WorkspaceID ids.UUID   `json:"workspaceId"`
	ID          ids.UUID   `json:"id"`
	Prefix      string     `json:"prefix"`
	Name        string     `json:"name"`
	Scopes      []Scope    `json:"scopes"`
	CreatedBy   ids.UUID   `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
}

type GeneratedAPIKey struct {
	APIKey APIKey `json:"apiKey"`
	Token  string `json:"token"`
}

type APIKeyCredential struct {
	WorkspaceID ids.UUID
	ID          ids.UUID
	Prefix      string
	SecretHash  [32]byte
	Scopes      []Scope
	CreatedBy   ids.UUID
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
}

type APIKeyCreate struct {
	WorkspaceID ids.UUID
	CreatedBy   ids.UUID
	Name        string
	Scopes      []Scope
	ExpiresAt   *time.Time
	Now         time.Time
}

type APIKeyRepository interface {
	CreateAPIKey(context.Context, APIKey, [32]byte) (APIKey, error)
	ListAPIKeys(context.Context, ids.UUID, int) ([]APIKey, error)
	LookupAPIKey(context.Context, ids.UUID, string) (APIKeyCredential, bool, error)
	TouchAPIKey(context.Context, ids.UUID, ids.UUID) error
	RevokeAPIKey(context.Context, ids.UUID, ids.UUID) (bool, error)
}

type APIKeyService struct {
	repository APIKeyRepository
}

func NewAPIKeyService(repository APIKeyRepository) *APIKeyService {
	return &APIKeyService{repository: repository}
}

func (service *APIKeyService) Create(ctx context.Context, input APIKeyCreate) (GeneratedAPIKey, error) {
	if service == nil || service.repository == nil {
		return GeneratedAPIKey{}, errors.New("API key repository is required")
	}
	name := strings.TrimSpace(input.Name)
	if !apiKeyNamePattern.MatchString(name) {
		return GeneratedAPIKey{}, validation("/name", "validation.length")
	}
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil {
		return GeneratedAPIKey{}, err
	}
	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
		return GeneratedAPIKey{}, validation("/expiresAt", "validation.date.future")
	}
	id, err := ids.NewV7()
	if err != nil {
		return GeneratedAPIKey{}, err
	}
	prefixBytes := make([]byte, 8)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(prefixBytes); err != nil {
		return GeneratedAPIKey{}, fmt.Errorf("generate API key prefix: %w", err)
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return GeneratedAPIKey{}, fmt.Errorf("generate API key secret: %w", err)
	}
	defer clear(secretBytes)
	prefix := base64.RawURLEncoding.EncodeToString(prefixBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	token := strings.Join([]string{apiKeyVersion, input.WorkspaceID.String(), prefix, secret}, ".")
	hash := hashAPIKeyToken(token)
	key := APIKey{
		WorkspaceID: input.WorkspaceID, ID: id, Prefix: prefix, Name: name,
		Scopes: scopes, CreatedBy: input.CreatedBy, CreatedAt: now, ExpiresAt: input.ExpiresAt,
	}
	created, err := service.repository.CreateAPIKey(ctx, key, hash)
	if err != nil {
		return GeneratedAPIKey{}, err
	}
	return GeneratedAPIKey{APIKey: created, Token: token}, nil
}

type AuthenticatedAPIKey struct {
	WorkspaceID ids.UUID
	KeyID       ids.UUID
	CreatedBy   ids.UUID
	Scopes      []Scope
	ExpiresAt   *time.Time
}

func (service *APIKeyService) Authenticate(ctx context.Context, token string, now time.Time) (AuthenticatedAPIKey, error) {
	if service == nil || service.repository == nil {
		return AuthenticatedAPIKey{}, errx.ErrUnauthenticated
	}
	workspaceID, prefix, parseOK := parseAPIKeyToken(token)
	credential := APIKeyCredential{}
	found := false
	var lookupErr error
	if parseOK {
		credential, found, lookupErr = service.repository.LookupAPIKey(ctx, workspaceID, prefix)
	}
	wanted := hashAPIKeyToken(token)
	stored := credential.SecretHash
	if lookupErr != nil || !found {
		// Always perform a fixed-size comparison, including lookup misses.
		stored = sha256.Sum256([]byte("crm-api-key-dummy-v1"))
	}
	matched := subtle.ConstantTimeCompare(wanted[:], stored[:]) == 1
	if now.IsZero() {
		now = time.Now().UTC()
	}
	active := credential.WorkspaceID == workspaceID && credential.Prefix == prefix &&
		credential.RevokedAt == nil && (credential.ExpiresAt == nil || credential.ExpiresAt.After(now))
	if lookupErr != nil {
		return AuthenticatedAPIKey{}, fmt.Errorf("lookup API key: %w", lookupErr)
	}
	if !parseOK || !found || !matched || !active {
		return AuthenticatedAPIKey{}, errx.ErrUnauthenticated
	}
	if err := service.repository.TouchAPIKey(ctx, credential.WorkspaceID, credential.ID); err != nil {
		return AuthenticatedAPIKey{}, fmt.Errorf("touch API key: %w", err)
	}
	return AuthenticatedAPIKey{
		WorkspaceID: credential.WorkspaceID, KeyID: credential.ID, CreatedBy: credential.CreatedBy,
		Scopes: append([]Scope(nil), credential.Scopes...), ExpiresAt: credential.ExpiresAt,
	}, nil
}

func (service *APIKeyService) List(ctx context.Context, workspaceID ids.UUID, limit int) ([]APIKey, error) {
	if service == nil || service.repository == nil {
		return nil, errors.New("API key repository is required")
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	return service.repository.ListAPIKeys(ctx, workspaceID, limit)
}

func (service *APIKeyService) Revoke(ctx context.Context, workspaceID, keyID ids.UUID) error {
	if service == nil || service.repository == nil {
		return errors.New("API key repository is required")
	}
	revoked, err := service.repository.RevokeAPIKey(ctx, workspaceID, keyID)
	if err != nil {
		return err
	}
	if !revoked {
		return errx.ErrNotFound
	}
	return nil
}

func HasScope(granted []Scope, required Scope) bool {
	for _, scope := range granted {
		if scope == required {
			return true
		}
	}
	return false
}

func parseAPIKeyToken(token string) (ids.UUID, string, bool) {
	if len(token) < 80 || len(token) > 220 || strings.ContainsAny(token, "\r\n\t ") {
		return ids.UUID{}, "", false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != apiKeyVersion || len(parts[2]) < 8 || len(parts[2]) > 32 {
		return ids.UUID{}, "", false
	}
	workspaceID, err := ids.Parse(parts[1])
	if err != nil {
		return ids.UUID{}, "", false
	}
	if _, err := base64.RawURLEncoding.Strict().DecodeString(parts[2]); err != nil {
		return ids.UUID{}, "", false
	}
	secret, err := base64.RawURLEncoding.Strict().DecodeString(parts[3])
	if err != nil || len(secret) != 32 {
		return ids.UUID{}, "", false
	}
	clear(secret)
	return workspaceID, parts[2], true
}

func hashAPIKeyToken(token string) [32]byte {
	return sha256.Sum256([]byte("crm-api-key-v1\x00" + token))
}

func normalizeScopes(scopes []Scope) ([]Scope, error) {
	if len(scopes) < 1 || len(scopes) > 20 {
		return nil, validation("/scopes", "validation.array.size")
	}
	seen := make(map[Scope]struct{}, len(scopes))
	for _, scope := range scopes {
		if !validScope(scope) {
			return nil, validation("/scopes", "validation.enum")
		}
		seen[scope] = struct{}{}
	}
	result := make([]Scope, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func validScope(scope Scope) bool {
	switch scope {
	case ScopeContactsRead, ScopeContactsWrite, ScopeCompaniesRead, ScopeCompaniesWrite,
		ScopeDealsRead, ScopeDealsWrite, ScopeActivitiesRead, ScopeActivitiesWrite,
		ScopeReportsRead, ScopeWebhooksWrite:
		return true
	default:
		return false
	}
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
