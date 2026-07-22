//go:build integration

package collaboration_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/collaboration"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestPrivateChatMembershipAndRecipientSSEOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "chat-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 1, "chat-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID := mustChatID(t)
	userA, userB, outsider := mustChatID(t), mustChatID(t), mustChatID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	_, err = admin.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Chat Alice','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($3,$4,$4,'Chat Bob','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($5,$6,$6,'Chat Outsider','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')`,
		userA.PG(), "chat-a-"+suffix+"@example.invalid",
		userB.PG(), "chat-b-"+suffix+"@example.invalid",
		outsider.PG(), "chat-c-"+suffix+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{userA.String(), userB.String(), outsider.String()})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES($1,'Chat Workspace',$2)`,
		workspaceID.PG(), "chat-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `
INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'owner'), ($1,$4,$5,'sales'), ($1,$6,$7,'viewer')`,
		workspaceID.PG(), mustChatID(t).PG(), userA.PG(), mustChatID(t).PG(), userB.PG(),
		mustChatID(t).PG(), outsider.PG()); err != nil {
		t.Fatal(err)
	}

	tenantService := tenancy.NewService(appPool)
	chatService := collaboration.NewService()
	alice := identity.Principal{UserID: userA}
	bob := identity.Principal{UserID: userB}
	third := identity.Principal{UserID: outsider}
	var conversation collaboration.Conversation
	err = tenantService.WithWorkspace(ctx, alice, workspaceID, "chat-create", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			conversation, createErr = chatService.Create(ctx, workspace, workspaceID, userA,
				collaboration.ConversationInput{MemberUserIDs: []ids.UUID{userB}})
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}
	conversationID := ids.MustParse(conversation.ID)
	var sent collaboration.Message
	err = tenantService.WithWorkspace(ctx, alice, workspaceID, "chat-send", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var sendErr error
			sent, sendErr = chatService.Send(ctx, workspace, workspaceID, conversationID, userA,
				collaboration.MessageInput{Kind: "text", Body: "private regression body"})
			return sendErr
		})
	if err != nil {
		t.Fatal(err)
	}

	err = tenantService.WithWorkspace(ctx, bob, workspaceID, "chat-read", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			page, listErr := chatService.ListMessages(ctx, workspace, workspaceID, conversationID, userB, "", 20)
			if listErr != nil {
				return listErr
			}
			if len(page.Items) != 1 || page.Items[0].ID != sent.ID || page.Items[0].Body != "private regression body" {
				t.Fatalf("unexpected recipient page: %#v", page)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	err = tenantService.WithWorkspace(ctx, third, workspaceID, "chat-outsider", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			_, listErr := chatService.ListMessages(ctx, workspace, workspaceID, conversationID, outsider, "", 20)
			if !errors.Is(listErr, errx.ErrNotFound) {
				t.Fatalf("outsider list error=%v, want not found", listErr)
			}
			var visibleEvents int
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM notifications.sse_events WHERE event_type='chat.message.created'`).Scan(&visibleEvents); queryErr != nil {
				return queryErr
			}
			if visibleEvents != 0 {
				t.Fatalf("outsider can read %d private SSE events", visibleEvents)
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
	var rawConversations, rawMembers, rawMessages int
	if err = outsiderTx.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM collaboration.conversations WHERE id=$1),
  (SELECT count(*) FROM collaboration.conversation_members WHERE conversation_id=$1),
  (SELECT count(*) FROM collaboration.messages WHERE conversation_id=$1)
`, conversationID.PG()).Scan(&rawConversations, &rawMembers, &rawMessages); err != nil {
		_ = outsiderTx.Rollback(ctx)
		t.Fatal(err)
	}
	if rawConversations != 0 || rawMembers != 0 || rawMessages != 0 {
		_ = outsiderTx.Rollback(ctx)
		t.Fatalf("RLS exposed private chat to outsider: conversations=%d members=%d messages=%d",
			rawConversations, rawMembers, rawMessages)
	}
	if _, err = outsiderTx.Exec(ctx, `
INSERT INTO collaboration.messages(workspace_id,id,conversation_id,sender_user_id,message_kind,body)
VALUES($1,$2,$3,$4,'text','blocked outsider message')
`, workspaceID.PG(), mustChatID(t).PG(), conversationID.PG(), outsider.PG()); err == nil {
		_ = outsiderTx.Rollback(ctx)
		t.Fatal("RLS allowed outsider to insert a private chat message")
	}
	_ = outsiderTx.Rollback(ctx)

	ownerTx, err := appPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ownerTx.Exec(ctx,
		`SELECT set_config('app.actor_id',$1,true), set_config('app.workspace_id',$2,true)`,
		userA.String(), workspaceID.String()); err != nil {
		_ = ownerTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = ownerTx.Exec(ctx, `
UPDATE collaboration.conversation_members
SET member_role='member'
WHERE workspace_id=$1 AND conversation_id=$2 AND user_id=$3
`, workspaceID.PG(), conversationID.PG(), userA.PG()); err == nil {
		_ = ownerTx.Rollback(ctx)
		t.Fatal("conversation owner could mutate the immutable security role directly")
	}
	_ = ownerTx.Rollback(ctx)

	var eventCount int
	var recipientsOnly, bodyAbsent bool
	if err := admin.QueryRow(ctx, `
SELECT count(*),
       bool_and(recipient_user_id IN ($2,$3)),
       bool_and(NOT (data ? 'body') AND NOT (data ? 'preview'))
FROM notifications.sse_events
WHERE workspace_id=$1 AND event_type='chat.message.created' AND data->>'messageId'=$4`,
		workspaceID.PG(), userA.PG(), userB.PG(), sent.ID).Scan(&eventCount, &recipientsOnly, &bodyAbsent); err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 || !recipientsOnly || !bodyAbsent {
		t.Fatalf("private SSE count=%d recipientsOnly=%t bodyAbsent=%t", eventCount, recipientsOnly, bodyAbsent)
	}
}

func mustChatID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
