package mailbox

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const credentialPurpose = "mail.account.credentials.v1"

type Service struct {
	cipher      identity.SecretCipher
	imap        IMAPTransport
	smtp        SMTPTransport
	policy      EndpointPolicy
	connections chan struct{}
}

type credentialEnvelope struct {
	SchemaVersion int    `json:"schemaVersion"`
	Password      string `json:"password"`
}

func NewService(
	cipher identity.SecretCipher,
	imapTransport IMAPTransport,
	smtpTransport SMTPTransport,
	policy EndpointPolicy,
) (*Service, error) {
	if cipher == nil || imapTransport == nil || smtpTransport == nil {
		return nil, errors.New("mailbox cipher, IMAP transport, and SMTP transport are required")
	}
	if policy.IMAPPorts == nil || policy.SMTPPorts == nil {
		policy = DefaultEndpointPolicy()
	}
	return &Service{
		cipher: cipher, imap: imapTransport, smtp: smtpTransport, policy: policy,
		connections: make(chan struct{}, MaxConcurrentConnections),
	}, nil
}

func (service *Service) CreateAccount(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, userID ids.UUID,
	input AccountInput,
) (Account, error) {
	validated, err := service.validateAccount(input, true)
	if err != nil {
		return Account{}, err
	}
	existing, err := workspace.Queries.ListMailboxAccounts(ctx, dbgen.ListMailboxAccountsParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), PageLimit: MaxAccounts + 1,
	})
	if err != nil {
		return Account{}, fmt.Errorf("list mailbox accounts: %w", err)
	}
	if len(existing) >= MaxAccounts {
		return Account{}, validation("/account", "mail.account.limit")
	}
	accountID, err := ids.NewV7()
	if err != nil {
		return Account{}, err
	}
	encrypted, err := service.encryptCredential(ctx, workspaceID, accountID, userID, validated.Password)
	if err != nil {
		return Account{}, err
	}
	row, err := workspace.Queries.CreateMailboxAccount(ctx, dbgen.CreateMailboxAccountParams{
		WorkspaceID: workspaceID.PG(), ID: accountID.PG(), UserID: userID.PG(),
		DisplayName: validated.DisplayName, Email: validated.Email, Username: validated.Username,
		ImapHost: validated.IMAPHost, ImapPort: int32(validated.IMAPPort), ImapSecurity: validated.IMAPSecurity,
		SmtpHost: validated.SMTPHost, SmtpPort: int32(validated.SMTPPort), SmtpSecurity: validated.SMTPSecurity,
		CredentialCiphertext: encrypted.Ciphertext, CredentialNonce: encrypted.Nonce,
		KeyID: encrypted.KeyID, SyncEnabled: validated.SyncEnabled,
	})
	if err != nil {
		return Account{}, fmt.Errorf("create mailbox account: %w", err)
	}
	return mapAccount(row.ID, row.DisplayName, row.Email, row.Username, row.ImapHost, row.ImapPort,
		row.ImapSecurity, row.SmtpHost, row.SmtpPort, row.SmtpSecurity, row.SyncEnabled,
		row.SyncState, row.LastSyncAt, row.LastErrorCode, row.Version), nil
}

func (service *Service) ListAccounts(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, userID ids.UUID,
) ([]Account, error) {
	rows, err := workspace.Queries.ListMailboxAccounts(ctx, dbgen.ListMailboxAccountsParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), PageLimit: MaxAccounts,
	})
	if err != nil {
		return nil, fmt.Errorf("list mailbox accounts: %w", err)
	}
	result := make([]Account, len(rows))
	for index, row := range rows {
		result[index] = mapAccount(row.ID, row.DisplayName, row.Email, row.Username, row.ImapHost,
			row.ImapPort, row.ImapSecurity, row.SmtpHost, row.SmtpPort, row.SmtpSecurity,
			row.SyncEnabled, row.SyncState, row.LastSyncAt, row.LastErrorCode, row.Version)
	}
	return result, nil
}

