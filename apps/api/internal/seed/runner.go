package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type passwordHasher interface {
	Hash(password string) (string, error)
}

type Options struct {
	Environment string
	Email       string
	Password    string
	Progress    io.Writer
}

type Result struct {
	Profile        string
	WorkspaceID    string
	Counts         Counts
	DatasetHash    string
	AlreadyApplied bool
}

func Run(
	ctx context.Context,
	pool *pgxpool.Pool,
	hasher passwordHasher,
	profile Profile,
	options Options,
) (Result, error) {
	if strings.EqualFold(strings.TrimSpace(options.Environment), "production") {
		return Result{}, errors.New("synthetic seed profiles are disabled in production")
	}
	if err := profile.validate(); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(options.Email) == "" || options.Password == "" {
		return Result{}, errors.New("demo email and password are required outside production")
	}
	if pool == nil || hasher == nil {
		return Result{}, errors.New("seed database and password hasher are required")
	}

	ownerID := stableID("global", "user", 0)
	generator := newGenerator(profile, ownerID)
	result := Result{
		Profile:     profile.Name,
		WorkspaceID: generator.workspaceID.String(),
		Counts:      profile.counts(),
		DatasetHash: profile.datasetHash(),
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Result{}, fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = 0"); err != nil {
		return Result{}, fmt.Errorf("disable statement timeout for seed transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "crm-seed:"+profile.Name); err != nil {
		return Result{}, fmt.Errorf("acquire seed lock: %w", err)
	}
	var ledgerAvailable bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass('platform.seed_runs') IS NOT NULL").Scan(&ledgerAvailable); err != nil {
		return Result{}, fmt.Errorf("check seed ledger: %w", err)
	}
	if !ledgerAvailable {
		return Result{}, errors.New("platform.seed_runs is missing; apply database migrations before seeding")
	}

	alreadyApplied, err := inspectLedger(ctx, tx, generator, result)
	if err != nil {
		return Result{}, err
	}
	if alreadyApplied {
		result.AlreadyApplied = true
		if err := tx.Commit(ctx); err != nil {
			return Result{}, fmt.Errorf("commit seed verification: %w", err)
		}
		if err := analyzeSeedTables(ctx, pool); err != nil {
			return Result{}, err
		}
		writeProgress(options.Progress, "profile %s already matches its deterministic seed ledger\n", profile.Name)
		return result, nil
	}

	var workspaceCollision bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM tenancy.workspaces WHERE id = $1 OR slug = $2)", generator.workspaceID.PG(), profile.Slug).Scan(&workspaceCollision); err != nil {
		return Result{}, fmt.Errorf("check seed workspace collision: %w", err)
	}
	if workspaceCollision {
		return Result{}, fmt.Errorf("seed workspace %q exists without a matching ledger; refusing to overwrite it", profile.Slug)
	}

	passwordHash, err := hasher.Hash(options.Password)
	if err != nil {
		return Result{}, fmt.Errorf("hash demo password: %w", err)
	}
	actualOwnerID, err := upsertOwner(ctx, tx, ownerID, options.Email, passwordHash, generator.base)
	if err != nil {
		return Result{}, err
	}
	generator.ownerID = actualOwnerID

	if err := insertFoundation(ctx, tx, generator); err != nil {
		return Result{}, err
	}
	writeProgress(options.Progress, "profile %s: foundation created\n", profile.Name)

	if err := copyDomainData(ctx, tx, generator, options.Progress); err != nil {
		return Result{}, err
	}
	if err := insertDerivedData(ctx, tx, generator); err != nil {
		return Result{}, err
	}

	countsJSON, err := json.Marshal(result.Counts)
	if err != nil {
		return Result{}, fmt.Errorf("encode seed counts: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO platform.seed_runs (profile, seed_version, workspace_id, dataset_hash, counts, completed_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6)`,
		profile.Name, seedVersion, generator.workspaceID.PG(), result.DatasetHash, string(countsJSON), generator.base,
	); err != nil {
		return Result{}, fmt.Errorf("record seed ledger: %w", err)
	}
	if err := verifyCounts(ctx, tx, generator.workspaceID, result.Counts); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit seed transaction: %w", err)
	}
	if err := analyzeSeedTables(ctx, pool); err != nil {
		return Result{}, err
	}
	writeProgress(options.Progress, "profile %s: seed committed\n", profile.Name)
	return result, nil
}

