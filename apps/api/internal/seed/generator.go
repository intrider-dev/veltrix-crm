package seed

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

var syntheticEpoch = time.Date(2025, time.January, 1, 9, 0, 0, 0, time.UTC)

type generator struct {
	profile     Profile
	workspaceID ids.UUID
	ownerID     ids.UUID
	pipelineID  ids.UUID
	stageIDs    [6]ids.UUID
	base        time.Time
}

func newGenerator(profile Profile, ownerID ids.UUID) generator {
	base := syntheticEpoch.Add(time.Duration(profileOrdinal(profile.Name)) * 24 * time.Hour)
	result := generator{
		profile:     profile,
		workspaceID: stableID(profile.Name, "workspace", 0),
		ownerID:     ownerID,
		pipelineID:  stableID(profile.Name, "pipeline", 0),
		base:        base,
	}
	for index := range result.stageIDs {
		result.stageIDs[index] = stableID(profile.Name, "stage", int64(index))
	}
	return result
}

func profileOrdinal(name string) int {
	switch name {
	case "demo":
		return 0
	case "small":
		return 1
	case "benchmark":
		return 2
	default:
		return 9
	}
}

func stableID(scope, kind string, index int64) ids.UUID {
	seed := fmt.Sprintf("%s:%s:%s:%d", generatorContract, scope, kind, index)
	digest := sha256.Sum256([]byte(seed))
	base := syntheticEpoch.Add(time.Duration(profileOrdinal(scope))*24*time.Hour + time.Duration(kindOrdinal(kind))*time.Hour)
	milliseconds := uint64(base.UnixMilli()) + uint64(index)

	var id ids.UUID
	id[0] = byte(milliseconds >> 40)
	id[1] = byte(milliseconds >> 32)
	id[2] = byte(milliseconds >> 24)
	id[3] = byte(milliseconds >> 16)
	id[4] = byte(milliseconds >> 8)
	id[5] = byte(milliseconds)
	copy(id[6:], digest[:10])
	id[6] = (id[6] & 0x0f) | 0x70
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}

func kindOrdinal(kind string) int {
	switch kind {
	case "user":
		return 0
	case "workspace":
		return 1
	case "membership":
		return 2
	case "team":
		return 3
	case "pipeline":
		return 4
	case "stage":
		return 5
	case "company":
		return 6
	case "contact":
		return 7
	case "deal":
		return 8
	case "activity":
		return 9
	case "history":
		return 10
	case "audit":
		return 11
	default:
		return 12
	}
}

func (g generator) companyID(index int64) ids.UUID {
	return stableID(g.profile.Name, "company", index)
}

func (g generator) contactID(index int64) ids.UUID {
	return stableID(g.profile.Name, "contact", index)
}

func (g generator) dealID(index int64) ids.UUID {
	return stableID(g.profile.Name, "deal", index)
}

func (g generator) activityID(index int64) ids.UUID {
	return stableID(g.profile.Name, "activity", index)
}

func (g generator) timestamp(kind string, index int64) time.Time {
	return g.base.Add(time.Duration(kindOrdinal(kind))*time.Hour + time.Duration(index)*time.Millisecond)
}

func (g generator) companyRow(index int64) []any {
	number := index + 1
	name := fmt.Sprintf("Synthetic Company %06d", number)
	domain := fmt.Sprintf("company-%06d.example.invalid", number)
	createdAt := g.timestamp("company", index)
	industries := [...]string{"Software", "Manufacturing", "Services", "Retail", "Research"}
	return []any{
		g.workspaceID.PG(),
		g.companyID(index).PG(),
		name,
		domain,
		domain,
		industries[index%int64(len(industries))],
		"active",
		g.ownerID.PG(),
		`{"synthetic":true}`,
		`{"seed_profile":"` + g.profile.Name + `"}`,
		createdAt,
		createdAt,
	}
}

func (g generator) contactRow(index int64) []any {
	number := index + 1
	firstName := "Synthetic"
	lastName := fmt.Sprintf("Contact %06d", number)
	displayName := firstName + " " + lastName
	email := fmt.Sprintf("contact-%06d@example.invalid", number)
	phone := fmt.Sprintf("+1555%07d", number)
	createdAt := g.timestamp("contact", index)
	lastContactedAt := createdAt.Add(48 * time.Hour)
	nextActivityAt := createdAt.Add(7 * 24 * time.Hour)
	jobTitles := [...]string{"Account lead", "Operations manager", "Founder", "Sales manager", "Research lead"}
	return []any{
		g.workspaceID.PG(),
		g.contactID(index).PG(),
		firstName,
		lastName,
		displayName,
		email,
		email,
		phone,
		phone,
		jobTitles[index%int64(len(jobTitles))],
		g.companyID(index % g.profile.Companies).PG(),
		g.ownerID.PG(),
		"synthetic_seed",
		"active",
		`{"country":"ZZ"}`,
		`{"synthetic":true}`,
		lastContactedAt,
		nextActivityAt,
		createdAt,
		createdAt,
	}
}