func (service *Service) GetAccount(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, userID, accountID ids.UUID,
) (Account, error) {
	row, err := workspace.Queries.GetMailboxAccount(ctx, dbgen.GetMailboxAccountParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, errx.ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get mailbox account: %w", err)
	}
	return mapAccount(row.ID, row.DisplayName, row.Email, row.Username, row.ImapHost,
		row.ImapPort, row.ImapSecurity, row.SmtpHost, row.SmtpPort, row.SmtpSecurity,
		row.SyncEnabled, row.SyncState, row.LastSyncAt, row.LastErrorCode, row.Version), nil
}

func (service *Service) UpdateAccount(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, userID, accountID ids.UUID,
	expectedVersion int64,
	input AccountInput,
) (Account, error) {
	validated, err := service.validateAccount(input, false)
	if err != nil {
		return Account{}, err
	}
	row, err := workspace.Queries.UpdateMailboxAccount(ctx, dbgen.UpdateMailboxAccountParams{
		DisplayName: validated.DisplayName, Email: validated.Email, Username: validated.Username,
		ImapHost: validated.IMAPHost, ImapPort: int32(validated.IMAPPort), ImapSecurity: validated.IMAPSecurity,
		SmtpHost: validated.SMTPHost, SmtpPort: int32(validated.SMTPPort), SmtpSecurity: validated.SMTPSecurity,
		SyncEnabled: validated.SyncEnabled, WorkspaceID: workspaceID.PG(), UserID: userID.PG(),
		ID: accountID.PG(), ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, errx.ErrConflict
	}
	if err != nil {
		return Account{}, fmt.Errorf("update mailbox account: %w", err)
	}
	if validated.Password != "" {
		encrypted, encryptErr := service.encryptCredential(ctx, workspaceID, accountID, userID, validated.Password)
		if encryptErr != nil {
			return Account{}, encryptErr
		}
		updated, replaceErr := workspace.Queries.ReplaceMailboxCredential(ctx, dbgen.ReplaceMailboxCredentialParams{
			CredentialCiphertext: encrypted.Ciphertext, CredentialNonce: encrypted.Nonce, KeyID: encrypted.KeyID,
			WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(), ExpectedVersion: row.Version,
		})
		if replaceErr != nil {
			return Account{}, fmt.Errorf("replace mailbox credential: %w", replaceErr)
		}
		if updated != 1 {
			return Account{}, errx.ErrConflict
		}
		row.Version++
	}
	return mapAccount(row.ID, row.DisplayName, row.Email, row.Username, row.ImapHost, row.ImapPort,
		row.ImapSecurity, row.SmtpHost, row.SmtpPort, row.SmtpSecurity, row.SyncEnabled,
		row.SyncState, row.LastSyncAt, row.LastErrorCode, row.Version), nil
}

func (service *Service) DeleteAccount(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, userID, accountID ids.UUID, expectedVersion int64,
) error {
	rows, err := workspace.Queries.DeleteMailboxAccount(ctx, dbgen.DeleteMailboxAccountParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(), ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return fmt.Errorf("delete mailbox account: %w", err)
	}
	if rows != 1 {
		return errx.ErrConflict
	}
	return nil
}

func (service *Service) ListFolders(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, userID, accountID ids.UUID,
) ([]Folder, error) {
	rows, err := workspace.Queries.ListMailboxFolders(ctx, dbgen.ListMailboxFoldersParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), AccountID: accountID.PG(),
	})
	if err != nil {
		return nil, fmt.Errorf("list mailbox folders: %w", err)
	}
	result := make([]Folder, len(rows))
	for index, row := range rows {
		result[index] = mapFolder(row)
	}
	return result, nil
}

func (service *Service) ListMessages(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, userID, folderID ids.UUID,
	cursor string, limit int,
) (MessagePage, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > MaxMessagePage {
		limit = MaxMessagePage
	}
	cursorTime, cursorID, err := decodeCursor(cursor)
	if err != nil {
		return MessagePage{}, validation("/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListMailboxMessages(ctx, dbgen.ListMailboxMessagesParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), FolderID: folderID.PG(),
		CursorTime: cursorTime, CursorID: cursorID, PageLimit: int32(limit + 1),
	})
	if err != nil {
		return MessagePage{}, fmt.Errorf("list mailbox messages: %w", err)
	}
	page := MessagePage{Items: make([]Message, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			last := rows[index-1]
			page.NextCursor = encodeCursor(last.ReceivedAt.Time.UTC(), mustUUID(last.ID))
			break
		}
		page.Items = append(page.Items, mapMessage(row))
	}
	return page, nil
}