func analyzeSeedTables(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
ANALYZE customers.contacts, customers.companies, sales.deals,
        activities.activities, search.documents, tenancy.memberships,
        tenancy.role_permissions
`); err != nil {
		return fmt.Errorf("analyze seeded tables: %w", err)
	}
	return nil
}

func inspectLedger(ctx context.Context, tx pgx.Tx, generator generator, expected Result) (bool, error) {
	var workspaceID pgtype.UUID
	var datasetHash string
	var countsJSON []byte
	err := tx.QueryRow(ctx, `
SELECT workspace_id, dataset_hash, counts
FROM platform.seed_runs
WHERE profile = $1 AND seed_version = $2`, generator.profile.Name, seedVersion).Scan(&workspaceID, &datasetHash, &countsJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read seed ledger: %w", err)
	}
	storedID, valid := ids.FromPG(workspaceID)
	if !valid || storedID != generator.workspaceID {
		return false, errors.New("seed ledger workspace does not match the deterministic workspace ID")
	}
	if datasetHash != expected.DatasetHash {
		return false, fmt.Errorf("seed profile %q was applied with a different dataset hash; use a new seed version instead of mutating generated data", generator.profile.Name)
	}
	var stored Counts
	if err := json.Unmarshal(countsJSON, &stored); err != nil {
		return false, fmt.Errorf("decode seed ledger counts: %w", err)
	}
	if stored != expected.Counts {
		return false, fmt.Errorf("seed profile %q ledger counts differ from the current contract", generator.profile.Name)
	}
	// The ledger proves which deterministic dataset initialized the workspace.
	// Live CRM rows are intentionally mutable after that point; re-checking the
	// original counts would make every legitimate create/delete prevent the
	// application from restarting with DEMO_SEED enabled.
	return true, nil
}

func verifyCounts(ctx context.Context, tx pgx.Tx, workspaceID ids.UUID, expected Counts) error {
	var actual Counts
	if err := tx.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM customers.contacts WHERE workspace_id = $1),
  (SELECT count(*) FROM customers.companies WHERE workspace_id = $1),
  (SELECT count(*) FROM sales.deals WHERE workspace_id = $1),
  (SELECT count(*) FROM activities.activities WHERE workspace_id = $1)`, workspaceID.PG()).Scan(
		&actual.Contacts,
		&actual.Companies,
		&actual.Deals,
		&actual.Activities,
	); err != nil {
		return fmt.Errorf("verify seed counts: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("seed workspace counts changed: expected %+v, got %+v", expected, actual)
	}
	return nil
}

func upsertOwner(ctx context.Context, tx pgx.Tx, deterministicID ids.UUID, email, passwordHash string, timestamp time.Time) (ids.UUID, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	var actual pgtype.UUID
	err := tx.QueryRow(ctx, `
INSERT INTO identity.users (
  id, email, email_normalized, display_name, password_hash, preferred_locale,
  status, password_changed_at, created_at, updated_at
)
VALUES ($1, $2, $3, 'Demo administrator', $4, 'en', 'active', $5, $5, $5)
ON CONFLICT (email_normalized) DO UPDATE SET
  email = EXCLUDED.email,
  password_hash = EXCLUDED.password_hash,
  status = 'active',
  failed_login_count = 0,
  locked_until = NULL,
  password_changed_at = EXCLUDED.password_changed_at,
  updated_at = EXCLUDED.updated_at
RETURNING id`, deterministicID.PG(), strings.TrimSpace(email), normalizedEmail, passwordHash, timestamp).Scan(&actual)
	if err != nil {
		return ids.UUID{}, fmt.Errorf("upsert demo owner: %w", err)
	}
	ownerID, valid := ids.FromPG(actual)
	if !valid {
		return ids.UUID{}, errors.New("demo owner returned an invalid ID")
	}
	return ownerID, nil
}