func (g generator) dealStatusAndStage(index int64) (string, ids.UUID, any) {
	if index%10 == 0 {
		return "won", g.stageIDs[4], nil
	}
	if index%13 == 0 {
		return "lost", g.stageIDs[5], "Synthetic budget timing"
	}
	return "open", g.stageIDs[index%4], nil
}

func (g generator) dealRow(index int64) []any {
	number := index + 1
	status, stageID, lostReason := g.dealStatusAndStage(index)
	createdAt := g.timestamp("deal", index)
	closeDate := g.base.AddDate(0, int(index%6)+1, int(index%23))
	forecastCategory := "pipeline"
	var wonAt any
	var lostAt any
	if status == "won" {
		forecastCategory = "closed"
		wonAt = createdAt.Add(14 * 24 * time.Hour)
	} else if status == "lost" {
		forecastCategory = "closed"
		lostAt = createdAt.Add(14 * 24 * time.Hour)
	}
	return []any{
		g.workspaceID.PG(),
		g.dealID(index).PG(),
		g.pipelineID.PG(),
		stageID.PG(),
		fmt.Sprintf("Synthetic opportunity %06d", number),
		g.contactID(index % g.profile.Contacts).PG(),
		g.companyID(index % g.profile.Companies).PG(),
		g.ownerID.PG(),
		int64(50_000 + (index%2_000)*2_500),
		"USD",
		closeDate,
		int(index % 2_000),
		status,
		lostReason,
		forecastCategory,
		wonAt,
		lostAt,
		`{"synthetic":true}`,
		createdAt,
		createdAt,
	}
}

func (g generator) activityRow(index int64) []any {
	types := [...]string{"task", "call", "meeting", "note"}
	activityType := types[index%int64(len(types))]
	createdAt := g.timestamp("activity", index)
	status := "open"
	var completedAt any
	if activityType == "note" || index%5 == 0 {
		status = "completed"
		completedAt = createdAt.Add(time.Hour)
	}
	priorities := [...]string{"normal", "high", "low"}
	relatedType, relatedID := g.activityRelation(index)
	return []any{
		g.workspaceID.PG(),
		g.activityID(index).PG(),
		activityType,
		fmt.Sprintf("Synthetic %s %06d", activityType, index+1),
		fmt.Sprintf("Generated %s content for deterministic testing.", activityType),
		relatedType,
		relatedID.PG(),
		g.ownerID.PG(),
		status,
		priorities[index%int64(len(priorities))],
		createdAt.Add(72 * time.Hour),
		createdAt,
		g.ownerID.PG(),
		completedAt,
		createdAt,
		createdAt,
	}
}

func (g generator) activityRelation(index int64) (string, ids.UUID) {
	switch index % 3 {
	case 0:
		return "contact", g.contactID(index % g.profile.Contacts)
	case 1:
		return "company", g.companyID(index % g.profile.Companies)
	default:
		return "deal", g.dealID(index % g.profile.Deals)
	}
}

func (g generator) companySearchRow(index int64) []any {
	name := fmt.Sprintf("Synthetic Company %06d", index+1)
	domain := fmt.Sprintf("company-%06d.example.invalid", index+1)
	return []any{g.workspaceID.PG(), "company", g.companyID(index).PG(), name, domain, name + " " + domain, float32(1), g.timestamp("company", index)}
}

func (g generator) contactSearchRow(index int64) []any {
	name := fmt.Sprintf("Synthetic Contact %06d", index+1)
	email := fmt.Sprintf("contact-%06d@example.invalid", index+1)
	return []any{g.workspaceID.PG(), "contact", g.contactID(index).PG(), name, email, name + " " + email, float32(1), g.timestamp("contact", index)}
}

func (g generator) dealSearchRow(index int64) []any {
	name := fmt.Sprintf("Synthetic opportunity %06d", index+1)
	return []any{g.workspaceID.PG(), "deal", g.dealID(index).PG(), name, "USD", name, float32(1), g.timestamp("deal", index)}
}

func (g generator) noteSearchRow(noteIndex int64) []any {
	activityIndex := noteIndex*4 + 3
	title := fmt.Sprintf("Synthetic note %06d", activityIndex+1)
	body := "Generated note content for deterministic testing."
	return []any{g.workspaceID.PG(), "note", g.activityID(activityIndex).PG(), title, nil, title + " " + body, float32(0.8), g.timestamp("activity", activityIndex)}
}