func (service *Service) ReadBody(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, userID, messageID ids.UUID,
) (string, error) {
	if cached, err := workspace.Queries.GetMailboxMessageBody(ctx, dbgen.GetMailboxMessageBodyParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), MessageID: messageID.PG(),
	}); err == nil {
		return cached.PlainText, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("get mailbox message body: %w", err)
	}
	messageRow, err := workspace.Queries.GetMailboxMessage(ctx, dbgen.GetMailboxMessageParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: messageID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errx.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get mailbox message: %w", err)
	}
	if messageRow.SizeBytes > MaxMessageBytes {
		return "", ErrMessageTooLarge
	}
	release, err := service.acquireConnection(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	folder, err := workspace.Queries.GetMailboxFolder(ctx, dbgen.GetMailboxFolderParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: messageRow.FolderID,
	})
	if err != nil {
		return "", fmt.Errorf("get mailbox folder: %w", err)
	}
	account, password, err := service.secretAccount(ctx, workspace, workspaceID, userID, mustUUID(messageRow.AccountID))
	if err != nil {
		return "", err
	}
	defer clear(password)
	body, err := service.imap.ReadBody(ctx, imapConfig(account), string(password), folder.RemoteName, uint32(messageRow.RemoteUid))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > MaxBodyBytes {
		return "", ErrMessageTooLarge
	}
	if err := workspace.Queries.StoreMailboxMessageBody(ctx, dbgen.StoreMailboxMessageBodyParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), MessageID: messageID.PG(),
		AccountID: messageRow.AccountID, PlainText: body,
	}); err != nil {
		return "", fmt.Errorf("store mailbox message body: %w", err)
	}
	return body, nil
}

func (service *Service) CreateOutgoing(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, userID, accountID ids.UUID, input SendInput,
) (ids.UUID, error) {
	if _, err := validatedRecipients(input.Recipients); err != nil {
		return ids.UUID{}, validation("/recipients", "mail.recipients.invalid")
	}
	if strings.ContainsAny(input.Subject, "\r\n") || len([]rune(input.Subject)) > 2000 || int64(len(input.PlainText)) > MaxBodyBytes {
		return ids.UUID{}, validation("/message", "mail.message.invalid")
	}
	account, err := workspace.Queries.GetMailboxAccount(ctx, dbgen.GetMailboxAccountParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ids.UUID{}, errx.ErrNotFound
	}
	if err != nil {
		return ids.UUID{}, fmt.Errorf("get mailbox account: %w", err)
	}
	id, err := ids.NewV7()
	if err != nil {
		return ids.UUID{}, err
	}
	domain := "mail.invalid"
	if address, parseErr := exactAddress(account.Email); parseErr == nil {
		if _, candidate, found := strings.Cut(address.Address, "@"); found && candidate != "" {
			domain = candidate
		}
	}
	internetID := id.String() + "@" + domain
	recipients, err := json.Marshal(input.Recipients)
	if err != nil {
		return ids.UUID{}, err
	}
	if _, err := workspace.Queries.CreateMailboxOutgoing(ctx, dbgen.CreateMailboxOutgoingParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), AccountID: accountID.PG(), ID: id.PG(),
		InternetMessageID: internetID, Recipients: recipients, Subject: input.Subject,
		PlainText: normalizePlainText(input.PlainText),
	}); err != nil {
		return ids.UUID{}, fmt.Errorf("create outgoing mailbox message: %w", err)
	}
	jobID, err := ids.NewV7()
	if err != nil {
		return ids.UUID{}, err
	}
	if err := workspace.Queries.EnqueueMailboxOutgoingDelivery(ctx, dbgen.EnqueueMailboxOutgoingDeliveryParams{
		WorkspaceID: workspaceID.PG(), JobID: jobID.PG(), OutgoingID: id.String(), UserID: userID.String(),
	}); err != nil {
		return ids.UUID{}, fmt.Errorf("enqueue outgoing mailbox delivery: %w", err)
	}
	return id, nil
}