func insertFoundation(ctx context.Context, tx pgx.Tx, generator generator) error {
	membershipID := stableID(generator.profile.Name, "membership", 0)
	teamID := stableID(generator.profile.Name, "team", 0)
	if _, err := tx.Exec(ctx, `
INSERT INTO tenancy.workspaces (id, name, slug, default_locale, timezone, default_currency, created_at, updated_at)
VALUES ($1, $2, $3, 'en', 'UTC', 'USD', $4, $4)`,
		generator.workspaceID.PG(), generator.profile.Workspace, generator.profile.Slug, generator.base,
	); err != nil {
		return fmt.Errorf("insert seed workspace: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO tenancy.memberships (workspace_id, id, user_id, role, status, created_at, updated_at)
VALUES ($1, $2, $3, 'owner', 'active', $4, $4)`,
		generator.workspaceID.PG(), membershipID.PG(), generator.ownerID.PG(), generator.base,
	); err != nil {
		return fmt.Errorf("insert seed membership: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO tenancy.teams (workspace_id, id, name, created_at, updated_at)
VALUES ($1, $2, 'Sales team', $3, $3)`, generator.workspaceID.PG(), teamID.PG(), generator.base); err != nil {
		return fmt.Errorf("insert seed team: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO tenancy.team_memberships (workspace_id, team_id, membership_id, created_at)
VALUES ($1, $2, $3, $4)`, generator.workspaceID.PG(), teamID.PG(), membershipID.PG(), generator.base); err != nil {
		return fmt.Errorf("insert seed team membership: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO sales.pipelines (workspace_id, id, name, is_default, created_at, updated_at)
VALUES ($1, $2, 'Sales pipeline', true, $3, $3)`, generator.workspaceID.PG(), generator.pipelineID.PG(), generator.base); err != nil {
		return fmt.Errorf("insert seed pipeline: %w", err)
	}
	stageNames := [...]string{"Qualified", "Discovery", "Proposal", "Negotiation", "Won", "Lost"}
	probabilities := [...]int16{10, 25, 50, 75, 100, 0}
	forecastCategories := [...]string{"pipeline", "pipeline", "best_case", "commit", "closed", "closed"}
	batch := &pgx.Batch{}
	for index, stageID := range generator.stageIDs {
		batch.Queue(`
INSERT INTO sales.pipeline_stages (
  workspace_id, id, pipeline_id, name, probability, forecast_category, position, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
			generator.workspaceID.PG(), stageID.PG(), generator.pipelineID.PG(), stageNames[index], probabilities[index], forecastCategories[index], index, generator.base,
		)
	}
	results := tx.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for index := range generator.stageIDs {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert seed pipeline stage %d: %w", index, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close pipeline stage batch: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO localization.content_resources (
  workspace_id, namespace, resource_key, source_locale, source_text,
  description, placeholders, created_by, updated_by, created_at, updated_at
)
VALUES ($1, 'sales.pipeline.name', $2, 'en', 'Sales pipeline', '', ARRAY[]::text[], $3, $3, $4, $4)`,
		generator.workspaceID.PG(), generator.pipelineID.String(), generator.ownerID.PG(), generator.base); err != nil {
		return fmt.Errorf("insert seed pipeline translation resource: %w", err)
	}
	translationBatch := &pgx.Batch{}
	for index, stageID := range generator.stageIDs {
		translationBatch.Queue(`
INSERT INTO localization.content_resources (
  workspace_id, namespace, resource_key, source_locale, source_text,
  description, placeholders, created_by, updated_by, created_at, updated_at
)
VALUES ($1, 'sales.pipeline_stage.name', $2, 'en', $3, '', ARRAY[]::text[], $4, $4, $5, $5)`,
			generator.workspaceID.PG(), stageID.String(), stageNames[index], generator.ownerID.PG(), generator.base)
	}
	translationResults := tx.SendBatch(ctx, translationBatch)
	for index := range generator.stageIDs {
		if _, err := translationResults.Exec(); err != nil {
			_ = translationResults.Close()
			return fmt.Errorf("insert seed pipeline stage translation resource %d: %w", index, err)
		}
	}
	if err := translationResults.Close(); err != nil {
		return fmt.Errorf("close pipeline stage translation batch: %w", err)
	}
	return nil
}

func copyDomainData(ctx context.Context, tx pgx.Tx, generator generator, progress io.Writer) error {
	profile := generator.profile
	if err := copyGenerated(ctx, tx, pgx.Identifier{"customers", "companies"}, []string{
		"workspace_id", "id", "name", "domain", "domain_normalized", "industry", "status", "owner_user_id",
		"address", "custom_fields", "created_at", "updated_at",
	}, profile.Companies, generator.companyRow); err != nil {
		return err
	}
	writeProgress(progress, "profile %s: %d companies copied\n", profile.Name, profile.Companies)

	if err := copyGenerated(ctx, tx, pgx.Identifier{"customers", "contacts"}, []string{
		"workspace_id", "id", "first_name", "last_name", "display_name", "email", "email_normalized", "phone", "phone_normalized",
		"job_title", "company_id", "owner_user_id", "source", "status", "address", "custom_fields", "last_contacted_at",
		"next_activity_at", "created_at", "updated_at",
	}, profile.Contacts, generator.contactRow); err != nil {
		return err
	}
	writeProgress(progress, "profile %s: %d contacts copied\n", profile.Name, profile.Contacts)

	if err := copyGenerated(ctx, tx, pgx.Identifier{"sales", "deals"}, []string{
		"workspace_id", "id", "pipeline_id", "stage_id", "name", "contact_id", "company_id", "owner_user_id", "amount_minor",
		"currency", "expected_close_date", "position", "status", "lost_reason", "forecast_category", "won_at", "lost_at",
		"custom_fields", "created_at", "updated_at",
	}, profile.Deals, generator.dealRow); err != nil {
		return err
	}
	writeProgress(progress, "profile %s: %d deals copied\n", profile.Name, profile.Deals)

	if err := copyGenerated(ctx, tx, pgx.Identifier{"activities", "activities"}, []string{
		"workspace_id", "id", "activity_type", "title", "body", "related_type", "related_id", "assignee_user_id", "status",
		"priority", "due_at", "occurred_at", "created_by", "completed_at", "created_at", "updated_at",
	}, profile.Activities, generator.activityRow); err != nil {
		return err
	}
	writeProgress(progress, "profile %s: %d activities copied\n", profile.Name, profile.Activities)
	return nil
}

func insertDerivedData(ctx context.Context, tx pgx.Tx, generator generator) error {
	profile := generator.profile
	searchColumns := []string{"workspace_id", "entity_type", "entity_id", "title", "subtitle", "searchable_text", "rank_boost", "updated_at"}
	for _, source := range []struct {
		count int64
		row   func(int64) []any
	}{
		{profile.Companies, generator.companySearchRow},
		{profile.Contacts, generator.contactSearchRow},
		{profile.Deals, generator.dealSearchRow},
		{profile.Activities / 4, generator.noteSearchRow},
	} {
		if err := copyGenerated(ctx, tx, pgx.Identifier{"search", "documents"}, searchColumns, source.count, source.row); err != nil {
			return err
		}
	}
	if err := copyGenerated(ctx, tx, pgx.Identifier{"sales", "deal_stage_history"}, []string{
		"workspace_id", "id", "deal_id", "to_stage_id", "changed_by", "changed_at",
	}, profile.Deals, func(index int64) []any {
		_, stageID, _ := generator.dealStatusAndStage(index)
		return []any{
			generator.workspaceID.PG(),
			stableID(profile.Name, "history", index).PG(),
			generator.dealID(index).PG(),
			stageID.PG(),
			generator.ownerID.PG(),
			generator.timestamp("history", index),
		}
	}); err != nil {
		return err
	}

	var openPipeline int64
	var weightedForecast int64
	var wonCount int64
	var lostCount int64
	stageCounts := make([]int64, len(generator.stageIDs))
	stageAmounts := make([]int64, len(generator.stageIDs))
	for index := int64(0); index < profile.Deals; index++ {
		status, stageID, _ := generator.dealStatusAndStage(index)
		stageIndex := 0
		for candidate, id := range generator.stageIDs {
			if id == stageID {
				stageIndex = candidate
				break
			}
		}
		amount := int64(50_000 + (index%2_000)*2_500)
		switch status {
		case "won":
			wonCount++
		case "lost":
			lostCount++
		default:
			stageCounts[stageIndex]++
			stageAmounts[stageIndex] += amount
			openPipeline += amount
			probabilities := [...]int64{10, 25, 50, 75, 100, 0}
			weightedForecast += amount * probabilities[stageIndex] / 100
		}
	}
	stageNames := [...]string{"Qualified", "Discovery", "Proposal", "Negotiation", "Won", "Lost"}
	stageSummaryRows := make([]struct {
		StageID     string `json:"stageId"`
		StageName   string `json:"stageName"`
		Count       int64  `json:"count"`
		AmountMinor int64  `json:"amountMinor"`
	}, len(generator.stageIDs))
	for index, stageID := range generator.stageIDs {
		stageSummaryRows[index].StageID = stageID.String()
		stageSummaryRows[index].StageName = stageNames[index]
		stageSummaryRows[index].Count = stageCounts[index]
		stageSummaryRows[index].AmountMinor = stageAmounts[index]
	}
	stageSummary, err := json.Marshal(stageSummaryRows)
	if err != nil {
		return fmt.Errorf("encode deal stage summary: %w", err)
	}
	// Every fourth generated activity is a task. One fifth of those tasks is
	// completed, so open overdue tasks equal activities / 5 for all profiles.
	overdueTasks := profile.Activities / 5
	if _, err := tx.Exec(ctx, `
INSERT INTO reporting.dashboard_summaries (
  workspace_id, currency, open_pipeline_minor, weighted_forecast_minor, won_count, lost_count,
  overdue_tasks, deals_by_stage, computed_at, source_version
)
VALUES ($1, 'USD', $2, $3, $4, $5, $6, $7::jsonb, $8, 1)`,
		generator.workspaceID.PG(), openPipeline, weightedForecast, wonCount, lostCount, overdueTasks, string(stageSummary), generator.base,
	); err != nil {
		return fmt.Errorf("insert dashboard summary: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit.events (
  workspace_id, id, actor_user_id, action, entity_type, entity_id, request_id, summary, occurred_at
)
VALUES ($1, $2, $3, 'seed.completed', 'workspace', $1, $4, $5::jsonb, $6)`,
		generator.workspaceID.PG(), stableID(profile.Name, "audit", 0).PG(), generator.ownerID.PG(),
		"seed-"+profile.Name, `{"synthetic":true,"profile":"`+profile.Name+`"}`, generator.base,
	); err != nil {
		return fmt.Errorf("insert seed audit event: %w", err)
	}
	return nil
}

func writeProgress(output io.Writer, format string, values ...any) {
	if output == nil {
		return
	}
	_, _ = fmt.Fprintf(output, format, values...)
}
