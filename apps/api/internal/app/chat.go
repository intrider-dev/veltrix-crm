package app

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/collaboration"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) registerChatRoutes(router chi.Router) {
	router.Get("/conversations", application.listConversations)
	router.Post("/conversations", application.createConversation)
	router.Get("/conversations/{conversationId}/messages", application.listChatMessages)
	router.Get("/conversations/{conversationId}/attachments", application.listChatAttachments)
	router.Post("/conversations/{conversationId}/messages", application.sendChatMessage)
	router.Post("/conversations/{conversationId}/read", application.markConversationRead)
	router.Put("/chat/messages/{messageId}/reactions", application.addChatReaction)
	router.Delete("/chat/messages/{messageId}/reactions", application.removeChatReaction)
	router.Put("/chat/messages/{messageId}/pin", application.pinChatMessage)
	router.Delete("/chat/messages/{messageId}/pin", application.unpinChatMessage)
}

func (application *Application) listChatAttachments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, conversationID, ok := application.workspaceAndPathID(writer, request, "conversationId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []collaboration.Attachment
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.chat.ListAttachments(request.Context(), workspace, workspaceID, conversationID, principal.UserID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listConversations(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []collaboration.Conversation
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.chat.List(request.Context(), workspace, workspaceID, principal.UserID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createConversation(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.CreateChatConversation](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	members := make([]ids.UUID, 0, len(body.MemberUserIds))
	for _, memberID := range body.MemberUserIds {
		members = append(members, ids.UUID(memberID))
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsRead,
		"chat.conversations.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			created, createErr := application.chat.Create(request.Context(), workspace,
				workspaceID, metadata.ActorID, collaboration.ConversationInput{
					Title: body.Title, MemberUserIDs: members,
				})
			return created, created.Version, createErr
		})
}

func (application *Application) listChatMessages(writer http.ResponseWriter, request *http.Request) {
	workspaceID, conversationID, ok := application.workspaceAndPathID(writer, request, "conversationId")
	if !ok {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result collaboration.MessagePage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.chat.ListMessages(request.Context(), workspace, workspaceID,
				conversationID, principal.UserID, request.URL.Query().Get("cursor"), limit)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) sendChatMessage(writer http.ResponseWriter, request *http.Request) {
	workspaceID, conversationID, ok := application.workspaceAndPathID(writer, request, "conversationId")
	if !ok {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.CreateChatMessage](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsRead,
		"chat.messages.send", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			created, createErr := application.chat.Send(request.Context(), workspace,
				workspaceID, conversationID, metadata.ActorID, collaboration.MessageInput{
					Kind: string(body.Kind), Body: body.Body,
					ReplyToMessageID: internalIDPointer(body.ReplyToMessageId),
				})
			return created, created.Version, createErr
		})
}

func (application *Application) markConversationRead(writer http.ResponseWriter, request *http.Request) {
	workspaceID, conversationID, ok := application.workspaceAndPathID(writer, request, "conversationId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.chat.MarkRead(request.Context(), workspace, workspaceID, conversationID, principal.UserID)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) addChatReaction(writer http.ResponseWriter, request *http.Request) {
	application.mutateChatReaction(writer, request, true)
}

func (application *Application) removeChatReaction(writer http.ResponseWriter, request *http.Request) {
	application.mutateChatReaction(writer, request, false)
}

func (application *Application) mutateChatReaction(writer http.ResponseWriter, request *http.Request, add bool) {
	workspaceID, messageID, ok := application.workspaceAndPathID(writer, request, "messageId")
	if !ok {
		return
	}
	emoji := strings.TrimSpace(request.URL.Query().Get("emoji"))
	if add {
		body, _, err := httpx.DecodeJSON[apigen.ChatReactionInput](writer, request, httpx.DefaultJSONLimit)
		if writeError(application, writer, request, err) {
			return
		}
		emoji = body.Emoji
	}
	principal, _ := httpx.Principal(request.Context())
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.chat.React(request.Context(), workspace, workspaceID, messageID, principal.UserID, emoji, add)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) pinChatMessage(writer http.ResponseWriter, request *http.Request) {
	application.mutateChatPin(writer, request, true)
}

func (application *Application) unpinChatMessage(writer http.ResponseWriter, request *http.Request) {
	application.mutateChatPin(writer, request, false)
}

func (application *Application) mutateChatPin(writer http.ResponseWriter, request *http.Request, pin bool) {
	workspaceID, messageID, ok := application.workspaceAndPathID(writer, request, "messageId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.chat.Pin(request.Context(), workspace, workspaceID, messageID, principal.UserID, pin)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
