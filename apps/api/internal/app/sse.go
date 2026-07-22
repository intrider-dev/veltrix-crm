package app

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/notifications"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) events(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		httpx.WriteProblem(writer, request, application.logger, fmt.Errorf("streaming unsupported"))
		return
	}
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	lastEventID := strings.TrimSpace(request.Header.Get("Last-Event-ID"))
	var anchorID ids.UUID
	if lastEventID != "" {
		anchorID, err = ids.Parse(lastEventID)
		if err != nil {
			httpx.WriteProblem(writer, request, application.logger, &errx.ValidationError{Fields: []errx.FieldError{{
				Pointer: "/headers/Last-Event-ID", Code: "validation.uuid.invalid",
			}}})
			return
		}
	}

	live, unsubscribe := application.hub.Subscribe(workspaceID.String(), principal.UserID.String())
	defer unsubscribe()
	var replay []dbgen.ListSSEEventsAfterForRecipientRow
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			if lastEventID == "" {
				return nil
			}
			var replayErr error
			replay, replayErr = workspace.Queries.ListSSEEventsAfterForRecipient(request.Context(), dbgen.ListSSEEventsAfterForRecipientParams{
				WorkspaceID: workspaceID.PG(), AnchorID: anchorID.PG(), RecipientUserID: principal.UserID.PG(),
			})
			return replayErr
		})
	if writeError(application, writer, request, err) {
		return
	}

	headers := writer.Header()
	headers.Set("Content-Type", "text/event-stream; charset=utf-8")
	headers.Set("Cache-Control", "no-store")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	sent := make(map[string]struct{}, len(replay))
	for _, row := range replay {
		id := apiID(row.ID).String()
		event := notifications.Event{
			ID: id, Type: row.EventType, Data: row.Data,
			Audience: notifications.AudienceWorkspace,
		}
		if row.RecipientUserID.Valid {
			event.Audience = notifications.AudienceUser
			event.RecipientUserID = apiID(row.RecipientUserID).String()
		}
		if !visibleSSEEvent(principal.UserID, event) {
			continue
		}
		writeSSE(writer, event)
		sent[id] = struct{}{}
	}
	_, _ = writer.Write([]byte(": connected\n\n"))
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	expiresIn := time.Until(principal.ExpiresAt)
	if expiresIn <= 0 {
		return
	}
	expiry := time.NewTimer(expiresIn)
	defer expiry.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case <-expiry.C:
			return
		case event, open := <-live:
			if !open {
				return
			}
			if _, duplicate := sent[event.ID]; duplicate {
				continue
			}
			if !visibleSSEEvent(principal.UserID, event) {
				continue
			}
			writeSSE(writer, event)
			flusher.Flush()
		case <-heartbeat.C:
			if !application.membershipStillActive(request, principal, workspaceID) {
				return
			}
			_, _ = writer.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		}
	}
}

func visibleSSEEvent(userID ids.UUID, event notifications.Event) bool {
	return notifications.EventVisibleTo(event, userID.String())
}

func (application *Application) membershipStillActive(request *http.Request, principal identity.Principal, workspaceID ids.UUID) bool {
	ctx, cancel := application.readinessContext(request.Context())
	defer cancel()
	err := application.tenancy.WithWorkspace(ctx, principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(*tenancy.WorkspaceTx) error { return nil })
	return err == nil
}

func writeSSE(writer http.ResponseWriter, event notifications.Event) {
	id := strings.NewReplacer("\r", "", "\n", "").Replace(event.ID)
	eventType := strings.NewReplacer("\r", "", "\n", "").Replace(event.Type)
	_, _ = fmt.Fprintf(writer, "id: %s\nevent: %s\n", id, eventType)
	for _, line := range bytes.Split(event.Data, []byte{'\n'}) {
		_, _ = writer.Write([]byte("data: "))
		_, _ = writer.Write(line)
		_, _ = writer.Write([]byte{'\n'})
	}
	_, _ = writer.Write([]byte{'\n'})
}
