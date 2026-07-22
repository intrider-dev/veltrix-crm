package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/mailbox"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const mailboxOperationTimeout = 30 * time.Second

type mailboxAccountRequest struct {
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	Username     string `json:"username"`
	IMAPHost     string `json:"imapHost"`
	IMAPPort     int    `json:"imapPort"`
	IMAPSecurity string `json:"imapSecurity"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPSecurity string `json:"smtpSecurity"`
	Password     string `json:"password"`
	SyncEnabled  bool   `json:"syncEnabled"`
}

type mailboxAddressRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type mailboxSendRequest struct {
	To        []mailboxAddressRequest `json:"to"`
	Cc        []mailboxAddressRequest `json:"cc"`
	Bcc       []mailboxAddressRequest `json:"bcc"`
	Subject   string                  `json:"subject"`
	PlainText string                  `json:"plainText"`
}

type mailboxSendResponse struct {
	OutgoingID string `json:"outgoingId"`
	Sent       bool   `json:"sent"`
	Queued     bool   `json:"queued,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"`
}

func (application *Application) registerMailboxRoutes(router chi.Router) {
	router.Get("/mail/accounts", application.listMailboxAccounts)
	router.Post("/mail/accounts", application.createMailboxAccount)
	router.Put("/mail/accounts/{accountId}", application.updateMailboxAccount)
	router.Delete("/mail/accounts/{accountId}", application.deleteMailboxAccount)
	router.Post("/mail/accounts/{accountId}/sync", application.syncMailboxAccount)
	router.Get("/mail/accounts/{accountId}/folders", application.listMailboxFolders)
	router.Get("/mail/folders/{folderId}/messages", application.listMailboxMessages)
	router.Get("/mail/messages/{messageId}/body", application.readMailboxMessageBody)
	router.Post("/mail/accounts/{accountId}/send", application.sendMailboxMessage)
}

func (application *Application) listMailboxAccounts(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) || !application.requireMailbox(writer, request) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []mailbox.Account
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.mailbox.ListAccounts(request.Context(), workspace, workspaceID, principal.UserID)
			return err
		})
	if writeError(application, writer, request, mailboxHTTPError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createMailboxAccount(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) || !application.requireMailbox(writer, request) {
		return
	}
	body, raw, err := httpx.DecodeJSON[mailboxAccountRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsRead,
		"mail.account.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, mutationMetadata events.Metadata) (any, int64, error) {
			account, createErr := application.mailbox.CreateAccount(
				request.Context(), workspace, workspaceID, principal.UserID, mailboxAccountInput(body),
			)
			if createErr != nil {
				return account, 0, mailboxHTTPError(createErr)
			}
			if createErr = recordMailboxMutation(request.Context(), workspace, mutationMetadata,
				principal.UserID, "mail.account.created", "mail_account", account.ID); createErr != nil {
				return account, 0, createErr
			}
			return account, account.Version, nil
		})
}

func (application *Application) updateMailboxAccount(writer http.ResponseWriter, request *http.Request) {
	workspaceID, accountID, ok := application.mailboxPathID(writer, request, "accountId")
	if !ok || !application.requireMailbox(writer, request) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[mailboxAccountRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result mailbox.Account
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.mailbox.UpdateAccount(request.Context(), workspace, workspaceID,
				principal.UserID, accountID, version, mailboxAccountInput(body))
			if err != nil {
				return mailboxHTTPError(err)
			}
			return recordMailboxMutation(request.Context(), workspace, metadata(request, workspaceID, principal),
				principal.UserID, "mail.account.updated", "mail_account", accountID)
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteMailboxAccount(writer http.ResponseWriter, request *http.Request) {
	workspaceID, accountID, ok := application.mailboxPathID(writer, request, "accountId")
	if !ok || !application.requireMailbox(writer, request) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			if deleteErr := application.mailbox.DeleteAccount(request.Context(), workspace, workspaceID,
				principal.UserID, accountID, version); deleteErr != nil {
				return mailboxHTTPError(deleteErr)
			}
			return recordMailboxMutation(request.Context(), workspace, metadata(request, workspaceID, principal),
				principal.UserID, "mail.account.deleted", "mail_account", accountID)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) syncMailboxAccount(writer http.ResponseWriter, request *http.Request) {
	workspaceID, accountID, ok := application.mailboxPathID(writer, request, "accountId")
	if !ok || !application.requireMailbox(writer, request) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	ctx, cancel := context.WithTimeout(request.Context(), mailboxOperationTimeout)
	defer cancel()
	var plan *mailbox.SyncPlan
	err := application.tenancy.WithWorkspace(ctx, principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var prepareErr error
			plan, prepareErr = application.mailbox.PrepareSync(ctx, workspace, workspaceID, principal.UserID, accountID)
			return mailboxHTTPError(prepareErr)
		})
	if writeError(application, writer, request, err) {
		return
	}
	defer plan.Close()
	snapshot, err := application.mailbox.FetchSync(ctx, plan)
	if err != nil {
		application.persistMailboxSyncFailure(request, workspaceID, accountID, mailbox.SyncFailureCode(err))
		writeError(application, writer, request, mailboxHTTPError(err))
		return
	}
	err = application.tenancy.WithWorkspace(ctx, principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			if applyErr := application.mailbox.ApplySync(
				ctx, workspace, workspaceID, principal.UserID, accountID, snapshot,
			); applyErr != nil {
				return applyErr
			}
			return recordMailboxMutation(ctx, workspace, metadata(request, workspaceID, principal),
				principal.UserID, "mail.account.synced", "mail_account", accountID)
		})
	if err != nil {
		application.persistMailboxSyncFailure(request, workspaceID, accountID, mailbox.SyncFailureCode(err))
		writeError(application, writer, request, mailboxHTTPError(err))
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, map[string]bool{"synced": true})
}

func (application *Application) persistMailboxSyncFailure(
	request *http.Request, workspaceID, accountID ids.UUID, errorCode string,
) {
	principal, _ := httpx.Principal(request.Context())
	stateCtx, cancel := context.WithTimeout(context.WithoutCancel(request.Context()), 3*time.Second)
	defer cancel()
	if err := application.tenancy.WithWorkspace(
		stateCtx, principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.mailbox.MarkSyncFailed(
				stateCtx, workspace, workspaceID, principal.UserID, accountID, errorCode,
			)
		},
	); err != nil {
		application.logger.Warn("persist mailbox sync failure", "error_code", "mail_sync_state_failed")
	}
}

func (application *Application) listMailboxFolders(writer http.ResponseWriter, request *http.Request) {
	workspaceID, accountID, ok := application.mailboxPathID(writer, request, "accountId")
	if !ok || !application.requireMailbox(writer, request) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []mailbox.Folder
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.mailbox.ListFolders(request.Context(), workspace, workspaceID, principal.UserID, accountID)
			return err
		})
	if writeError(application, writer, request, mailboxHTTPError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listMailboxMessages(writer http.ResponseWriter, request *http.Request) {
	workspaceID, folderID, ok := application.mailboxPathID(writer, request, "folderId")
	if !ok || !application.requireMailbox(writer, request) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result mailbox.MessagePage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.mailbox.ListMessages(request.Context(), workspace, workspaceID,
				principal.UserID, folderID, request.URL.Query().Get("cursor"), limit)
			return err
		})
	if writeError(application, writer, request, mailboxHTTPError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) readMailboxMessageBody(writer http.ResponseWriter, request *http.Request) {
	workspaceID, messageID, ok := application.mailboxPathID(writer, request, "messageId")
	if !ok || !application.requireMailbox(writer, request) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	ctx, cancel := context.WithTimeout(request.Context(), mailboxOperationTimeout)
	defer cancel()
	var plainText string
	err := application.tenancy.WithWorkspace(ctx, principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var readErr error
			plainText, readErr = application.mailbox.ReadBody(ctx, workspace, workspaceID, principal.UserID, messageID)
			return mailboxHTTPError(readErr)
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, map[string]string{"plainText": plainText})
}

func (application *Application) sendMailboxMessage(writer http.ResponseWriter, request *http.Request) {
	workspaceID, accountID, ok := application.mailboxPathID(writer, request, "accountId")
	if !ok || !application.requireMailbox(writer, request) {
		return
	}
	body, raw, err := httpx.DecodeJSON[mailboxSendRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsRead,
		"mail.message.send", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, mutationMetadata events.Metadata) (any, int64, error) {
			outgoingID, createErr := application.mailbox.CreateOutgoing(request.Context(), workspace, workspaceID,
				principal.UserID, accountID, mailboxSendInput(body))
			if createErr != nil {
				return mailboxSendResponse{}, 0, mailboxHTTPError(createErr)
			}
			if createErr = recordMailboxMutation(request.Context(), workspace, mutationMetadata, principal.UserID,
				"mail.message.queued", "mail_outgoing", outgoingID); createErr != nil {
				return mailboxSendResponse{}, 0, createErr
			}
			return mailboxSendResponse{OutgoingID: outgoingID.String(), Queued: true}, 1, nil
		})
}

func (application *Application) requireMailbox(writer http.ResponseWriter, request *http.Request) bool {
	if application.mailbox != nil {
		return true
	}
	httpx.WriteProblem(writer, request, application.logger, errx.ErrUnavailable)
	return false
}

func (application *Application) mailboxPathID(
	writer http.ResponseWriter, request *http.Request, name string,
) (ids.UUID, ids.UUID, bool) {
	return application.workspaceAndPathID(writer, request, name)
}

func mailboxAccountInput(body mailboxAccountRequest) mailbox.AccountInput {
	return mailbox.AccountInput{
		DisplayName: body.DisplayName, Email: body.Email, Username: body.Username,
		IMAPHost: body.IMAPHost, IMAPPort: body.IMAPPort, IMAPSecurity: body.IMAPSecurity,
		SMTPHost: body.SMTPHost, SMTPPort: body.SMTPPort, SMTPSecurity: body.SMTPSecurity,
		Password: body.Password, SyncEnabled: body.SyncEnabled,
	}
}

func mailboxSendInput(body mailboxSendRequest) mailbox.SendInput {
	return mailbox.SendInput{
		Recipients: mailbox.RecipientSet{
			To: mailboxAddresses(body.To), Cc: mailboxAddresses(body.Cc), Bcc: mailboxAddresses(body.Bcc),
		},
		Subject: body.Subject, PlainText: body.PlainText,
	}
}

func mailboxAddresses(input []mailboxAddressRequest) []mailbox.Address {
	result := make([]mailbox.Address, 0, len(input))
	for _, address := range input {
		result = append(result, mailbox.Address{Name: address.Name, Address: address.Address})
	}
	return result
}

func recordMailboxMutation(
	ctx context.Context, workspace *tenancy.WorkspaceTx, mutationMetadata events.Metadata,
	userID ids.UUID, eventType, aggregateType string, aggregateID ids.UUID,
) error {
	return events.RecordTargeted(ctx, workspace.Queries, mutationMetadata, events.Mutation{
		Action: eventType, EventType: eventType, AggregateType: aggregateType, AggregateID: aggregateID,
		Summary: map[string]any{"fields": []string{"state"}}, Payload: map[string]any{"id": aggregateID.String()},
	}, []ids.UUID{userID})
}

func mailboxHTTPError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mailbox.ErrEndpointRejected):
		return errx.ErrSecurityRejected
	case errors.Is(err, mailbox.ErrEndpointUnavailable), errors.Is(err, mailbox.ErrIMAPOperation),
		errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return errx.ErrUnavailable
	case errors.Is(err, mailbox.ErrMessageTooLarge):
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/message", Code: "mail.message.too_large"}}}
	case errors.Is(err, mailbox.ErrMalformedMessage):
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/message", Code: "mail.message.invalid"}}}
	default:
		return err
	}
}
