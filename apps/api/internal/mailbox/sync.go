package mailbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

// SyncPlan is an opaque, short-lived snapshot prepared inside a tenant
// transaction. Network I/O consumes the snapshot only after that transaction
// has committed. Close must be called to clear decrypted credential bytes.
type SyncPlan struct {
	account         dbgen.MailboxAccount
	password        []byte
	highestByFolder map[string]uint32
}

type syncFolderSnapshot struct {
	folder RemoteFolder
	page   RemoteFolderPage
}

// SyncSnapshot contains bounded remote data and no credentials. It is applied
// in a new, short tenant transaction after all IMAP operations have completed.
type SyncSnapshot struct {
	folders []syncFolderSnapshot
}

func (plan *SyncPlan) Close() {
	if plan == nil {
		return
	}
	clear(plan.password)
	plan.password = nil
}

func (service *Service) PrepareSync(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, userID, accountID ids.UUID,
) (*SyncPlan, error) {
	account, password, err := service.secretAccount(ctx, workspace, workspaceID, userID, accountID)
	if err != nil {
		return nil, err
	}
	folders, err := workspace.Queries.ListMailboxFolders(ctx, dbgen.ListMailboxFoldersParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), AccountID: accountID.PG(),
	})
	if err != nil {
		clear(password)
		return nil, fmt.Errorf("list cached mailbox folders: %w", err)
	}
	started, err := workspace.Queries.MarkMailboxSyncStarted(ctx, dbgen.MarkMailboxSyncStartedParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(),
	})
	if err != nil {
		clear(password)
		return nil, fmt.Errorf("mark mailbox sync started: %w", err)
	}
	if started != 1 {
		clear(password)
		return nil, errx.ErrConflict
	}
	highest := make(map[string]uint32, len(folders))
	for _, folder := range folders {
		if folder.HighestUid >= 0 && folder.HighestUid <= 4294967295 {
			highest[folder.RemoteName] = uint32(folder.HighestUid)
		}
	}
	return &SyncPlan{account: account, password: password, highestByFolder: highest}, nil
}

func (service *Service) FetchSync(ctx context.Context, plan *SyncPlan) (*SyncSnapshot, error) {
	if plan == nil || len(plan.password) == 0 {
		return nil, errors.New("mail sync plan is invalid")
	}
	release, err := service.acquireConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	config := imapConfig(plan.account)
	remoteFolders, err := service.imap.ListFolders(ctx, config, string(plan.password))
	if err != nil {
		return nil, err
	}
	limit := min(len(remoteFolders), MaxFoldersPerSync)
	snapshot := &SyncSnapshot{folders: make([]syncFolderSnapshot, 0, limit)}
	for index := 0; index < limit; index++ {
		folder := remoteFolders[index]
		page, fetchErr := service.imap.FetchMessages(
			ctx, config, string(plan.password), folder.Name, plan.highestByFolder[folder.Name], MaxSyncMessages,
		)
		if fetchErr != nil {
			return nil, fetchErr
		}
		snapshot.folders = append(snapshot.folders, syncFolderSnapshot{folder: folder, page: page})
	}
	return snapshot, nil
}