func (service *Service) secretAccount(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, userID, accountID ids.UUID,
) (dbgen.MailboxAccount, []byte, error) {
	account, err := workspace.Queries.GetMailboxAccountSecret(ctx, dbgen.GetMailboxAccountSecretParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), ID: accountID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.MailboxAccount{}, nil, errx.ErrNotFound
	}
	if err != nil {
		return dbgen.MailboxAccount{}, nil, fmt.Errorf("get mailbox secret: %w", err)
	}
	plaintext, err := service.cipher.Decrypt(ctx, credentialPurpose, credentialSubject(workspaceID, accountID, userID), identity.SecretEnvelope{
		Ciphertext: account.CredentialCiphertext, Nonce: account.CredentialNonce, KeyID: account.KeyID,
	})
	if err != nil {
		return dbgen.MailboxAccount{}, nil, errors.New("mail credential decryption failed")
	}
	var credential credentialEnvelope
	if err := json.Unmarshal(plaintext, &credential); err != nil {
		clear(plaintext)
		return dbgen.MailboxAccount{}, nil, errors.New("mail credential is invalid")
	}
	clear(plaintext)
	if credential.SchemaVersion != 1 || credential.Password == "" || len(credential.Password) > 4096 {
		return dbgen.MailboxAccount{}, nil, errors.New("mail credential is invalid")
	}
	password := []byte(credential.Password)
	credential.Password = ""
	return account, password, nil
}

func (service *Service) encryptCredential(
	ctx context.Context, workspaceID, accountID, userID ids.UUID, password string,
) (identity.SecretEnvelope, error) {
	if password == "" || len(password) > 4096 {
		return identity.SecretEnvelope{}, validation("/password", "validation.length")
	}
	payload, err := json.Marshal(credentialEnvelope{SchemaVersion: 1, Password: password})
	if err != nil {
		return identity.SecretEnvelope{}, err
	}
	defer clear(payload)
	envelope, err := service.cipher.Encrypt(ctx, credentialPurpose, credentialSubject(workspaceID, accountID, userID), payload)
	if err != nil {
		return identity.SecretEnvelope{}, errors.New("mail credential encryption failed")
	}
	return envelope, nil
}

func (service *Service) validateAccount(input AccountInput, passwordRequired bool) (AccountInput, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Username = strings.TrimSpace(input.Username)
	input.IMAPHost = strings.ToLower(strings.TrimSpace(input.IMAPHost))
	input.SMTPHost = strings.ToLower(strings.TrimSpace(input.SMTPHost))
	address, err := exactAddress(input.Email)
	if err != nil || address.Name != "" {
		return AccountInput{}, validation("/email", "validation.email.invalid")
	}
	input.Email = strings.ToLower(address.Address)
	if len([]rune(input.DisplayName)) < 1 || len([]rune(input.DisplayName)) > 160 ||
		len(input.Username) < 1 || len(input.Username) > 320 || strings.ContainsAny(input.Username, "\r\n") {
		return AccountInput{}, validation("/account", "validation.length")
	}
	if input.IMAPSecurity != "tls" && input.IMAPSecurity != "starttls" {
		return AccountInput{}, validation("/imapSecurity", "validation.enum")
	}
	if input.SMTPSecurity != "tls" && input.SMTPSecurity != "starttls" {
		return AccountInput{}, validation("/smtpSecurity", "validation.enum")
	}
	if err := service.policy.Validate(input.IMAPHost, input.IMAPPort, "imap"); err != nil {
		return AccountInput{}, validation("/imapHost", "mail.endpoint.rejected")
	}
	if err := service.policy.Validate(input.SMTPHost, input.SMTPPort, "smtp"); err != nil {
		return AccountInput{}, validation("/smtpHost", "mail.endpoint.rejected")
	}
	if passwordRequired && input.Password == "" {
		return AccountInput{}, validation("/password", "validation.required")
	}
	if len(input.Password) > 4096 {
		return AccountInput{}, validation("/password", "validation.length")
	}
	return input, nil
}

func credentialSubject(workspaceID, accountID, userID ids.UUID) string {
	return workspaceID.String() + ":" + accountID.String() + ":" + userID.String()
}

func imapConfig(account dbgen.MailboxAccount) ConnectionConfig {
	return ConnectionConfig{Host: account.ImapHost, Port: int(account.ImapPort), Security: account.ImapSecurity, Username: account.Username}
}

