package assignment

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const MaxItems = 100

type Input struct {
	Kind        string
	SubjectType string
	SubjectID   ids.UUID
	IsPrimary   bool
}

type Item struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	SubjectType string    `json:"subjectType"`
	SubjectID   string    `json:"subjectId"`
	DisplayName string    `json:"displayName"`
	IsPrimary   bool      `json:"isPrimary"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Set struct {
	Items   []Item `json:"items"`
	Version int64  `json:"version"`
}

type SubjectOption struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

func Validate(items []Input) error {
	if len(items) > MaxItems {
		return fieldError("/assignments", "validation.items.range")
	}
	seen := make(map[string]struct{}, len(items))
	primaryCount := 0
	for index, item := range items {
		if item.Kind != "responsible" && item.Kind != "watcher" {
			return fieldError(fmt.Sprintf("/assignments/%d/kind", index), "validation.enum")
		}
		if item.SubjectType != "user" && item.SubjectType != "department" {
			return fieldError(fmt.Sprintf("/assignments/%d/subjectType", index), "validation.enum")
		}
		if item.SubjectID == (ids.UUID{}) {
			return fieldError(fmt.Sprintf("/assignments/%d/subjectId", index), "validation.uuid.invalid")
		}
		if item.IsPrimary {
			if item.Kind != "responsible" {
				return fieldError(fmt.Sprintf("/assignments/%d/isPrimary", index), "validation.assignment.primary_responsible")
			}
			primaryCount++
		}
		key := item.Kind + ":" + item.SubjectType + ":" + item.SubjectID.String()
		if _, duplicate := seen[key]; duplicate {
			return fieldError(fmt.Sprintf("/assignments/%d", index), "validation.duplicate")
		}
		seen[key] = struct{}{}
	}
	if primaryCount > 1 {
		return fieldError("/assignments", "validation.assignment.primary_unique")
	}
	return nil
}

func NewItem(
	id pgtype.UUID,
	kind string,
	userID, departmentID pgtype.UUID,
	isPrimary bool,
	userName, departmentName *string,
	createdAt time.Time,
) Item {
	assignmentID, _ := ids.FromPG(id)
	subjectType := "user"
	subjectID, _ := ids.FromPG(userID)
	displayName := dereference(userName)
	if !userID.Valid {
		subjectType = "department"
		subjectID, _ = ids.FromPG(departmentID)
		displayName = dereference(departmentName)
	}
	return Item{
		ID: assignmentID.String(), Kind: kind, SubjectType: subjectType,
		SubjectID: subjectID.String(), DisplayName: displayName, IsPrimary: isPrimary,
		CreatedAt: createdAt.UTC(),
	}
}

func fieldError(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
