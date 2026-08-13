package reporting

import (
	"encoding/json"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type Preferences struct {
	Layout            json.RawMessage `json:"layout"`
	PeriodDays        int16           `json:"periodDays"`
	Timezone          *string         `json:"timezone,omitempty"`
	EffectiveTimezone string          `json:"effectiveTimezone"`
	Version           int64           `json:"version"`
	UpdatedAt         *time.Time      `json:"updatedAt,omitempty"`
}

type PreferencesInput struct {
	Layout     json.RawMessage
	PeriodDays int16
	Timezone   *string
}

type Period struct {
	Start    time.Time
	End      time.Time
	Timezone string
}

type Overview struct {
	WonCount           int64   `json:"wonCount"`
	LostCount          int64   `json:"lostCount"`
	WonValueMinor      int64   `json:"wonValueMinor"`
	LeadCount          int64   `json:"leadCount"`
	ConvertedLeadCount int64   `json:"convertedLeadCount"`
	ConversionRate     float64 `json:"conversionRate"`
	ActivityCount      int64   `json:"activityCount"`
}

type StageMetric struct {
	StageID             ids.UUID `json:"stageId"`
	StageName           string   `json:"stageName"`
	Position            int32    `json:"position"`
	DealCount           int64    `json:"dealCount"`
	AmountMinor         int64    `json:"amountMinor"`
	WeightedAmountMinor int64    `json:"weightedAmountMinor"`
}

type OwnerMetric struct {
	OwnerID     *ids.UUID `json:"ownerId,omitempty"`
	OwnerName   string    `json:"ownerName"`
	DealCount   int64     `json:"dealCount"`
	WonCount    int64     `json:"wonCount"`
	LostCount   int64     `json:"lostCount"`
	AmountMinor int64     `json:"amountMinor"`
}

type ActivityDayMetric struct {
	Date         string `json:"date"`
	Count        int64  `json:"count"`
	TaskCount    int64  `json:"taskCount"`
	CallCount    int64  `json:"callCount"`
	MeetingCount int64  `json:"meetingCount"`
	NoteCount    int64  `json:"noteCount"`
}

type LeadSourceMetric struct {
	Source         string `json:"source"`
	LeadCount      int64  `json:"leadCount"`
	ConvertedCount int64  `json:"convertedCount"`
}

type Report struct {
	Period       Period              `json:"period"`
	Overview     Overview            `json:"overview"`
	DealsByStage []StageMetric       `json:"dealsByStage"`
	DealsByOwner []OwnerMetric       `json:"dealsByOwner"`
	Activities   []ActivityDayMetric `json:"activities"`
	LeadSources  []LeadSourceMetric  `json:"leadSources"`
}

type RecentActivity struct {
	ID          ids.UUID   `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	RelatedType *string    `json:"relatedType,omitempty"`
	RelatedID   *ids.UUID  `json:"relatedId,omitempty"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	OccurredAt  time.Time  `json:"occurredAt"`
	DueAt       *time.Time `json:"dueAt,omitempty"`
	AssigneeID  *ids.UUID  `json:"assigneeId,omitempty"`
}