func (service *Service) ApplySync(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, userID, accountID ids.UUID,
	snapshot *SyncSnapshot,
) error {
	if snapshot == nil || len(snapshot.folders) > MaxFoldersPerSync {
		return errors.New("mail sync snapshot is invalid")
	}
	for _, remote := range snapshot.folders {
		folderID, err := ids.NewV7()
		if err != nil {
			return err
		}
		uidValidity, uidNext := int64(remote.page.UIDValidity), int64(remote.page.UIDNext)
		folderRow, err := workspace.Queries.UpsertMailboxFolder(ctx, dbgen.UpsertMailboxFolderParams{
			WorkspaceID: workspaceID.PG(), UserID: userID.PG(), AccountID: accountID.PG(), ID: folderID.PG(),
			RemoteName: remote.folder.Name, DisplayName: remote.folder.DisplayName,
			Delimiter: optionalString(remote.folder.Delimiter), SpecialUse: optionalString(remote.folder.SpecialUse),
			UidValidity: &uidValidity, UidNext: &uidNext,
			TotalCount: boundedUint32(remote.page.Total), UnreadCount: boundedUint32(remote.page.Unseen),
		})
		if err != nil {
			return fmt.Errorf("upsert mailbox folder: %w", err)
		}
		if err := workspace.Queries.ResetMailboxFolderForUIDValidity(ctx, dbgen.ResetMailboxFolderForUIDValidityParams{
			WorkspaceID: workspaceID.PG(), UserID: userID.PG(), AccountID: accountID.PG(),
			FolderID: folderRow.ID, UidValidity: uidValidity,
		}); err != nil {
			return fmt.Errorf("reset mailbox UID validity: %w", err)
		}
		highestUID := folderRow.HighestUid
		for _, remoteMessage := range remote.page.Messages {
			messageID, idErr := ids.NewV7()
			if idErr != nil {
				return idErr
			}
			allRecipients := append([]Address{}, remoteMessage.Recipients.To...)
			allRecipients = append(allRecipients, remoteMessage.Recipients.Cc...)
			recipients, marshalErr := json.Marshal(allRecipients)
			if marshalErr != nil {
				return marshalErr
			}
			received := remoteMessage.ReceivedAt.UTC()
			if received.IsZero() {
				received = remoteMessage.SentAt.UTC()
			}
			if received.IsZero() {
				received = time.Now().UTC()
			}
			sentAt := pgtype.Timestamptz{}
			if !remoteMessage.SentAt.IsZero() {
				sentAt = pgtype.Timestamptz{Time: remoteMessage.SentAt.UTC(), Valid: true}
			}
			_, upsertErr := workspace.Queries.UpsertMailboxMessage(ctx, dbgen.UpsertMailboxMessageParams{
				WorkspaceID: workspaceID.PG(), UserID: userID.PG(), AccountID: accountID.PG(), FolderID: folderRow.ID,
				ID: messageID.PG(), UidValidity: uidValidity, RemoteUid: int64(remoteMessage.UID),
				InternetMessageID: optionalString(remoteMessage.InternetMessageID),
				Subject:           truncateRunes(remoteMessage.Subject, 2000),
				SenderName:        truncateRunes(remoteMessage.Sender.Name, 320),
				SenderAddress:     truncateRunes(remoteMessage.Sender.Address, 320),
				Recipients:        recipients, SentAt: sentAt,
				ReceivedAt: pgtype.Timestamptz{Time: received, Valid: true},
				Flags:      append([]string{}, remoteMessage.Flags...), SizeBytes: int64(remoteMessage.SizeBytes),
				Snippet: "", HasAttachments: remoteMessage.HasAttachments,
			})
			if upsertErr != nil {
				return fmt.Errorf("upsert mailbox message: %w", upsertErr)
			}
			if int64(remoteMessage.UID) > highestUID {
				highestUID = int64(remoteMessage.UID)
			}
		}
		if err := workspace.Queries.UpdateMailboxFolderHighWater(ctx, dbgen.UpdateMailboxFolderHighWaterParams{
			HighestUid: highestUID, UidValidity: &uidValidity, UidNext: &uidNext,
			WorkspaceID: workspaceID.PG(), UserID: userID.PG(), AccountID: accountID.PG(), ID: folderRow.ID,
		}); err != nil {
			return fmt.Errorf("update mailbox high water: %w", err)
		}
	}
	if err := workspace.Queries.MarkMailboxSyncFinished(ctx, dbgen.MarkMailboxSyncFinishedParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(),
	}); err != nil {
		return fmt.Errorf("finish mailbox sync: %w", err)
	}
	return nil
}

func (service *Service) MarkSyncFailed(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, userID, accountID ids.UUID,
	errorCode string,
) error {
	if errorCode == "" || len(errorCode) > 120 {
		errorCode = "mail_sync_failed"
	}
	return workspace.Queries.MarkMailboxSyncFailed(ctx, dbgen.MarkMailboxSyncFailedParams{
		ErrorCode: &errorCode, WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(),
	})
}

func SyncFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrEndpointRejected):
		return "mail_endpoint_rejected"
	case errors.Is(err, ErrEndpointUnavailable), errors.Is(err, ErrIMAPOperation):
		return "mail_endpoint_unavailable"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "mail_timeout"
	default:
		return "mail_sync_failed"
	}
}
