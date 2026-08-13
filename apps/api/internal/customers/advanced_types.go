package customers

import (
	"encoding/json"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const (
	MaxBulkRecords      = 500
	MaxCSVBytes         = 20 << 20
	MaxCSVRows          = 50_000
	MaxCSVColumns       = 200
	MaxCSVFieldBytes    = 16 << 10
	ImportPreviewRows   = 20
	ImportProcessingLot = 250
)

type CompanyUpdateInput struct {
	Name         string
	Domain       *string
	Industry     *string
	OwnerID      *ids.UUID
	TeamID       *ids.UUID
	Status       string
	Address      map[string]string
	CustomFields map[string]any
}

type DeletedRecord struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	Version     int64     `json:"version"`
	DeletedAt   time.Time `json:"deletedAt"`
	DeletedBy   *string   `json:"deletedBy,omitempty"`
	Email       *string   `json:"email,omitempty"`
	Domain      *string   `json:"domain,omitempty"`
	CompanyID   *string   `json:"companyId,omitempty"`
	OwnerID     *string   `json:"ownerId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DeletedPage struct {
	Items      []DeletedRecord `json:"items"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type CompanyRecord struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Domain       *string           `json:"domain,omitempty"`
	Industry     *string           `json:"industry,omitempty"`
	Status       string            `json:"status"`
	OwnerID      *string           `json:"ownerId,omitempty"`
	TeamID       *string           `json:"teamId,omitempty"`
	Address      map[string]string `json:"address"`
	CustomFields map[string]any    `json:"customFields"`
	Version      int64             `json:"version"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TagInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CustomFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type CustomFieldValidation struct {
	Required  bool     `json:"required,omitempty"`
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Minimum   *float64 `json:"minimum,omitempty"`
	Maximum   *float64 `json:"maximum,omitempty"`
}

type CustomFieldDefinitionInput struct {
	EntityType string                `json:"entityType"`
	FieldKey   string                `json:"fieldKey"`
	Label      string                `json:"label"`
	ValueType  string                `json:"valueType"`
	Validation CustomFieldValidation `json:"validation"`
	Options    []CustomFieldOption   `json:"options"`
}

type CustomFieldDefinition struct {
	ID            string                `json:"id"`
	EntityType    string                `json:"entityType"`
	FieldKey      string                `json:"fieldKey"`
	Label         string                `json:"label"`
	ValueType     string                `json:"valueType"`
	Validation    CustomFieldValidation `json:"validation"`
	Options       []CustomFieldOption   `json:"options"`
	SchemaVersion int32                 `json:"schemaVersion"`
	Version       int64                 `json:"version"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
}

type SavedViewFilter struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
}

type SavedViewSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type SavedViewDefinition struct {
	Filters []SavedViewFilter `json:"filters"`
	Sort    []SavedViewSort   `json:"sort"`
	Columns []string          `json:"columns"`
}

type SavedViewInput struct {
	EntityType string              `json:"entityType"`
	Name       string              `json:"name"`
	Definition SavedViewDefinition `json:"definition"`
	IsShared   bool                `json:"isShared"`
}

type SavedView struct {
	ID         string              `json:"id"`
	OwnerID    string              `json:"ownerId"`
	EntityType string              `json:"entityType"`
	Name       string              `json:"name"`
	Definition SavedViewDefinition `json:"definition"`
	IsShared   bool                `json:"isShared"`
	Version    int64               `json:"version"`
	CreatedAt  time.Time           `json:"createdAt"`
	UpdatedAt  time.Time           `json:"updatedAt"`
}

type VersionedID struct {
	ID      ids.UUID `json:"-"`
	Version int64    `json:"version"`
}

type BulkResult struct {
	OperationID string `json:"operationId"`
	Updated     int    `json:"updated"`
}

type DuplicateCandidate struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	Reason      string  `json:"reason"`
	Score       float64 `json:"score"`
	Email       *string `json:"email,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	Domain      *string `json:"domain,omitempty"`
}

type MergeInput struct {
	SourceID      ids.UUID
	SourceVersion int64
	TargetID      ids.UUID
	TargetVersion int64
}

type MergeResult struct {
	TargetID      string `json:"targetId"`
	TargetVersion int64  `json:"targetVersion"`
	SourceID      string `json:"sourceId"`
	SourceVersion int64  `json:"sourceVersion"`
}

type ContactExportFilter struct {
	Query   string
	Status  string
	OwnerID *ids.UUID
	TagIDs  []ids.UUID
	Sort    string
	Order   string
}

type ImportPreview struct {
	ID               string              `json:"id"`
	EntityType       string              `json:"entityType"`
	Headers          []string            `json:"headers"`
	SampleRows       []map[string]string `json:"sampleRows"`
	TotalRows        int                 `json:"totalRows"`
	Status           string              `json:"status"`
	SuggestedMapping map[string]string   `json:"suggestedMapping"`
}

type ContactImportMapping struct {
	FirstName    string            `json:"firstName"`
	LastName     string            `json:"lastName"`
	Email        string            `json:"email,omitempty"`
	Phone        string            `json:"phone,omitempty"`
	JobTitle     string            `json:"jobTitle,omitempty"`
	CompanyName  string            `json:"companyName,omitempty"`
	OwnerEmail   string            `json:"ownerEmail,omitempty"`
	Status       string            `json:"status,omitempty"`
	Source       string            `json:"source,omitempty"`
	CustomFields map[string]string `json:"customFields,omitempty"`
}

type ImportStatus struct {
	ID            string     `json:"id"`
	EntityType    string     `json:"entityType"`
	Status        string     `json:"status"`
	TotalRows     int        `json:"totalRows"`
	ProcessedRows int        `json:"processedRows"`
	CreatedRows   int        `json:"createdRows"`
	ErrorRows     int        `json:"errorRows"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}
