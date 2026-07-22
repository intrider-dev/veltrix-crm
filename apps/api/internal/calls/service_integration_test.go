//go:build integration

package calls_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/calls"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/collaboration"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestCallMembershipAndRecipientEventsOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "call-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 1, "call-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID := mustCallID(t)
	caller, recipient, outsider := mustCallID(t), mustCallID(t), mustCallID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	_, err = admin.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Call Alice','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($3,$4,$4,'Call Bob','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($5,$6,$6,'Call Outsider','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')`,
		caller.PG(), "call-a-"+suffix+"@example.invalid",
		recipient.PG(), "call-b-"+suffix+"@example.invalid",
		outsider.PG(), "call-c-"+suffix+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{caller.String(), recipient.String(), outsider.String()})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES($1,'Call Workspace',$2)`,
		workspaceID.PG(), "call-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `
INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'owner'), ($1,$4,$5,'sales'), ($1,$6,$7,'viewer')`,
		workspaceID.PG(), mustCallID(t).PG(), caller.PG(), mustCallID(t).PG(), recipient.PG(),
		mustCallID(t).PG(), outsider.PG()); err != nil {
		t.Fatal(err)
	}

	tenantService := tenancy.NewService(appPool)
	chatService := collaboration.NewService()
	provider, err := calls.NewLiveKitProvider("wss://calls.example.invalid", "test-key", "test-secret", 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	callService := calls.NewService(provider)
	callerPrincipal := identity.Principal{UserID: caller}
	var conversation collaboration.Conversation
	err = tenantService.WithWorkspace(ctx, callerPrincipal, workspaceID, "call-chat-create", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			conversation, createErr = chatService.Create(ctx, workspace, workspaceID, caller,
				collaboration.ConversationInput{MemberUserIDs: []ids.UUID{recipient}})
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}
	conversationID := ids.MustParse(conversation.ID)
	var created calls.Call
	err = tenantService.WithWorkspace(ctx, callerPrincipal, workspaceID, "call-create", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			created, createErr = callService.Create(ctx, workspace, events.Metadata{
				WorkspaceID: workspaceID, ActorID: caller, RequestID: "call-create",
			}, conversationID, "video")
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}

	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: recipient}, workspaceID,
		"call-join", tenancy.PermissionRecordsRead, func(workspace *tenancy.WorkspaceTx) error {
			visible, grant, joinErr := callService.Join(ctx, workspace, workspaceID, created.ID, recipient, "Call Bob")
			if joinErr != nil {
				return joinErr
			}
			if visible.ParticipantState != "joined" || grant.Token == "" || grant.URL != "wss://calls.example.invalid" {
				t.Fatalf("unexpected joined call=%+v grant=%+v", visible, grant)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: outsider}, workspaceID,
		"call-outsider", tenancy.PermissionRecordsRead, func(workspace *tenancy.WorkspaceTx) error {
			_, getErr := callService.Get(ctx, workspace, workspaceID, created.ID, outsider)
			if !errors.Is(getErr, errx.ErrNotFound) {
				t.Fatalf("outsider call read error=%v, want not found", getErr)
			}
			var visibleEvents int
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM notifications.sse_events WHERE event_type LIKE 'call.%'`).Scan(&visibleEvents); queryErr != nil {
				return queryErr
			}
			if visibleEvents != 0 {
				t.Fatalf("outsider can read %d call events", visibleEvents)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	outsiderTx, err := appPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = outsiderTx.Exec(ctx,
		`SELECT set_config('app.actor_id',$1,true), set_config('app.workspace_id',$2,true)`,
		outsider.String(), workspaceID.String()); err != nil {
		_ = outsiderTx.Rollback(ctx)
		t.Fatal(err)
	}
	var rawCalls, rawParticipants int
	if err = outsiderTx.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM collaboration.calls WHERE id=$1),
  (SELECT count(*) FROM collaboration.call_participants WHERE call_id=$1)
`, created.ID.PG()).Scan(&rawCalls, &rawParticipants); err != nil {
		_ = outsiderTx.Rollback(ctx)
		t.Fatal(err)
	}
	if rawCalls != 0 || rawParticipants != 0 {
		_ = outsiderTx.Rollback(ctx)
		t.Fatalf("RLS exposed private call to outsider: calls=%d participants=%d", rawCalls, rawParticipants)
	}
	_ = outsiderTx.Rollback(ctx)

	callerTx, err := appPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = callerTx.Exec(ctx,
		`SELECT set_config('app.actor_id',$1,true), set_config('app.workspace_id',$2,true)`,
		caller.String(), workspaceID.String()); err != nil {
		_ = callerTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = callerTx.Exec(ctx, `
UPDATE collaboration.calls SET created_by=$1 WHERE workspace_id=$2 AND id=$3
`, recipient.PG(), workspaceID.PG(), created.ID.PG()); err == nil {
		_ = callerTx.Rollback(ctx)
		t.Fatal("call creator could mutate immutable call ownership directly")
	}
	_ = callerTx.Rollback(ctx)

	participantTx, err := appPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = participantTx.Exec(ctx,
		`SELECT set_config('app.actor_id',$1,true), set_config('app.workspace_id',$2,true)`,
		caller.String(), workspaceID.String()); err != nil {
		_ = participantTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = participantTx.Exec(ctx, `
UPDATE collaboration.call_participants SET user_id=$1
WHERE workspace_id=$2 AND call_id=$3 AND user_id=$4
`, outsider.PG(), workspaceID.PG(), created.ID.PG(), recipient.PG()); err == nil {
		_ = participantTx.Rollback(ctx)
		t.Fatal("call creator could mutate immutable participant identity directly")
	}
	_ = participantTx.Rollback(ctx)

	var count int
	var recipientsOnly, tokenAbsent bool
	if err := admin.QueryRow(ctx, `
SELECT count(*), bool_and(recipient_user_id IN ($2,$3)),
       bool_and(NOT (data ? 'token') AND NOT (data ? 'url'))
FROM notifications.sse_events
WHERE workspace_id=$1 AND event_type='call.invited' AND data->>'callId'=$4`,
		workspaceID.PG(), caller.PG(), recipient.PG(), created.ID.String()).Scan(&count, &recipientsOnly, &tokenAbsent); err != nil {
		t.Fatal(err)
	}
	if count != 2 || !recipientsOnly || !tokenAbsent {
		t.Fatalf("call events=%d recipientsOnly=%t tokenAbsent=%t", count, recipientsOnly, tokenAbsent)
	}
}

func mustCallID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
