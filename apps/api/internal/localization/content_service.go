package localization

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const (
	maxContentTextLength = 8192
	maxContentPageSize   = 100
)

var (
	localePattern      = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	namespacePattern   = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
	resourceKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*$`)
	placeholderPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)
)

type ContentService struct {
	installedLocales map[string]struct{}
}

type ContentTranslationInput struct {
	SourceLocale   string
	SourceText     string
	Description    string
	TranslatedText string
	Status         string
	Version        int64
}

type ContentResourceInput struct {
	SourceLocale string
	SourceText   string
	Description  string
}

type ContentTranslation struct {
	Namespace          string    `json:"namespace"`
	ResourceKey        string    `json:"key"`
	SourceLocale       string    `json:"sourceLocale"`
	SourceText         string    `json:"sourceText"`
	Description        string    `json:"description"`
	Placeholders       []string  `json:"placeholders"`
	ResourceVersion    int64     `json:"resourceVersion"`
	Locale             string    `json:"locale"`
	TranslatedText     string    `json:"translatedText"`
	Status             string    `json:"status"`
	TranslationVersion int64     `json:"version"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type ContentTranslationPage struct {
	Items      []ContentTranslation `json:"items"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

type TranslationCoverage struct {
	Namespace string `json:"namespace"`
	Total     int64  `json:"total"`
	Published int64  `json:"published"`
	Draft     int64  `json:"draft"`
	Missing   int64  `json:"missing"`
}

type WorkspaceLocaleSettings struct {
	DefaultLocale    string    `json:"defaultLocale"`
	SupportedLocales []string  `json:"supportedLocales"`
	Version          int64     `json:"version"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ResolvedContent struct {
	Text   string `json:"text"`
	Locale string `json:"locale"`
}

type contentCursor struct {
	Namespace   string `json:"n"`
	ResourceKey string `json:"k"`
	FilterHash  string `json:"f"`
}

func NewContentService(installedLocales []string) *ContentService {
	installed := make(map[string]struct{}, len(installedLocales))
	for _, locale := range installedLocales {
		if normalized, ok := NormalizeLocale(locale); ok {
			installed[normalized] = struct{}{}
		}
	}
	return &ContentService{installedLocales: installed}
}

// NormalizeLocale accepts a conservative BCP 47 subset and stores it in a
// case-insensitive canonical form. Catalog filenames and database values use
// the same representation, so locale lookup never depends on host casing.
func NormalizeLocale(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !localePattern.MatchString(value) || len(value) > 35 {
		return "", false
	}
	return strings.ToLower(value), true
}

func (service *ContentService) List(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	locale, namespace, status, query, cursor string,
	limit int,
) (ContentTranslationPage, error) {
	locale, err := service.validateWorkspaceLocale(ctx, workspace, workspaceID, locale)
	if err != nil {
		return ContentTranslationPage{}, err
	}
	namespace = strings.TrimSpace(namespace)
	status = strings.TrimSpace(status)
	query = strings.TrimSpace(query)
	if namespace != "" && (len(namespace) > 64 || !namespacePattern.MatchString(namespace)) {
		return ContentTranslationPage{}, validation("/query/namespace", "translation.namespace.invalid")
	}
	if status != "" && status != "missing" && status != "draft" && status != "published" {
		return ContentTranslationPage{}, validation("/query/status", "translation.status.invalid")
	}
	if len(query) > 160 {
		return ContentTranslationPage{}, validation("/query/q", "validation.length")
	}
	if limit < 1 {
		limit = 50
	}
	if limit > maxContentPageSize {
		limit = maxContentPageSize
	}
	filter := strings.Join([]string{locale, namespace, status, query}, "\x00")
	cursorNamespace, cursorKey, err := decodeContentCursor(cursor, filter)
	if err != nil {
		return ContentTranslationPage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListContentTranslations(ctx, dbgen.ListContentTranslationsParams{
		Locale: locale, WorkspaceID: workspaceID.PG(), NamespaceFilter: namespace,
		StatusFilter: status, SearchQuery: query, CursorNamespace: cursorNamespace,
		CursorResourceKey: cursorKey, PageLimit: int32(limit + 1),
	})
	if err != nil {
		return ContentTranslationPage{}, fmt.Errorf("list content translations: %w", err)
	}
	page := ContentTranslationPage{Items: make([]ContentTranslation, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			last := rows[limit-1]
			page.NextCursor, err = encodeContentCursor(last.Namespace, last.ResourceKey, filter)
			if err != nil {
				return ContentTranslationPage{}, err
			}
			break
		}
		item := ContentTranslation{
			Namespace: row.Namespace, ResourceKey: row.ResourceKey,
			SourceLocale: row.SourceLocale, SourceText: row.SourceText,
			Description: row.Description, Placeholders: row.Placeholders,
			ResourceVersion: row.ResourceVersion, Locale: locale,
			Status: "missing", UpdatedAt: row.UpdatedAt.Time.UTC(),
		}
		if row.TranslatedText != nil {
			item.TranslatedText = *row.TranslatedText
		}
		if row.Status != nil {
			item.Status = *row.Status
		}
		if row.TranslationVersion != nil {
			item.TranslationVersion = *row.TranslationVersion
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

func (service *ContentService) Coverage(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	locale string,
) ([]TranslationCoverage, error) {
	locale, err := service.validateWorkspaceLocale(ctx, workspace, workspaceID, locale)
	if err != nil {
		return nil, err
	}
	rows, err := workspace.Queries.ContentTranslationCoverage(ctx, dbgen.ContentTranslationCoverageParams{
		WorkspaceID: workspaceID.PG(), Locale: locale,
	})
	if err != nil {
		return nil, fmt.Errorf("translation coverage: %w", err)
	}
	result := make([]TranslationCoverage, len(rows))
	for index, row := range rows {
		result[index] = TranslationCoverage(row)
	}
	return result, nil
}

func (service *ContentService) Settings(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
) (WorkspaceLocaleSettings, error) {
	row, err := workspace.Queries.GetWorkspaceLocalizationSettings(ctx, workspaceID.PG())
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceLocaleSettings{}, errx.ErrNotFound
	}
	if err != nil {
		return WorkspaceLocaleSettings{}, fmt.Errorf("load workspace locale settings: %w", err)
	}
	return WorkspaceLocaleSettings{
		DefaultLocale: row.DefaultLocale, SupportedLocales: row.SupportedLocales,
		Version: row.Version, UpdatedAt: row.UpdatedAt.Time.UTC(),
	}, nil
}

func (service *ContentService) UpdateSettings(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	defaultLocale string,
	supportedLocales []string,
	expectedVersion int64,
) (WorkspaceLocaleSettings, error) {
	if expectedVersion < 1 {
		return WorkspaceLocaleSettings{}, validation("/version", "validation.range")
	}
	normalizedDefault, ok := NormalizeLocale(defaultLocale)
	if !ok {
		return WorkspaceLocaleSettings{}, validation("/defaultLocale", "translation.locale.invalid")
	}
	if len(supportedLocales) < 1 || len(supportedLocales) > 20 {
		return WorkspaceLocaleSettings{}, validation("/supportedLocales", "validation.range")
	}
	normalized := make([]string, 0, len(supportedLocales))
	seen := make(map[string]struct{}, len(supportedLocales))
	defaultIncluded := false
	for index, locale := range supportedLocales {
		candidate, valid := NormalizeLocale(locale)
		if !valid {
			return WorkspaceLocaleSettings{}, validation(fmt.Sprintf("/supportedLocales/%d", index), "translation.locale.invalid")
		}
		if len(service.installedLocales) > 0 {
			if _, installed := service.installedLocales[candidate]; !installed {
				return WorkspaceLocaleSettings{}, validation(fmt.Sprintf("/supportedLocales/%d", index), "translation.locale.not_installed")
			}
		}
		if _, duplicate := seen[candidate]; duplicate {
			return WorkspaceLocaleSettings{}, validation(fmt.Sprintf("/supportedLocales/%d", index), "translation.locale.duplicate")
		}
		seen[candidate] = struct{}{}
		normalized = append(normalized, candidate)
		defaultIncluded = defaultIncluded || candidate == normalizedDefault
	}
	if !defaultIncluded {
		return WorkspaceLocaleSettings{}, validation("/defaultLocale", "translation.locale.default_not_supported")
	}
	row, err := workspace.Queries.UpdateWorkspaceLocales(ctx, dbgen.UpdateWorkspaceLocalesParams{
		DefaultLocale: normalizedDefault, SupportedLocales: normalized,
		WorkspaceID: metadata.WorkspaceID.PG(), ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkspaceLocaleSettings{}, errx.ErrVersionConflict
	}
	if err != nil {
		return WorkspaceLocaleSettings{}, fmt.Errorf("update workspace locales: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "workspace.locales.updated", EventType: "tenancy.workspace.locales_updated",
		AggregateType: "workspace", AggregateID: metadata.WorkspaceID,
		Summary: map[string]any{"defaultLocale": normalizedDefault, "supportedLocales": normalized},
		Payload: map[string]any{"workspaceId": metadata.WorkspaceID.String(), "version": row.Version},
	}); err != nil {
		return WorkspaceLocaleSettings{}, err
	}
	return WorkspaceLocaleSettings{
		DefaultLocale: row.DefaultLocale, SupportedLocales: row.SupportedLocales,
		Version: row.Version, UpdatedAt: row.UpdatedAt.Time.UTC(),
	}, nil
}

func (service *ContentService) Put(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	locale, namespace, resourceKey string,
	input ContentTranslationInput,
) (ContentTranslation, error) {
	locale, err := service.validateWorkspaceLocale(ctx, workspace, metadata.WorkspaceID, locale)
	if err != nil {
		return ContentTranslation{}, err
	}
	input.SourceLocale, err = service.validateWorkspaceLocale(ctx, workspace, metadata.WorkspaceID, input.SourceLocale)
	if err != nil {
		return ContentTranslation{}, validation("/sourceLocale", "translation.locale.unsupported")
	}
	if err := validateResourceIdentity(namespace, resourceKey); err != nil {
		return ContentTranslation{}, err
	}
	input.SourceText = strings.TrimSpace(input.SourceText)
	input.TranslatedText = strings.TrimSpace(input.TranslatedText)
	input.Description = strings.TrimSpace(input.Description)
	if len(input.SourceText) == 0 || len(input.SourceText) > maxContentTextLength {
		return ContentTranslation{}, validation("/sourceText", "validation.length")
	}
	if len(input.TranslatedText) == 0 || len(input.TranslatedText) > maxContentTextLength {
		return ContentTranslation{}, validation("/translatedText", "validation.length")
	}
	if len(input.Description) > 1000 {
		return ContentTranslation{}, validation("/description", "validation.length")
	}
	if input.Status != "draft" && input.Status != "published" {
		return ContentTranslation{}, validation("/status", "translation.status.invalid")
	}
	if input.Version < 0 {
		return ContentTranslation{}, validation("/version", "validation.range")
	}
	sourcePlaceholders, err := ExtractPlaceholders(input.SourceText)
	if err != nil {
		return ContentTranslation{}, validation("/sourceText", "translation.placeholders.invalid")
	}
	translatedPlaceholders, err := ExtractPlaceholders(input.TranslatedText)
	if err != nil || !equalStrings(sourcePlaceholders, translatedPlaceholders) {
		return ContentTranslation{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/translatedText", Code: "translation.placeholders.mismatch",
			Params: map[string]any{"expected": sourcePlaceholders},
		}}}
	}
	resource, err := service.ensureResource(ctx, workspace, metadata, namespace, resourceKey, input, sourcePlaceholders)
	if err != nil {
		return ContentTranslation{}, err
	}

	var translation dbgen.LocalizationContentTranslation
	if input.Version == 0 {
		translation, err = workspace.Queries.CreateContentTranslation(ctx, dbgen.CreateContentTranslationParams{
			WorkspaceID: metadata.WorkspaceID.PG(), Namespace: namespace,
			ResourceKey: resourceKey, Locale: locale, TranslatedText: input.TranslatedText,
			Status: input.Status, ActorID: metadata.ActorID.PG(),
		})
	} else {
		translation, err = workspace.Queries.UpdateContentTranslation(ctx, dbgen.UpdateContentTranslationParams{
			WorkspaceID: metadata.WorkspaceID.PG(), Namespace: namespace,
			ResourceKey: resourceKey, Locale: locale, TranslatedText: input.TranslatedText,
			Status: input.Status, ActorID: metadata.ActorID.PG(), ExpectedVersion: input.Version,
		})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ContentTranslation{}, errx.ErrVersionConflict
	}
	if err != nil {
		return ContentTranslation{}, fmt.Errorf("write content translation: %w", err)
	}
	entityID := translationEntityID(metadata.WorkspaceID, namespace, resourceKey, locale)
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "translation." + input.Status, EventType: "localization.translation.updated",
		AggregateType: "content_translation", AggregateID: entityID,
		Summary: map[string]any{"namespace": namespace, "key": resourceKey, "locale": locale, "status": input.Status},
		Payload: map[string]any{"namespace": namespace, "key": resourceKey, "locale": locale, "version": translation.Version},
	}); err != nil {
		return ContentTranslation{}, err
	}
	return ContentTranslation{
		Namespace: namespace, ResourceKey: resourceKey, SourceLocale: resource.SourceLocale,
		SourceText: resource.SourceText, Description: resource.Description,
		Placeholders: resource.Placeholders, ResourceVersion: resource.Version,
		Locale: locale, TranslatedText: translation.TranslatedText,
		Status: translation.Status, TranslationVersion: translation.Version,
		UpdatedAt: translation.UpdatedAt.Time.UTC(),
	}, nil
}

func (service *ContentService) Resolve(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	locale, namespace, resourceKey string,
) (ResolvedContent, error) {
	locale, err := service.validateWorkspaceLocale(ctx, workspace, workspaceID, locale)
	if err != nil {
		return ResolvedContent{}, err
	}
	if err := validateResourceIdentity(namespace, resourceKey); err != nil {
		return ResolvedContent{}, err
	}
	settings, err := workspace.Queries.GetWorkspaceLocalizationSettings(ctx, workspaceID.PG())
	if err != nil {
		return ResolvedContent{}, fmt.Errorf("load workspace locale settings: %w", err)
	}
	row, err := workspace.Queries.ResolvePublishedContent(ctx, dbgen.ResolvePublishedContentParams{
		WorkspaceID: workspaceID.PG(), RequestedLocale: locale,
		FallbackLocale: settings.DefaultLocale, Namespace: namespace, ResourceKey: resourceKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ResolvedContent{}, errx.ErrNotFound
	}
	if err != nil {
		return ResolvedContent{}, fmt.Errorf("resolve translated content: %w", err)
	}
	return ResolvedContent{Text: row.ResolvedText, Locale: row.ResolvedLocale}, nil
}

// EffectiveLocale resolves a user's preference against the workspace locale
// policy. A stale or unsupported preference safely falls back to the workspace
// default instead of breaking an otherwise valid read response.
func (service *ContentService) EffectiveLocale(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	preferred string,
) (string, error) {
	settings, err := workspace.Queries.GetWorkspaceLocalizationSettings(ctx, workspaceID.PG())
	if err != nil {
		return "", fmt.Errorf("load workspace locales: %w", err)
	}
	normalized, valid := NormalizeLocale(preferred)
	if valid {
		for _, supported := range settings.SupportedLocales {
			candidate, candidateValid := NormalizeLocale(supported)
			if candidateValid && candidate == normalized {
				return normalized, nil
			}
		}
	}
	return settings.DefaultLocale, nil
}

// RegisterResource connects domain-owned content to the workspace translation
// workflow. When source semantics change, published translations return to
// draft so an old translation is never silently served for a renamed label.
func (service *ContentService) RegisterResource(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	namespace, resourceKey string,
	input ContentResourceInput,
) (dbgen.LocalizationContentResource, error) {
	var err error
	input.SourceLocale, err = service.validateWorkspaceLocale(
		ctx, workspace, metadata.WorkspaceID, input.SourceLocale,
	)
	if err != nil {
		return dbgen.LocalizationContentResource{}, validation("/sourceLocale", "translation.locale.unsupported")
	}
	if err := validateResourceIdentity(namespace, resourceKey); err != nil {
		return dbgen.LocalizationContentResource{}, err
	}
	input.SourceText = strings.TrimSpace(input.SourceText)
	input.Description = strings.TrimSpace(input.Description)
	if len(input.SourceText) == 0 || len(input.SourceText) > maxContentTextLength {
		return dbgen.LocalizationContentResource{}, validation("/sourceText", "validation.length")
	}
	if len(input.Description) > 1000 {
		return dbgen.LocalizationContentResource{}, validation("/description", "validation.length")
	}
	placeholders, err := ExtractPlaceholders(input.SourceText)
	if err != nil {
		return dbgen.LocalizationContentResource{}, validation("/sourceText", "translation.placeholders.invalid")
	}
	params := dbgen.GetContentResourceParams{
		WorkspaceID: metadata.WorkspaceID.PG(), Namespace: namespace, ResourceKey: resourceKey,
	}
	resource, err := workspace.Queries.GetContentResource(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		resource, err = workspace.Queries.CreateContentResource(ctx, dbgen.CreateContentResourceParams{
			WorkspaceID: metadata.WorkspaceID.PG(), Namespace: namespace, ResourceKey: resourceKey,
			SourceLocale: input.SourceLocale, SourceText: input.SourceText,
			Description: input.Description, Placeholders: placeholders, ActorID: metadata.ActorID.PG(),
		})
	}
	if err != nil {
		return dbgen.LocalizationContentResource{}, fmt.Errorf("register content resource: %w", err)
	}
	sourceChanged := resource.SourceLocale != input.SourceLocale ||
		resource.SourceText != input.SourceText || !equalStrings(resource.Placeholders, placeholders)
	if !sourceChanged && resource.Description == input.Description {
		return resource, nil
	}
	resource, err = workspace.Queries.UpdateContentResourceSource(ctx, dbgen.UpdateContentResourceSourceParams{
		SourceLocale: input.SourceLocale, SourceText: input.SourceText,
		Description: input.Description, Placeholders: placeholders, ActorID: metadata.ActorID.PG(),
		WorkspaceID: metadata.WorkspaceID.PG(), Namespace: namespace, ResourceKey: resourceKey,
	})
	if err != nil {
		return dbgen.LocalizationContentResource{}, fmt.Errorf("update content resource source: %w", err)
	}
	if sourceChanged {
		if err := workspace.Queries.MarkContentTranslationsDraft(ctx, dbgen.MarkContentTranslationsDraftParams{
			ActorID: metadata.ActorID.PG(), WorkspaceID: metadata.WorkspaceID.PG(),
			Namespace: namespace, ResourceKey: resourceKey,
		}); err != nil {
			return dbgen.LocalizationContentResource{}, fmt.Errorf("invalidate changed source translations: %w", err)
		}
	}
	return resource, nil
}

// ResolveBatch performs one bounded query per namespace, avoiding N+1 reads
// when rendering configurable pipeline and stage labels.
func (service *ContentService) ResolveBatch(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	preferredLocale, namespace string,
	resourceKeys []string,
) (map[string]ResolvedContent, error) {
	result := make(map[string]ResolvedContent, len(resourceKeys))
	if len(resourceKeys) == 0 {
		return result, nil
	}
	if len(resourceKeys) > 500 {
		return nil, validation("/keys", "validation.max_items")
	}
	if len(namespace) < 1 || len(namespace) > 64 || !namespacePattern.MatchString(namespace) {
		return nil, validation("/namespace", "translation.namespace.invalid")
	}
	unique := make([]string, 0, len(resourceKeys))
	seen := make(map[string]struct{}, len(resourceKeys))
	for _, key := range resourceKeys {
		if len(key) < 1 || len(key) > 160 || !resourceKeyPattern.MatchString(key) {
			return nil, validation("/keys", "translation.key.invalid")
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}
	settings, err := workspace.Queries.GetWorkspaceLocalizationSettings(ctx, workspaceID.PG())
	if err != nil {
		return nil, fmt.Errorf("load workspace locales: %w", err)
	}
	requested := settings.DefaultLocale
	if normalized, valid := NormalizeLocale(preferredLocale); valid {
		for _, supported := range settings.SupportedLocales {
			candidate, candidateValid := NormalizeLocale(supported)
			if candidateValid && candidate == normalized {
				requested = normalized
				break
			}
		}
	}
	rows, err := workspace.Queries.ResolvePublishedContents(ctx, dbgen.ResolvePublishedContentsParams{
		WorkspaceID: workspaceID.PG(), RequestedLocale: requested,
		FallbackLocale: settings.DefaultLocale, Namespace: namespace, ResourceKeys: unique,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve translated content batch: %w", err)
	}
	for _, row := range rows {
		result[row.ResourceKey] = ResolvedContent{Text: row.ResolvedText, Locale: row.ResolvedLocale}
	}
	return result, nil
}

// DeleteResources removes domain-owned translation resources in the same
// transaction as their owning records, so the translation center cannot
// accumulate orphaned content.
func (service *ContentService) DeleteResources(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	namespace string,
	resourceKeys []string,
) error {
	if len(resourceKeys) == 0 {
		return nil
	}
	if len(resourceKeys) > 500 {
		return validation("/keys", "validation.max_items")
	}
	if len(namespace) < 1 || len(namespace) > 64 || !namespacePattern.MatchString(namespace) {
		return validation("/namespace", "translation.namespace.invalid")
	}
	for _, key := range resourceKeys {
		if len(key) < 1 || len(key) > 160 || !resourceKeyPattern.MatchString(key) {
			return validation("/keys", "translation.key.invalid")
		}
	}
	if err := workspace.Queries.DeleteContentResources(ctx, dbgen.DeleteContentResourcesParams{
		WorkspaceID: workspaceID.PG(), Namespace: namespace, ResourceKeys: resourceKeys,
	}); err != nil {
		return fmt.Errorf("delete content resources: %w", err)
	}
	return nil
}

func (service *ContentService) ensureResource(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	namespace, resourceKey string,
	input ContentTranslationInput,
	placeholders []string,
) (dbgen.LocalizationContentResource, error) {
	params := dbgen.GetContentResourceParams{WorkspaceID: metadata.WorkspaceID.PG(), Namespace: namespace, ResourceKey: resourceKey}
	resource, err := workspace.Queries.GetContentResource(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		resource, err = workspace.Queries.CreateContentResource(ctx, dbgen.CreateContentResourceParams{
			WorkspaceID: metadata.WorkspaceID.PG(), Namespace: namespace, ResourceKey: resourceKey,
			SourceLocale: input.SourceLocale, SourceText: input.SourceText,
			Description: input.Description, Placeholders: placeholders, ActorID: metadata.ActorID.PG(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			resource, err = workspace.Queries.GetContentResource(ctx, params)
		}
	}
	if err != nil {
		return dbgen.LocalizationContentResource{}, fmt.Errorf("ensure content resource: %w", err)
	}
	if resource.SourceLocale != input.SourceLocale || resource.SourceText != input.SourceText ||
		!equalStrings(resource.Placeholders, placeholders) {
		return dbgen.LocalizationContentResource{}, validation("/sourceText", "translation.source.changed")
	}
	return resource, nil
}

func (service *ContentService) validateWorkspaceLocale(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	locale string,
) (string, error) {
	settings, err := workspace.Queries.GetWorkspaceLocalizationSettings(ctx, workspaceID.PG())
	if err != nil {
		return "", fmt.Errorf("load workspace locales: %w", err)
	}
	if strings.TrimSpace(locale) == "" {
		locale = settings.DefaultLocale
	}
	normalized, ok := NormalizeLocale(locale)
	if !ok {
		return "", validation("/locale", "translation.locale.invalid")
	}
	if len(service.installedLocales) > 0 {
		if _, exists := service.installedLocales[normalized]; !exists {
			return "", validation("/locale", "translation.locale.not_installed")
		}
	}
	for _, supported := range settings.SupportedLocales {
		candidate, valid := NormalizeLocale(supported)
		if valid && candidate == normalized {
			return normalized, nil
		}
	}
	return "", validation("/locale", "translation.locale.unsupported")
}

func validateResourceIdentity(namespace, resourceKey string) error {
	if len(namespace) < 1 || len(namespace) > 64 || !namespacePattern.MatchString(namespace) {
		return validation("/namespace", "translation.namespace.invalid")
	}
	if len(resourceKey) < 1 || len(resourceKey) > 160 || !resourceKeyPattern.MatchString(resourceKey) {
		return validation("/key", "translation.key.invalid")
	}
	return nil
}

// ExtractPlaceholders understands named `{placeholder}` tokens and escaped
// braces (`{{` and `}}`). Exact set equality is required before publication,
// preventing translated emails and notifications from dropping data fields.
func ExtractPlaceholders(message string) ([]string, error) {
	found := make(map[string]struct{})
	for index := 0; index < len(message); {
		switch message[index] {
		case '{':
			if index+1 < len(message) && message[index+1] == '{' {
				index += 2
				continue
			}
			end := strings.IndexByte(message[index+1:], '}')
			if end < 0 {
				return nil, errors.New("unclosed placeholder")
			}
			end += index + 1
			name := message[index+1 : end]
			if !placeholderPattern.MatchString(name) {
				return nil, errors.New("invalid placeholder")
			}
			found[name] = struct{}{}
			index = end + 1
		case '}':
			if index+1 < len(message) && message[index+1] == '}' {
				index += 2
				continue
			}
			return nil, errors.New("unmatched closing brace")
		default:
			index++
		}
	}
	result := make([]string, 0, len(found))
	for name := range found {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func encodeContentCursor(namespace, resourceKey, filter string) (string, error) {
	payload, err := json.Marshal(contentCursor{Namespace: namespace, ResourceKey: resourceKey, FilterHash: contentFilterHash(filter)})
	if err != nil {
		return "", fmt.Errorf("encode translation cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeContentCursor(cursor, filter string) (string, string, error) {
	if cursor == "" {
		return "", "", nil
	}
	if len(cursor) > 1024 {
		return "", "", errors.New("cursor too long")
	}
	payload, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", err
	}
	var decoded contentCursor
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", "", err
	}
	if decoded.FilterHash != contentFilterHash(filter) || validateResourceIdentity(decoded.Namespace, decoded.ResourceKey) != nil {
		return "", "", errors.New("cursor filter mismatch")
	}
	return decoded.Namespace, decoded.ResourceKey, nil
}

func contentFilterHash(filter string) string {
	hash := sha256.Sum256([]byte(filter))
	return base64.RawURLEncoding.EncodeToString(hash[:12])
}

func translationEntityID(workspaceID ids.UUID, namespace, resourceKey, locale string) ids.UUID {
	digest := sha256.Sum256([]byte(workspaceID.String() + "\x00" + namespace + "\x00" + resourceKey + "\x00" + locale))
	var id ids.UUID
	copy(id[:], digest[:16])
	id[6] = (id[6] & 0x0f) | 0x50
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
