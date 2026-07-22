//go:build integration

package mailbox_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/mailbox"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type mailboxIMAPStub struct {
	passwords []string
	listErr   error
}

func (stub *mailboxIMAPStub) ListFolders(
	_ context.Context, _ mailbox.ConnectionConfig, password string,
) ([]mailbox.RemoteFolder, error) {
	stub.passwords = append(stub.passwords, password)
	if stub.listErr != nil {
		return nil, stub.listErr
	}
	return []mailbox.RemoteFolder{{Name: "INBOX", DisplayName: "Inbox", SpecialUse: "inbox"}}, nil
}

func (stub *mailboxIMAPStub) FetchMessages(
	_ context.Context, _ mailbox.ConnectionConfig, password, folder string, _ uint32, limit int,
) (mailbox.RemoteFolderPage, error) {
	stub.passwords = append(stub.passwords, password)
	if folder != "INBOX" || limit > mailbox.MaxSyncMessages {
		return mailbox.RemoteFolderPage{}, errors.New("unexpected bounded sync request")
	}
	return mailbox.RemoteFolderPage{
		UIDValidity: 7, UIDNext: 43, Total: 1, Unseen: 1,
		Messages: []mailbox.RemoteMessage{{
			UID: 42, InternetMessageID: "remote-42@example.invalid", Subject: "Private mail",
			Sender:     mailbox.Address{Name: "Remote", Address: "remote@example.invalid"},
			Recipients: mailbox.RecipientSet{To: []mailbox.Address{{Address: "alice@example.invalid"}}},
			SentAt:     time.Now().Add(-time.Minute).UTC(), ReceivedAt: time.Now().UTC(), SizeBytes: 512,
		}},
	}, nil
}

func (stub *mailboxIMAPStub) ReadBody(
	_ context.Context, _ mailbox.ConnectionConfig, password, folder string, uid uint32,
) (string, error) {
	stub.passwords = append(stub.passwords, password)
	if folder != "INBOX" || uid != 42 {
		return "", errors.New("unexpected body request")
	}
	return "private body", nil
}

type mailboxSMTPStub struct {
	password  string
	messageID string
	input     mailbox.SendInput
	calls     int
	sendErr   error
}

func (stub *mailboxSMTPStub) Send(
	_ context.Context, _ mailbox.ConnectionConfig, password, _, messageID string, input mailbox.SendInput,
) error {
	stub.password = password
	stub.messageID = messageID
	stub.input = input
	stub.calls++
	return stub.sendErr
}

