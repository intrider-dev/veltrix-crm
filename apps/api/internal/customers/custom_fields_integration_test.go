//go:build integration

package customers_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
	appmigrations "github.com/veltrixcrm/veltrix-crm/apps/api/migrations"
)

func TestLeadCustomFieldUsageBlocksSchemaChangeAndDeleteCleansAggregate(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, adminURL, appPassword); err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(ctx, adminURL, 1, "lead-custom-fields-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 1, "lead-custom-fields-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID, userID := mustCompanyListID(t), mustCompanyListID(t)
	membershipID, leadID := mustCompanyListID(t), mustCompanyListID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	if _, err = admin.Exec(ctx, `INSERT INTO identity.users(
id,email,email_normalized,display_name,password_hash
) VALUES($1,$2,$2,'Lead Field Owner','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')`,
		userID.PG(), "lead-fields-"+suffix+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug)
VALUES($1,'Lead Field Workspace',$2)`, workspaceID.PG(), "lead-fields-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role)
VALUES($1,$2,$3,'owner')`, workspaceID.PG(), membershipID.PG(), userID.PG()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id=$1`, userID.PG())
	}()

	service := customers.NewService()
	tenantService := tenancy.NewService(appPool)
	correlationID := mustCompanyListID(t)
	metadata := events.Metadata{
		WorkspaceID: workspaceID, ActorID: userID, RequestID: "lead-custom-fields",
		CorrelationID: correlationID,
	}
	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: userID}, workspaceID,
		"lead-custom-fields", tenancy.PermissionSettingsWrite, func(workspace *tenancy.WorkspaceTx) error {
			definition, createErr := service.CreateCustomFieldDefinition(ctx, workspace, metadata,
				customers.CustomFieldDefinitionInput{
					EntityType: "lead", FieldKey: "tier", Label: "Tier", ValueType: "text",
					Validation: customers.CustomFieldValidation{}, Options: []customers.CustomFieldOption{},
				})
			if createErr != nil {
				return createErr
			}
			var stageID pgtype.UUID
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT id FROM sales.lead_stages
				WHERE workspace_id=$1 AND category='new' AND is_default`, workspaceID.PG()).Scan(&stageID); queryErr != nil {
				return queryErr
			}
			if _, insertErr := workspace.Tx.Exec(ctx, `INSERT INTO sales.leads(
				workspace_id,id,name,status,stage_id,custom_fields
			) VALUES($1,$2,'Normalized field lead','new',$3,'{"tier":"gold"}'::jsonb)`,
				workspaceID.PG(), leadID.PG(), stageID); insertErr != nil {
				return insertErr
			}
			validated, validationErr := service.ValidateCustomFields(ctx, workspace, workspaceID,
				"lead", map[string]any{"tier": "gold"})
			if validationErr != nil {
				return validationErr
			}
			if persistErr := service.PersistValidatedCustomFields(ctx, workspace, workspaceID,
				"lead", leadID, validated); persistErr != nil {
				return persistErr
			}
			var valueCount int
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM customers.custom_field_values
				WHERE workspace_id=$1 AND definition_id=$2 AND entity_id=$3`,
				workspaceID.PG(), ids.MustParse(definition.ID).PG(), leadID.PG()).Scan(&valueCount); queryErr != nil {
				return queryErr
			}
			if valueCount != 1 {
				t.Fatalf("normalized lead custom field count=%d, want 1", valueCount)
			}
			_, updateErr := service.UpdateCustomFieldDefinition(ctx, workspace, metadata,
				ids.MustParse(definition.ID), definition.Version, customers.CustomFieldDefinitionInput{
					EntityType: "lead", FieldKey: "tier", Label: "Tier", ValueType: "number",
					Validation: customers.CustomFieldValidation{}, Options: []customers.CustomFieldOption{},
				})
			var fieldErr *errx.ValidationError
			if !errors.As(updateErr, &fieldErr) || len(fieldErr.Fields) == 0 ||
				fieldErr.Fields[0].Code != "validation.custom_field.migration_required" {
				t.Fatalf("incompatible schema update error=%v", updateErr)
			}
			if deleteErr := service.DeleteCustomFieldDefinition(ctx, workspace, metadata,
				ids.MustParse(definition.ID), definition.Version); deleteErr != nil {
				return deleteErr
			}
			var stale bool
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT custom_fields ? 'tier' FROM sales.leads
				WHERE workspace_id=$1 AND id=$2`, workspaceID.PG(), leadID.PG()).Scan(&stale); queryErr != nil {
				return queryErr
			}
			if stale {
				t.Fatal("deleted lead custom field remained in denormalized aggregate")
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	definitionID, trashedLeadID := mustCompanyListID(t), mustCompanyListID(t)
	if _, err := admin.Exec(ctx, `INSERT INTO customers.custom_field_definitions(
		workspace_id,id,entity_type,field_key,label,value_type,validation,options
	) VALUES($1,$2,'lead','legacy_tier','Legacy tier','text','{}'::jsonb,'[]'::jsonb)`,
		workspaceID.PG(), definitionID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO sales.leads(
		workspace_id,id,name,status,stage_id,custom_fields,deleted_at
	) SELECT $1,$2,'Trashed legacy field lead','new',id,'{"legacy_tier":"platinum"}'::jsonb,now()
	  FROM sales.lead_stages WHERE workspace_id=$1 AND category='new' AND is_default`,
		workspaceID.PG(), trashedLeadID.PG()); err != nil {
		t.Fatal(err)
	}
	backfill, err := appmigrations.Files.ReadFile("000038_sales_custom_field_backfill.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, string(backfill)); err != nil {
		t.Fatalf("rerun migration 38 backfill: %v", err)
	}
	var legacyValue string
	if err := admin.QueryRow(ctx, `SELECT value #>> '{}' FROM customers.custom_field_values
		WHERE workspace_id=$1 AND definition_id=$2 AND entity_id=$3`,
		workspaceID.PG(), definitionID.PG(), trashedLeadID.PG()).Scan(&legacyValue); err != nil {
		t.Fatalf("read backfilled trashed lead custom field: %v", err)
	}
	if legacyValue != "platinum" {
		t.Fatalf("backfilled trashed lead value=%q, want platinum", legacyValue)
	}
}
