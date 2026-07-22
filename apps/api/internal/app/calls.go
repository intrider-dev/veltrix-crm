package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) registerCallRoutes(router chi.Router) {
	router.Get("/calls/config", application.callConfig)
	router.Post("/conversations/{conversationId}/calls", application.createCall)
	router.Get("/calls/{callId}", application.getCall)
	router.Post("/calls/{callId}/join-token", application.joinCall)
	router.Post("/calls/{callId}/decline", application.declineCall)
	router.Post("/calls/{callId}/leave", application.leaveCall)
	router.Post("/calls/{callId}/end", application.endCall)
}

func (application *Application) callConfig(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(*tenancy.WorkspaceTx) error { return nil })
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, apigen.CallConfig{Enabled: application.calls.Enabled()})
}

func (application *Application) createCall(writer http.ResponseWriter, request *http.Request) {
	workspaceID, conversationID, ok := application.workspaceAndPathID(writer, request, "conversationId")
	if !ok {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.CreateCallInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	if !application.callLimits.Allow(workspaceID.String()+":"+principal.UserID.String(), time.Now().UTC()) {
		httpx.WriteProblem(writer, request, application.logger, errx.ErrRateLimited)
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsRead,
		"calls.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, eventMetadata events.Metadata) (any, int64, error) {
			created, createErr := application.calls.Create(
				request.Context(), workspace, eventMetadata, conversationID, string(body.Kind),
			)
			return created, created.Version, createErr
		})
}

func (application *Application) getCall(writer http.ResponseWriter, request *http.Request) {
	application.withCall(writer, request, func(
		workspace *tenancy.WorkspaceTx, workspaceID, callID, actorID ids.UUID,
	) (any, error) {
		return application.calls.Get(request.Context(), workspace, workspaceID, callID, actorID)
	})
}

func (application *Application) joinCall(writer http.ResponseWriter, request *http.Request) {
	workspaceID, callID, ok := application.workspaceAndPathID(writer, request, "callId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var response any
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			call, grant, joinErr := application.calls.Join(request.Context(), workspace,
				workspaceID, callID, principal.UserID, principal.DisplayName)
			if joinErr != nil {
				return joinErr
			}
			response = struct {
				Call      any       `json:"call"`
				URL       string    `json:"url"`
				Token     string    `json:"token"`
				ExpiresAt time.Time `json:"expiresAt"`
			}{Call: call, URL: grant.URL, Token: grant.Token, ExpiresAt: grant.ExpiresAt}
			return nil
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, response)
}

func (application *Application) declineCall(writer http.ResponseWriter, request *http.Request) {
	application.mutateCallParticipant(writer, request, application.calls.Decline)
}

func (application *Application) leaveCall(writer http.ResponseWriter, request *http.Request) {
	application.mutateCallParticipant(writer, request, application.calls.Leave)
}

func (application *Application) mutateCallParticipant(
	writer http.ResponseWriter, request *http.Request,
	mutation func(context.Context, *tenancy.WorkspaceTx, ids.UUID, ids.UUID, ids.UUID) error,
) {
	workspaceID, callID, ok := application.workspaceAndPathID(writer, request, "callId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return mutation(request.Context(), workspace, workspaceID, callID, principal.UserID)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) endCall(writer http.ResponseWriter, request *http.Request) {
	workspaceID, callID, ok := application.workspaceAndPathID(writer, request, "callId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result any
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			ended, endErr := application.calls.End(request.Context(), workspace, metadata(request, workspaceID, principal), callID)
			result = ended
			return endErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) withCall(
	writer http.ResponseWriter, request *http.Request,
	load func(*tenancy.WorkspaceTx, ids.UUID, ids.UUID, ids.UUID) (any, error),
) {
	workspaceID, callID, ok := application.workspaceAndPathID(writer, request, "callId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result any
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = load(workspace, workspaceID, callID, principal.UserID)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}
