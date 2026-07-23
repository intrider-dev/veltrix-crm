//go:build integration

package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/files"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestAttachmentRoutesEnforceDealPermissionsAndChatOwnership(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	adminURL, appURL := isolatedContactExportDatabase(t, ctx)
	admin, err := database.Open(ctx, adminURL, 2, "attachment-permissions-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 4, "attachment-permissions-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID, userA, userB := mustContactExportID(t), mustContactExportID(t), mustContactExportID(t)
	membershipA, membershipB, roleID := mustContactExportID(t), mustContactExportID(t), mustContactExportID(t)
	dealID, dealAttachmentID := mustContactExportID(t), mustContactExportID(t)
	conversationID, messageID, provisionalMessageID, chatAttachmentID := mustContactExportID(t), mustContactExportID(t), mustContactExportID(t), mustContactExportID(t)
	password := "Integration-attachments-123!"
	passwordHash, err := identity.NewPasswordHasher(1).Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	suffix := compactContactExportID(workspaceID)[:16]
	setup, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = setup.Rollback(ctx) }()
	mustExec := func(statement string, args ...any) {
		t.Helper()
		if _, execErr := setup.Exec(ctx, statement, args...); execErr != nil {
			t.Fatal(execErr)
		}
	}
	mustExec(`
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Attachment reader',$3),($4,$5,$5,'Attachment sender',$3)`,
		userA.PG(), "attachment-a-"+suffix+"@example.invalid", passwordHash,
		userB.PG(), "attachment-b-"+suffix+"@example.invalid")
	mustExec(`INSERT INTO tenancy.workspaces(id,name,slug) VALUES($1,'Attachment permissions',$2)`,
		workspaceID.PG(), "attachments-"+suffix)
	mustExec(`
INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
	($1,$2,$3,'owner'),($1,$4,$5,'viewer')`,
		workspaceID.PG(), membershipA.PG(), userA.PG(), membershipB.PG(), userB.PG())
	mustExec(`
INSERT INTO tenancy.workspace_roles(workspace_id,id,role_key,name,base_role,is_system)
	VALUES($1,$2,'record-reader','Record reader','viewer',false)`, workspaceID.PG(), roleID.PG())
	mustExec(`INSERT INTO tenancy.role_permissions(workspace_id,role_id,permission) VALUES($1,$2,'records.read')`,
		workspaceID.PG(), roleID.PG())
	mustExec(`UPDATE tenancy.memberships SET role_id=$2,role='viewer' WHERE workspace_id=$1 AND user_id=$3`,
		workspaceID.PG(), roleID.PG(), userA.PG())
	mustExec(`
INSERT INTO sales.deals(workspace_id,id,pipeline_id,stage_id,name,amount_minor,currency)
	SELECT $1,$2,pipeline.id,stage.id,'Protected deal',100,'USD'
 FROM sales.pipelines pipeline JOIN sales.pipeline_stages stage
   ON stage.workspace_id=pipeline.workspace_id AND stage.pipeline_id=pipeline.id
	WHERE pipeline.workspace_id=$1 AND pipeline.is_default ORDER BY stage.position LIMIT 1`,
		workspaceID.PG(), dealID.PG())
	mustExec(`
INSERT INTO collaboration.conversations(workspace_id,id,conversation_type,title,created_by)
	VALUES($1,$2,'group','Private fixture',$3)`, workspaceID.PG(), conversationID.PG(), userB.PG())
	mustExec(`
INSERT INTO collaboration.conversation_members(workspace_id,conversation_id,user_id,member_role)
	VALUES($1,$2,$3,'member'),($1,$2,$4,'owner')`,
		workspaceID.PG(), conversationID.PG(), userA.PG(), userB.PG())
	mustExec(`
INSERT INTO collaboration.messages(workspace_id,id,conversation_id,sender_user_id,message_kind,body)
	VALUES($1,$2,$3,$4,'file','sender attachment'),
	      ($1,$5,$3,$4,'file','provisional retry')`,
		workspaceID.PG(), messageID.PG(), conversationID.PG(), userB.PG(), provisionalMessageID.PG())
	mustExec(`
INSERT INTO files.attachments(workspace_id,id,entity_type,entity_id,storage_backend,storage_key,
 display_name,media_type,size_bytes,checksum_sha256,scan_state,uploaded_by) VALUES
	($1,$2,'deal',$3,'local',$4,'deal.txt','text/plain',1,decode(repeat('00',32),'hex'),'clean',$5),
	($1,$6,'chat_message',$7,'local',$8,'chat.txt','text/plain',1,decode(repeat('11',32),'hex'),'clean',$5)`,
		workspaceID.PG(), dealAttachmentID.PG(), dealID.PG(), workspaceID.String()+"/deal/blob", userB.PG(),
		chatAttachmentID.PG(), messageID.PG(), workspaceID.String()+"/chat/blob")
	if err := setup.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	application, err := New(config.Config{
		Environment: "development", PublicURL: "http://example.test", DefaultLocale: "en",
		SupportedLocales: []string{"en", "ru"}, SessionTTL: time.Hour,
		PasswordResetTTL: time.Hour, MFAChallengeTTL: 5 * time.Minute, MFASetupTTL: 10 * time.Minute,
		UploadDir: t.TempDir(), MaxUploadBytes: 1 << 20, StorageBackend: "local",
		AIProvider: "disabled", CallsProvider: "disabled",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), appPool, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	session, err := application.identity.Login(ctx, "attachment-a-"+suffix+"@example.invalid", password,
		"attachment-permissions", nil)
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"/api/v1/workspaces/" + workspaceID.String() + "/attachments?entityType=deal&entityId=" + dealID.String(),
		"/api/v1/workspaces/" + workspaceID.String() + "/attachments?entityType=%20deal%20&entityId=" + dealID.String(),
		"/api/v1/workspaces/" + workspaceID.String() + "/attachments/" + dealAttachmentID.String(),
	}
	for _, path := range paths {
		response := serveAttachmentRequest(application, http.MethodGet, path, session.Token, session.CSRFToken)
		if response.Code != http.StatusForbidden {
			t.Fatalf("GET %s status=%d body=%s, want 403", path, response.Code, response.Body.String())
		}
	}
	for _, attachmentID := range []string{dealAttachmentID.String(), chatAttachmentID.String()} {
		path := "/api/v1/workspaces/" + workspaceID.String() + "/attachments/" + attachmentID
		response := serveAttachmentRequest(application, http.MethodDelete, path, session.Token, session.CSRFToken)
		if response.Code != http.StatusForbidden {
			t.Fatalf("DELETE %s status=%d body=%s, want 403", path, response.Code, response.Body.String())
		}
	}
	var remaining int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM files.attachments
		WHERE workspace_id=$1 AND id IN ($2,$3) AND deleted_at IS NULL`,
		workspaceID.PG(), dealAttachmentID.PG(), chatAttachmentID.PG()).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 2 {
		t.Fatalf("unauthorized deletes left %d attachments, want 2", remaining)
	}

	var replay files.UploadResult
	err = application.tenancy.WithWorkspaceAny(ctx, identity.Principal{UserID: userB}, workspaceID,
		"attachment-retry", chatAccessPermissions(), func(workspace *tenancy.WorkspaceTx) error {
			var uploadErr error
			replay, uploadErr = application.attachments.Upload(ctx, workspace, events.Metadata{
				WorkspaceID: workspaceID, ActorID: userB, RequestID: "attachment-retry",
			}, files.UploadInput{
				EntityType: "chat_message", EntityID: messageID, DisplayName: "retry.txt",
				DeclaredMediaType: "text/plain", Contents: strings.NewReader("retry"),
			})
			return uploadErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if replay.Attachment.ID != chatAttachmentID.PG() {
		t.Fatalf("ambiguous upload retry returned %v, want existing %v", replay.Attachment.ID, chatAttachmentID.PG())
	}

	senderSession, err := application.identity.Login(ctx, "attachment-b-"+suffix+"@example.invalid", password,
		"attachment-retry", nil)
	if err != nil {
		t.Fatal(err)
	}
	deletePath := "/api/v1/workspaces/" + workspaceID.String() + "/chat/messages/" + provisionalMessageID.String()
	for attempt := 1; attempt <= 2; attempt++ {
		response := serveAttachmentRequest(application, http.MethodDelete, deletePath, senderSession.Token, senderSession.CSRFToken)
		if response.Code != http.StatusNoContent {
			t.Fatalf("provisional DELETE attempt %d status=%d body=%s, want 204", attempt, response.Code, response.Body.String())
		}
	}
}

func serveAttachmentRequest(
	application *Application, method, path, sessionToken, csrfToken string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.AddCookie(&http.Cookie{Name: application.sessionCookieName(), Value: sessionToken})
	if method != http.MethodGet {
		request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
		request.Header.Set("X-CSRF-Token", csrfToken)
	}
	response := httptest.NewRecorder()
	application.Handler().ServeHTTP(response, request)
	return response
}