func smtpConfig(account dbgen.MailboxAccount) ConnectionConfig {
	return ConnectionConfig{Host: account.SmtpHost, Port: int(account.SmtpPort), Security: account.SmtpSecurity, Username: account.Username}
}

func mapAccount(
	id pgtype.UUID, displayName, email, username, imapHost string, imapPort int32,
	imapSecurity, smtpHost string, smtpPort int32, smtpSecurity string, syncEnabled bool,
	syncState string, lastSync pgtype.Timestamptz, lastError *string, version int64,
) Account {
	return Account{
		ID: mustUUID(id), DisplayName: displayName, Email: email, Username: username,
		IMAPHost: imapHost, IMAPPort: int(imapPort), IMAPSecurity: imapSecurity,
		SMTPHost: smtpHost, SMTPPort: int(smtpPort), SMTPSecurity: smtpSecurity,
		SyncEnabled: syncEnabled, SyncState: publicSyncState(syncState), LastSyncAt: optionalTime(lastSync),
		LastErrorCode: lastError, Version: version,
	}
}

func mapFolder(row dbgen.MailboxFolder) Folder {
	return Folder{
		ID: mustUUID(row.ID), AccountID: mustUUID(row.AccountID), RemoteName: row.RemoteName,
		DisplayName: row.DisplayName, SpecialUse: row.SpecialUse, UIDValidity: row.UidValidity,
		UIDNext: row.UidNext, HighestUID: row.HighestUid, TotalCount: row.TotalCount,
		UnreadCount: row.UnreadCount, LastSyncAt: optionalTime(row.LastSyncAt),
	}
}

func mapMessage(row dbgen.MailboxMessage) Message {
	return Message{
		ID: mustUUID(row.ID), AccountID: mustUUID(row.AccountID), FolderID: mustUUID(row.FolderID),
		RemoteUID: row.RemoteUid, InternetMessageID: row.InternetMessageID, Subject: row.Subject,
		Sender: Address{Name: row.SenderName, Address: row.SenderAddress}, Recipients: append([]byte(nil), row.Recipients...),
		SentAt: optionalTime(row.SentAt), ReceivedAt: row.ReceivedAt.Time.UTC(), Flags: append([]string(nil), row.Flags...),
		SizeBytes: row.SizeBytes, Snippet: row.Snippet, HasAttachments: row.HasAttachments,
		BodyState: publicBodyState(row.BodyState),
	}
}

func publicSyncState(value string) string {
	switch value {
	case "syncing", "ready", "error":
		return value
	default:
		return "idle"
	}
}

func publicBodyState(value string) string {
	switch value {
	case "ready":
		return "cached"
	case "error":
		return "unavailable"
	default:
		return "metadata"
	}
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func mustUUID(value pgtype.UUID) ids.UUID {
	result, _ := ids.FromPG(value)
	return result
}

type messageCursor struct {
	Time time.Time `json:"t"`
	ID   string    `json:"i"`
}

func encodeCursor(value time.Time, id ids.UUID) string {
	payload, _ := json.Marshal(messageCursor{Time: value.UTC(), ID: id.String()})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeCursor(value string) (pgtype.Timestamptz, pgtype.UUID, error) {
	if value == "" {
		return pgtype.Timestamptz{}, pgtype.UUID{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(payload) > 256 {
		return pgtype.Timestamptz{}, pgtype.UUID{}, errors.New("invalid cursor")
	}
	var cursor messageCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.Time.IsZero() {
		return pgtype.Timestamptz{}, pgtype.UUID{}, errors.New("invalid cursor")
	}
	id, err := ids.Parse(cursor.ID)
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, errors.New("invalid cursor")
	}
	return pgtype.Timestamptz{Time: cursor.Time.UTC(), Valid: true}, id.PG(), nil
}

func (service *Service) acquireConnection(ctx context.Context) (func(), error) {
	if service.connections == nil {
		return func() {}, nil
	}
	select {
	case service.connections <- struct{}{}:
		return func() { <-service.connections }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func boundedUint32(value uint32) int32 {
	if value > 2147483647 {
		return 2147483647
	}
	return int32(value)
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
