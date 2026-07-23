package sales

import (
	"encoding/json"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const (
	MaxPageSize         = 100
	DefaultPageSize     = 50
	MaxKanbanPageSize   = 50
	MaxPipelineStages   = 100
	MaxDealLineItems    = 100
	MaxDealParticipants = 100
)

type PipelineInput struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type PipelineRecord struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	DisplayName string                `json:"displayName"`
	IsDefault   bool                  `json:"isDefault"`
	Version     int64                 `json:"version"`
	Stages      []PipelineStageRecord `json:"stages"`
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`
}

type PipelineStageInput struct {
	Name             string `json:"name"`
	Probability      int    `json:"probability"`
	ForecastCategory string `json:"forecastCategory"`
}

type PipelineStageRecord struct {
	ID               string    `json:"id"`
	PipelineID       string    `json:"pipelineId"`
	Name             string    `json:"name"`
	DisplayName      string    `json:"displayName"`
	Probability      int       `json:"probability"`
	ForecastCategory string    `json:"forecastCategory"`
	Position         int       `json:"position"`
	Version          int64     `json:"version"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type StageOrderItem struct {
	ID      ids.UUID `json:"-"`
	Version int64    `json:"version"`
}

type LeadInput struct {
	Name              string         `json:"name"`
	Email             *string        `json:"email"`
	Phone             *string        `json:"phone"`
	CompanyName       *string        `json:"companyName"`
	JobTitle          *string        `json:"jobTitle"`
	Source            *string        `json:"source"`
	Status            string         `json:"status"`
	StageID           *ids.UUID      `json:"-"`
	OwnerID           *ids.UUID      `json:"-"`
	TeamID            *ids.UUID      `json:"-"`
	PlannedStartDate  *time.Time     `json:"plannedStartDate"`
	ExpectedCloseDate *time.Time     `json:"expectedCloseDate"`
	CustomFields      map[string]any `json:"customFields"`
}

type LeadRecord struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Email              *string        `json:"email,omitempty"`
	Phone              *string        `json:"phone,omitempty"`
	CompanyName        *string        `json:"companyName,omitempty"`
	JobTitle           *string        `json:"jobTitle,omitempty"`
	Source             *string        `json:"source,omitempty"`
	Status             string         `json:"status"`
	StageID            string         `json:"stageId"`
	OwnerID            *string        `json:"ownerId,omitempty"`
	TeamID             *string        `json:"teamId,omitempty"`
	ConvertedContactID *string        `json:"convertedContactId,omitempty"`
	ConvertedCompanyID *string        `json:"convertedCompanyId,omitempty"`
	ConvertedDealID    *string        `json:"convertedDealId,omitempty"`
	PlannedStartDate   *string        `json:"plannedStartDate,omitempty"`
	ExpectedCloseDate  *string        `json:"expectedCloseDate,omitempty"`
	CustomFields       map[string]any `json:"customFields"`
	Version            int64          `json:"version"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

type LeadStageInput struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Color    string `json:"color"`
}

type LeadStageRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Category    string    `json:"category"`
	Color       string    `json:"color"`
	Position    int       `json:"position"`
	SystemKey   *string   `json:"systemKey,omitempty"`
	IsDefault   bool      `json:"isDefault"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type LeadPage struct {
	Items      []LeadRecord `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type LeadListFilter struct {
	Query   string
	Status  string
	OwnerID *ids.UUID
	StageID *ids.UUID
	Cursor  string
	Limit   int
}

type LeadConversionReferences struct {
	ContactID *ids.UUID
	CompanyID *ids.UUID
	DealID    *ids.UUID
}

type DeletedSalesRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Version   int64     `json:"version"`
	DeletedAt time.Time `json:"deletedAt"`
	DeletedBy *string   `json:"deletedBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DeletedSalesPage struct {
	Items      []DeletedSalesRecord `json:"items"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

type DealUpdateInput struct {
	Name              string         `json:"name"`
	PipelineID        ids.UUID       `json:"-"`
	StageID           ids.UUID       `json:"-"`
	ContactID         *ids.UUID      `json:"-"`
	CompanyID         *ids.UUID      `json:"-"`
	OwnerID           *ids.UUID      `json:"-"`
	AmountMinor       int64          `json:"amountMinor"`
	Currency          string         `json:"currency"`
	PlannedStartDate  *time.Time     `json:"plannedStartDate"`
	ExpectedCloseDate *time.Time     `json:"expectedCloseDate"`
	ForecastCategory  string         `json:"forecastCategory"`
	CustomFields      map[string]any `json:"customFields"`
}

type DealRecord struct {
	ID                string         `json:"id"`
	PipelineID        string         `json:"pipelineId"`
	StageID           string         `json:"stageId"`
	Name              string         `json:"name"`
	ContactID         *string        `json:"contactId,omitempty"`
	CompanyID         *string        `json:"companyId,omitempty"`
	OwnerID           *string        `json:"ownerId,omitempty"`
	AmountMinor       int64          `json:"amountMinor"`
	Currency          string         `json:"currency"`
	PlannedStartDate  *string        `json:"plannedStartDate,omitempty"`
	ExpectedCloseDate *string        `json:"expectedCloseDate,omitempty"`
	Position          int            `json:"position"`
	Status            string         `json:"status"`
	LostReason        *string        `json:"lostReason,omitempty"`
	ForecastCategory  string         `json:"forecastCategory"`
	WonAt             *time.Time     `json:"wonAt,omitempty"`
	LostAt            *time.Time     `json:"lostAt,omitempty"`
	CustomFields      map[string]any `json:"customFields"`
	Version           int64          `json:"version"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type DealPageAdvanced struct {
	Items      []DealRecord `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type DealListFilter struct {
	Query      string
	PipelineID *ids.UUID
	StageID    *ids.UUID
	OwnerID    *ids.UUID
	Status     string
	Cursor     string
	Limit      int
}

type DealOutcomeInput struct {
	Status           string  `json:"status"`
	LostReason       *string `json:"lostReason"`
	ForecastCategory string  `json:"forecastCategory"`
}

type KanbanCard struct {
	ID                string    `json:"id"`
	PipelineID        string    `json:"pipelineId"`
	StageID           string    `json:"stageId"`
	Name              string    `json:"name"`
	ContactID         *string   `json:"contactId,omitempty"`
	CompanyID         *string   `json:"companyId,omitempty"`
	OwnerID           *string   `json:"ownerId,omitempty"`
	AmountMinor       int64     `json:"amountMinor"`
	Currency          string    `json:"currency"`
	PlannedStartDate  *string   `json:"plannedStartDate,omitempty"`
	ExpectedCloseDate *string   `json:"expectedCloseDate,omitempty"`
	Position          int       `json:"position"`
	ForecastCategory  string    `json:"forecastCategory"`
	Version           int64     `json:"version"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type KanbanPage struct {
	Items      []KanbanCard `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type StageHistoryRecord struct {
	ID          string    `json:"id"`
	DealID      string    `json:"dealId"`
	FromStageID *string   `json:"fromStageId,omitempty"`
	ToStageID   string    `json:"toStageId"`
	ChangedBy   string    `json:"changedBy"`
	ChangedAt   time.Time `json:"changedAt"`
}

type StageHistoryPage struct {
	Items      []StageHistoryRecord `json:"items"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

type LineItemInput struct {
	Name           string `json:"name"`
	Quantity       string `json:"quantity"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
	Currency       string `json:"currency"`
	Position       int    `json:"position"`
}

type LineItemRecord struct {
	ID             string `json:"id"`
	DealID         string `json:"dealId"`
	Name           string `json:"name"`
	Quantity       string `json:"quantity"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
	Currency       string `json:"currency"`
	Position       int    `json:"position"`
	Version        int64  `json:"version"`
}

type DealParticipantInput struct {
	ContactID ids.UUID
	Role      *string `json:"role"`
}

type DealParticipantRecord struct {
	ContactID   string    `json:"contactId"`
	DisplayName string    `json:"displayName"`
	Email       *string   `json:"email,omitempty"`
	Role        *string   `json:"role,omitempty"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func decodeCustomFields(raw []byte) map[string]any {
	result := make(map[string]any)
	if len(raw) == 0 || json.Unmarshal(raw, &result) != nil {
		return map[string]any{}
	}
	return result
}