func TestMailboxUserOnlyRLSCredentialsSyncReadAndSend(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, adminURL, appPassword); err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(ctx, adminURL, 1, "mailbox-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 2, "mailbox-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceA, workspaceB := mailboxID(t), mailboxID(t)
	userA, adminB, otherTenant := mailboxID(t), mailboxID(t), mailboxID(t)
	suffix := strings.ReplaceAll(workspaceA.String(), "-", "")[:12]
	_, err = admin.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Mailbox Alice','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($3,$4,$4,'Workspace Owner','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($5,$6,$6,'Other Tenant','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')`,
		userA.PG(), "mail-a-"+suffix+"@example.invalid",
		adminB.PG(), "mail-b-"+suffix+"@example.invalid",
		otherTenant.PG(), "mail-c-"+suffix+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id = ANY($1::uuid[])`,
			[]string{workspaceA.String(), workspaceB.String()})
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{userA.String(), adminB.String(), otherTenant.String()})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES
 ($1,'Mailbox Workspace',$2),($3,'Other Workspace',$4)`,
		workspaceA.PG(), "mail-a-"+suffix, workspaceB.PG(), "mail-z-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'sales'),($1,$4,$5,'owner'),($6,$7,$8,'owner')`,
		workspaceA.PG(), mailboxID(t).PG(), userA.PG(), mailboxID(t).PG(), adminB.PG(),
		workspaceB.PG(), mailboxID(t).PG(), otherTenant.PG()); err != nil {
		t.Fatal(err)
	}

	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x51}, 32))
	cipher, err := identity.NewAESGCMKeyringFromBase64("mail-test-key", key)
	if err != nil {
		t.Fatal(err)
	}
	imapStub := &mailboxIMAPStub{}
	smtpStub := &mailboxSMTPStub{}
	mailService, err := mailbox.NewService(cipher, imapStub, smtpStub, mailbox.DefaultEndpointPolicy())
	if err != nil {
		t.Fatal(err)
	}
	tenantService := tenancy.NewService(appPool)
	alice := identity.Principal{UserID: userA}
	owner := identity.Principal{UserID: adminB}
	foreign := identity.Principal{UserID: otherTenant}

	var account mailbox.Account
	err = tenantService.WithWorkspace(ctx, alice, workspaceA, "mail-create", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			account, createErr = mailService.CreateAccount(ctx, workspace, workspaceA, userA, mailbox.AccountInput{
				DisplayName: "Corporate", Email: "alice@example.invalid", Username: "alice@example.invalid",
				IMAPHost: "imap.example.invalid", IMAPPort: 993, IMAPSecurity: "tls",
				SMTPHost: "smtp.example.invalid", SMTPPort: 587, SMTPSecurity: "starttls",
				Password: "mailbox-secret-value", SyncEnabled: true,
			})
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}

	var plaintextPresent bool
	if err := admin.QueryRow(ctx, `
SELECT position(convert_to('mailbox-secret-value','UTF8') in credential_ciphertext) > 0
FROM mailbox.accounts WHERE workspace_id=$1 AND id=$2`, workspaceA.PG(), account.ID.PG()).Scan(&plaintextPresent); err != nil {
		t.Fatal(err)
	}
	if plaintextPresent {
		t.Fatal("credential ciphertext contains plaintext secret")
	}

	err = tenantService.WithWorkspace(ctx, owner, workspaceA, "mail-owner-negative", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			accounts, listErr := mailService.ListAccounts(ctx, workspace, workspaceA, adminB)
			if listErr != nil || len(accounts) != 0 {
				t.Fatalf("workspace owner can see user mailbox: %#v, %v", accounts, listErr)
			}
			var visible int
			queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM mailbox.accounts WHERE id=$1`, account.ID.PG()).Scan(&visible)
			if queryErr != nil || visible != 0 {
				t.Fatalf("owner raw visibility=%d, err=%v", visible, queryErr)
			}
			result, updateErr := workspace.Tx.Exec(ctx, `UPDATE mailbox.accounts SET display_name='stolen' WHERE id=$1`, account.ID.PG())
			if updateErr != nil || result.RowsAffected() != 0 {
				t.Fatalf("owner updated another user's mailbox: rows=%d err=%v", result.RowsAffected(), updateErr)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	err = tenantService.WithWorkspace(ctx, owner, workspaceA, "mail-owner-spoof", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			_, insertErr := workspace.Tx.Exec(ctx, `
INSERT INTO mailbox.accounts(workspace_id,id,user_id,display_name,email,username,
 imap_host,imap_port,imap_security,smtp_host,smtp_port,smtp_security,
 credential_ciphertext,credential_nonce,key_id)
VALUES($1,$2,$3,'spoof','spoof@example.invalid','spoof','imap.example.invalid',993,'tls',
 'smtp.example.invalid',587,'starttls',$4,$5,'mail-test-key')`,
				workspaceA.PG(), mailboxID(t).PG(), userA.PG(), bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 12))
			if insertErr == nil {
				t.Fatal("workspace owner inserted a mailbox for another user")
			}
			return insertErr
		})
	if err == nil {
		t.Fatal("spoofed mailbox insert unexpectedly committed")
	}

	err = tenantService.WithWorkspace(ctx, foreign, workspaceB, "mail-tenant-negative", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var visible int
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM mailbox.accounts WHERE id=$1`, account.ID.PG()).Scan(&visible); queryErr != nil {
				return queryErr
			}
			if visible != 0 {
				t.Fatalf("other tenant can see %d mailbox accounts", visible)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	var syncPlan *mailbox.SyncPlan
	err = tenantService.WithWorkspace(ctx, alice, workspaceA, "mail-sync-prepare", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var prepareErr error
			syncPlan, prepareErr = mailService.PrepareSync(ctx, workspace, workspaceA, userA, account.ID)
			return prepareErr
		})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := mailService.FetchSync(ctx, syncPlan)
	syncPlan.Close()
	if err != nil {
		t.Fatal(err)
	}

	var syncedMessageID, outgoingID ids.UUID
	err = tenantService.WithWorkspace(ctx, alice, workspaceA, "mail-sync-apply-read-queue", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			if applyErr := mailService.ApplySync(ctx, workspace, workspaceA, userA, account.ID, snapshot); applyErr != nil {
				return applyErr
			}
			folders, listErr := mailService.ListFolders(ctx, workspace, workspaceA, userA, account.ID)
			if listErr != nil || len(folders) != 1 {
				t.Fatalf("folders=%#v err=%v", folders, listErr)
			}
			page, listErr := mailService.ListMessages(ctx, workspace, workspaceA, userA, folders[0].ID, "", 20)
			if listErr != nil || len(page.Items) != 1 || page.Items[0].Subject != "Private mail" {
				t.Fatalf("message page=%#v err=%v", page, listErr)
			}
			syncedMessageID = page.Items[0].ID
			body, bodyErr := mailService.ReadBody(ctx, workspace, workspaceA, userA, page.Items[0].ID)
			if bodyErr != nil || body != "private body" {
				t.Fatalf("body=%q err=%v", body, bodyErr)
			}
			var createErr error
			outgoingID, createErr = mailService.CreateOutgoing(ctx, workspace, workspaceA, userA, account.ID, mailbox.SendInput{
				Recipients: mailbox.RecipientSet{To: []mailbox.Address{{Address: "client@example.invalid"}}},
				Subject:    "Follow up", PlainText: "Hello",
			})
			if createErr != nil {
				return createErr
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	var deliveryJob worker.Job
	var jobID string
	if err := admin.QueryRow(ctx, `
SELECT id::text, kind, schema_version, payload, attempts, max_attempts
FROM platform.jobs
WHERE workspace_id=$1 AND kind=$2 AND idempotency_key=$3`,
		workspaceA.PG(), mailbox.DeliveryJobKind, outgoingID.String()).Scan(
		&jobID, &deliveryJob.Kind, &deliveryJob.SchemaVersion, &deliveryJob.Payload,
		&deliveryJob.Attempts, &deliveryJob.MaxAttempts,
	); err != nil {
		t.Fatalf("load durable delivery job: %v", err)
	}
	deliveryJob.WorkspaceID = workspaceA
	deliveryJob.ID, err = ids.Parse(jobID)
	if err != nil {
		t.Fatal(err)
	}
	deliveryJob.Attempts++
	if err := mailbox.NewDeliveryJobHandler(tenantService, mailService)(
		ctx, worker.Dependencies{AppPool: appPool}, deliveryJob,
	); err != nil {
		t.Fatalf("deliver durable mailbox job: %v", err)
	}
	var outgoingState, persistedMessageID string
	if err := admin.QueryRow(ctx, `
SELECT state,internet_message_id FROM mailbox.outgoing_messages
WHERE workspace_id=$1 AND id=$2`, workspaceA.PG(), outgoingID.PG()).Scan(
		&outgoingState, &persistedMessageID,
	); err != nil {
		t.Fatal(err)
	}
	if outgoingState != "sent" || persistedMessageID == "" {
		t.Fatalf("outgoing state=%q message_id=%q", outgoingState, persistedMessageID)
	}
	if smtpStub.calls != 1 || smtpStub.messageID != persistedMessageID {
		t.Fatalf("SMTP calls=%d message_id=%q, persisted=%q", smtpStub.calls, smtpStub.messageID, persistedMessageID)
	}
	if len(imapStub.passwords) < 3 || smtpStub.password != "mailbox-secret-value" {
		t.Fatalf("decrypted credentials did not reach bounded transports: imap=%d smtp=%q", len(imapStub.passwords), smtpStub.password)
	}
	if smtpStub.input.Subject != "Follow up" || smtpStub.input.PlainText != "Hello" {
		t.Fatalf("unexpected SMTP input %#v", smtpStub.input)
	}

	// A failure known to happen before SMTP submission is persisted as
	// retryable. Reprocessing the same durable job reuses the Message-ID and
	// advances the outgoing attempt count instead of creating a second record.
	var retryOutgoingID ids.UUID
	err = tenantService.WithWorkspace(ctx, alice, workspaceA, "mail-delivery-retry-queue", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			retryOutgoingID, createErr = mailService.CreateOutgoing(
				ctx, workspace, workspaceA, userA, account.ID, mailbox.SendInput{
					Recipients: mailbox.RecipientSet{To: []mailbox.Address{{Address: "retry@example.invalid"}}},
					Subject:    "Retry safely", PlainText: "Hello again",
				},
			)
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}
	var retryJob worker.Job
	if err := admin.QueryRow(ctx, `
SELECT id::text, kind, schema_version, payload, attempts, max_attempts
FROM platform.jobs
WHERE workspace_id=$1 AND kind=$2 AND idempotency_key=$3`,
		workspaceA.PG(), mailbox.DeliveryJobKind, retryOutgoingID.String()).Scan(
		&jobID, &retryJob.Kind, &retryJob.SchemaVersion, &retryJob.Payload,
		&retryJob.Attempts, &retryJob.MaxAttempts,
	); err != nil {
		t.Fatal(err)
	}
	retryJob.WorkspaceID = workspaceA
	retryJob.ID, err = ids.Parse(jobID)
	if err != nil {
		t.Fatal(err)
	}
	retryJob.Attempts++
	smtpStub.sendErr = mailbox.ErrEndpointUnavailable
	deliveryHandler := mailbox.NewDeliveryJobHandler(tenantService, mailService)
	if err := deliveryHandler(ctx, worker.Dependencies{AppPool: appPool}, retryJob); err == nil {
		t.Fatal("expected retryable delivery failure")
	}
	var retryState, retryCode string
	var retryAttempts int
	if err := admin.QueryRow(ctx, `
SELECT state,attempts,last_error_code FROM mailbox.outgoing_messages
WHERE workspace_id=$1 AND id=$2`, workspaceA.PG(), retryOutgoingID.PG()).Scan(
		&retryState, &retryAttempts, &retryCode,
	); err != nil {
		t.Fatal(err)
	}
	if retryState != "failed" || retryAttempts != 1 || retryCode != "mail_endpoint_unavailable" {
		t.Fatalf("retryable state=%q attempts=%d code=%q", retryState, retryAttempts, retryCode)
	}
	smtpStub.sendErr = nil
	retryJob.Attempts++
	if err := deliveryHandler(ctx, worker.Dependencies{AppPool: appPool}, retryJob); err != nil {
		t.Fatalf("retry delivery: %v", err)
	}
	if err := admin.QueryRow(ctx, `
SELECT state,attempts FROM mailbox.outgoing_messages
WHERE workspace_id=$1 AND id=$2`, workspaceA.PG(), retryOutgoingID.PG()).Scan(
		&retryState, &retryAttempts,
	); err != nil {
		t.Fatal(err)
	}
	if retryState != "sent" || retryAttempts != 2 {
		t.Fatalf("retried state=%q attempts=%d", retryState, retryAttempts)
	}

	// A failed remote sync is persisted in a separate transaction after the
	// IMAP call has returned. The account must never remain stuck in `syncing`.
	imapStub.listErr = mailbox.ErrEndpointUnavailable
	err = tenantService.WithWorkspace(ctx, alice, workspaceA, "mail-sync-failure-prepare", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var prepareErr error
			syncPlan, prepareErr = mailService.PrepareSync(ctx, workspace, workspaceA, userA, account.ID)
			return prepareErr
		})
	if err != nil {
		t.Fatal(err)
	}
	_, syncErr := mailService.FetchSync(ctx, syncPlan)
	syncPlan.Close()
	if !errors.Is(syncErr, mailbox.ErrEndpointUnavailable) {
		t.Fatalf("sync error=%v, want endpoint unavailable", syncErr)
	}
	err = tenantService.WithWorkspace(ctx, alice, workspaceA, "mail-sync-failure-state", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return mailService.MarkSyncFailed(
				ctx, workspace, workspaceA, userA, account.ID, mailbox.SyncFailureCode(syncErr),
			)
		})
	if err != nil {
		t.Fatal(err)
	}
	var syncState string
	var syncErrorCode *string
	if err := admin.QueryRow(ctx, `
SELECT sync_state,last_error_code FROM mailbox.accounts
WHERE workspace_id=$1 AND id=$2`, workspaceA.PG(), account.ID.PG()).Scan(&syncState, &syncErrorCode); err != nil {
		t.Fatal(err)
	}
	if syncState != "error" || syncErrorCode == nil || *syncErrorCode != "mail_endpoint_unavailable" {
		t.Fatalf("sync failure state=%q error=%v", syncState, syncErrorCode)
	}

	err = tenantService.WithWorkspace(ctx, owner, workspaceA, "mail-child-rls-negative", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var accounts, folders, messages, bodies, outgoing int
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM mailbox.accounts),
 (SELECT count(*) FROM mailbox.folders),
 (SELECT count(*) FROM mailbox.messages),
 (SELECT count(*) FROM mailbox.message_bodies),
 (SELECT count(*) FROM mailbox.outgoing_messages)`).Scan(
				&accounts, &folders, &messages, &bodies, &outgoing,
			); queryErr != nil {
				return queryErr
			}
			if accounts+folders+messages+bodies+outgoing != 0 {
				t.Fatalf("owner sees private mailbox children: %d/%d/%d/%d/%d", accounts, folders, messages, bodies, outgoing)
			}
			updated, updateErr := workspace.Tx.Exec(ctx,
				`UPDATE mailbox.messages SET subject='stolen' WHERE id=$1`, syncedMessageID.PG())
			if updateErr != nil || updated.RowsAffected() != 0 {
				t.Fatalf("owner updated private message: rows=%d err=%v", updated.RowsAffected(), updateErr)
			}
			deleted, deleteErr := workspace.Tx.Exec(ctx,
				`DELETE FROM mailbox.outgoing_messages WHERE id=$1`, outgoingID.PG())
			if deleteErr != nil || deleted.RowsAffected() != 0 {
				t.Fatalf("owner deleted private outgoing message: rows=%d err=%v", deleted.RowsAffected(), deleteErr)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	// A raw app-role transaction without actor context must fail closed.
	tx, err := appPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM mailbox.accounts`).Scan(&count); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("mailbox RLS without context exposed %d rows", count)
	}
}

func mailboxID(t *testing.T) ids.UUID {
	t.Helper()
	value, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
