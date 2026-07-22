package collaboration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/pagination"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const (
	maxConversationMembers = 50
	maxConversationPage    = 50
	defaultMessagePage     = 50
)

type Member struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type Conversation struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Title         string     `json:"title"`
	Members       []Member   `json:"members"`
	UnreadCount   int64      `json:"unreadCount"`
	LastMessageAt *time.Time `json:"lastMessageAt,omitempty"`
	Version       int64      `json:"version"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type ConversationInput struct {
	Title         string
	MemberUserIDs []ids.UUID
}

type Reaction struct {
	Emoji  string `json:"emoji"`
	UserID string `json:"userId"`
}

type Message struct {
	ID                string     `json:"id"`
	ConversationID    string     `json:"conversationId"`
	SenderUserID      string     `json:"senderUserId"`
	SenderDisplayName string     `json:"senderDisplayName"`
	Kind              string     `json:"kind"`
	Body              string     `json:"body"`
	ReplyToMessageID  *string    `json:"replyToMessageId,omitempty"`
	EditedAt          *time.Time `json:"editedAt,omitempty"`
	Pinned            bool       `json:"pinned"`
	Reactions         []Reaction `json:"reactions"`
	Version           int64      `json:"version"`
	CreatedAt         time.Time  `json:"createdAt"`
}

type MessageInput struct {
	Kind             string
	Body             string
	ReplyToMessageID *ids.UUID
}

type MessagePage struct {
	Items      []Message `json:"items"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

type Attachment struct {
	ID          string    `json:"id"`
	MessageID   string    `json:"messageId"`
	DisplayName string    `json:"displayName"`
	MediaType   string    `json:"mediaType"`
	SizeBytes   int64     `json:"sizeBytes"`
	ScanState   string    `json:"scanState"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) List(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, userID ids.UUID,
) ([]Conversation, error) {
	rows, err := workspace.Queries.ListUserConversations(ctx, dbgen.ListUserConversationsParams{
		UserID: userID.PG(), WorkspaceID: workspaceID.PG(), PageLimit: maxConversationPage,
	})
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	items := make([]Conversation, 0, len(rows))
	for _, row := range rows {
		item, mapErr := conversation(row.ID, row.ConversationType, row.Title, row.LastMessageAt,
			row.Version, row.Members, row.UnreadCount, row.CreatedAt, row.UpdatedAt)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (service *Service) Get(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, conversationID, userID ids.UUID,
) (Conversation, error) {
	row, err := workspace.Queries.GetUserConversation(ctx, dbgen.GetUserConversationParams{
		UserID: userID.PG(), WorkspaceID: workspaceID.PG(), ConversationID: conversationID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, errx.ErrNotFound
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("get conversation: %w", err)
	}
	return conversation(row.ID, row.ConversationType, row.Title, row.LastMessageAt,
		row.Version, row.Members, row.UnreadCount, row.CreatedAt, row.UpdatedAt)
}

func (service *Service) Create(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, actorID ids.UUID, input ConversationInput,
) (Conversation, error) {
	input.Title = strings.TrimSpace(input.Title)
	seen := map[string]struct{}{actorID.String(): {}}
	memberIDs := make([]ids.UUID, 0, len(input.MemberUserIDs)+1)
	memberIDs = append(memberIDs, actorID)
	for _, userID := range input.MemberUserIDs {
		if _, exists := seen[userID.String()]; exists {
			continue
		}
		seen[userID.String()] = struct{}{}
		memberIDs = append(memberIDs, userID)
	}
	if len(memberIDs) < 2 || len(memberIDs) > maxConversationMembers {
		return Conversation{}, validation("/memberUserIds", "validation.items.range")
	}
	for _, userID := range memberIDs {
		active, err := workspace.Queries.ActiveWorkspaceUserExists(ctx, dbgen.ActiveWorkspaceUserExistsParams{
			WorkspaceID: workspaceID.PG(), UserID: userID.PG(),
		})
		if err != nil {
			return Conversation{}, fmt.Errorf("validate conversation member: %w", err)
		}
		if !active {
			return Conversation{}, validation("/memberUserIds", "validation.reference.invalid")
		}
	}
	conversationType := "group"
	var directKey *string
	if len(memberIDs) == 2 {
		conversationType = "direct"
		input.Title = ""
		key := directConversationKey(memberIDs)
		directKey = &key
		existingID, err := workspace.Queries.FindDirectConversation(ctx, dbgen.FindDirectConversationParams{
			WorkspaceID: workspaceID.PG(), DirectKey: directKey,
		})
		if err == nil {
			id, _ := ids.FromPG(existingID)
			return service.Get(ctx, workspace, workspaceID, id, actorID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Conversation{}, fmt.Errorf("find direct conversation: %w", err)
		}
	} else if utf8.RuneCountInString(input.Title) < 1 || utf8.RuneCountInString(input.Title) > 160 {
		return Conversation{}, validation("/title", "validation.length")
	}
	conversationID, err := ids.NewV7()
	if err != nil {
		return Conversation{}, err
	}
	if _, err := workspace.Queries.CreateConversation(ctx, dbgen.CreateConversationParams{
		WorkspaceID: workspaceID.PG(), ConversationID: conversationID.PG(),
		ConversationType: conversationType, Title: input.Title, DirectKey: directKey, CreatedBy: actorID.PG(),
	}); err != nil {
		if directKey != nil {
			existingID, findErr := workspace.Queries.FindDirectConversation(ctx, dbgen.FindDirectConversationParams{
				WorkspaceID: workspaceID.PG(), DirectKey: directKey,
			})
			if findErr == nil {
				id, _ := ids.FromPG(existingID)
				return service.Get(ctx, workspace, workspaceID, id, actorID)
			}
		}
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	for _, userID := range memberIDs {
		role := "member"
		if userID == actorID {
			role = "owner"
		}
		if err := workspace.Queries.AddConversationMember(ctx, dbgen.AddConversationMemberParams{
			WorkspaceID: workspaceID.PG(), ConversationID: conversationID.PG(),
			UserID: userID.PG(), MemberRole: role,
		}); err != nil {
			return Conversation{}, fmt.Errorf("add conversation member: %w", err)
		}
	}
	return service.Get(ctx, workspace, workspaceID, conversationID, actorID)
}

func (service *Service) ListMessages(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, conversationID, userID ids.UUID, cursor string, limit int,
) (MessagePage, error) {
	member, err := workspace.Queries.ConversationMemberExists(ctx, dbgen.ConversationMemberExistsParams{
		WorkspaceID: workspaceID.PG(), ConversationID: conversationID.PG(), UserID: userID.PG(),
	})
	if err != nil {
		return MessagePage{}, fmt.Errorf("authorize conversation: %w", err)
	}
	if !member {
		return MessagePage{}, errx.ErrNotFound
	}
	if limit < 1 {
		limit = defaultMessagePage
	}
	if limit > 100 {
		limit = 100
	}
	fingerprint := "chat=" + conversationID.String() + ":" + userID.String()
	cursorTime, cursorID, err := pagination.Decode(cursor, fingerprint)
	if err != nil {
		return MessagePage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListConversationMessages(ctx, dbgen.ListConversationMessagesParams{
		UserID: userID.PG(), WorkspaceID: workspaceID.PG(), ConversationID: conversationID.PG(),
		CursorCreatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true}, CursorID: cursorID.PG(),
		PageLimit: int32(limit + 1),
	})
	if err != nil {
		return MessagePage{}, fmt.Errorf("list chat messages: %w", err)
	}
	page := MessagePage{Items: make([]Message, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			break
		}
		item, mapErr := message(row.ID, row.ConversationID, row.SenderUserID,
			row.SenderDisplayName, row.MessageKind, row.Body, row.ReplyToMessageID,
			row.EditedAt, row.Pinned, row.Reactions, row.Version, row.CreatedAt)
		if mapErr != nil {
			return MessagePage{}, mapErr
		}
		page.Items = append(page.Items, item)
	}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.CreatedAt.Time, lastID, fingerprint)
		if err != nil {
			return MessagePage{}, fmt.Errorf("encode chat cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) Send(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, conversationID, senderID ids.UUID, input MessageInput,
) (Message, error) {
	input.Kind = strings.TrimSpace(input.Kind)
	input.Body = strings.TrimSpace(input.Body)
	if input.Kind == "" {
		input.Kind = "text"
	}
	if input.Kind != "text" || utf8.RuneCountInString(input.Body) < 1 || utf8.RuneCountInString(input.Body) > 10000 {
		return Message{}, validation("/body", "validation.length")
	}
	messageID, err := ids.NewV7()
	if err != nil {
		return Message{}, err
	}
	row, err := workspace.Queries.CreateMessage(ctx, dbgen.CreateMessageParams{
		WorkspaceID: workspaceID.PG(), MessageID: messageID.PG(), ConversationID: conversationID.PG(),
		SenderUserID: senderID.PG(), MessageKind: input.Kind, Body: input.Body,
		ReplyToMessageID: optionalUUID(input.ReplyToMessageID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, errx.ErrNotFound
	}
	if err != nil {
		return Message{}, fmt.Errorf("create chat message: %w", err)
	}
	recipients, err := workspace.Queries.ListConversationRecipientIDs(ctx, dbgen.ListConversationRecipientIDsParams{
		WorkspaceID: workspaceID.PG(), ConversationID: conversationID.PG(),
	})
	if err != nil || len(recipients) < 2 || len(recipients) > maxConversationMembers {
		if err == nil {
			err = errors.New("invalid conversation recipient count")
		}
		return Message{}, fmt.Errorf("list chat recipients: %w", err)
	}
	data, err := json.Marshal(map[string]any{
		"conversationId": conversationID.String(), "messageId": messageID.String(),
		"senderUserId": senderID.String(), "version": row.Version,
	})
	if err != nil {
		return Message{}, err
	}
	for _, rawRecipient := range recipients {
		recipient, ok := ids.FromPG(rawRecipient)
		if !ok {
			return Message{}, errors.New("invalid chat recipient")
		}
		eventID, idErr := ids.NewV7()
		if idErr != nil {
			return Message{}, idErr
		}
		if err := workspace.Queries.InsertUserSSEEvent(ctx, dbgen.InsertUserSSEEventParams{
			WorkspaceID: workspaceID.PG(), EventID: eventID.PG(), EventType: "chat.message.created",
			Data: data, RecipientUserID: recipient.PG(),
		}); err != nil {
			return Message{}, fmt.Errorf("emit private chat event: %w", err)
		}
	}
	return message(row.ID, row.ConversationID, row.SenderUserID, row.SenderDisplayName,
		row.MessageKind, row.Body, row.ReplyToMessageID, row.EditedAt, false, "[]",
		row.Version, row.CreatedAt)
}

func (service *Service) MarkRead(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, conversationID, userID ids.UUID,
) error {
	changed, err := workspace.Queries.MarkConversationRead(ctx, dbgen.MarkConversationReadParams{
		WorkspaceID: workspaceID.PG(), ConversationID: conversationID.PG(), UserID: userID.PG(),
	})
	if err != nil {
		return fmt.Errorf("mark conversation read: %w", err)
	}
	if changed == 0 {
		return errx.ErrNotFound
	}
	return nil
}

func (service *Service) ListAttachments(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, conversationID, userID ids.UUID,
) ([]Attachment, error) {
	rows, err := workspace.Queries.ListConversationAttachments(ctx, dbgen.ListConversationAttachmentsParams{
		ConversationID: conversationID.PG(), UserID: userID.PG(), WorkspaceID: workspaceID.PG(),
	})
	if err != nil {
		return nil, fmt.Errorf("list conversation attachments: %w", err)
	}
	items := make([]Attachment, 0, len(rows))
	for _, row := range rows {
		attachmentID, _ := ids.FromPG(row.ID)
		messageID, _ := ids.FromPG(row.MessageID)
		items = append(items, Attachment{ID: attachmentID.String(), MessageID: messageID.String(),
			DisplayName: row.DisplayName, MediaType: row.MediaType, SizeBytes: row.SizeBytes,
			ScanState: row.ScanState, CreatedAt: row.CreatedAt.Time.UTC()})
	}
	return items, nil
}

func (service *Service) React(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, messageID, userID ids.UUID, emoji string, add bool,
) error {
	visible, err := service.MessageVisible(ctx, workspace, workspaceID, messageID, userID)
	if err != nil {
		return err
	}
	if !visible {
		return errx.ErrNotFound
	}
	emoji = strings.TrimSpace(emoji)
	if utf8.RuneCountInString(emoji) < 1 || utf8.RuneCountInString(emoji) > 8 || len(emoji) > 32 {
		return validation("/emoji", "validation.length")
	}
	if add {
		return workspace.Queries.AddMessageReaction(ctx, dbgen.AddMessageReactionParams{
			WorkspaceID: workspaceID.PG(), MessageID: messageID.PG(), UserID: userID.PG(), Emoji: emoji,
		})
	}
	_, err = workspace.Queries.RemoveMessageReaction(ctx, dbgen.RemoveMessageReactionParams{
		WorkspaceID: workspaceID.PG(), MessageID: messageID.PG(), UserID: userID.PG(), Emoji: emoji,
	})
	return err
}

func (service *Service) Pin(
	ctx context.Context, workspace *tenancy.WorkspaceTx,
	workspaceID, messageID, userID ids.UUID, pin bool,
) error {
	visible, err := service.MessageVisible(ctx, workspace, workspaceID, messageID, userID)
	if err != nil {
		return err
	}
	if !visible {
		return errx.ErrNotFound
	}
	if pin {
		return workspace.Queries.PinMessage(ctx, dbgen.PinMessageParams{
			UserID: userID.PG(), WorkspaceID: workspaceID.PG(), MessageID: messageID.PG(),
		})
	}
	_, err = workspace.Queries.UnpinMessage(ctx, dbgen.UnpinMessageParams{
		WorkspaceID: workspaceID.PG(), MessageID: messageID.PG(), UserID: userID.PG(),
	})
	return err
}

func (service *Service) MessageVisible(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, messageID, userID ids.UUID,
) (bool, error) {
	return workspace.Queries.ChatMessageVisible(ctx, dbgen.ChatMessageVisibleParams{
		UserID: userID.PG(), WorkspaceID: workspaceID.PG(), MessageID: messageID.PG(),
	})
}

func directConversationKey(memberIDs []ids.UUID) string {
	values := make([]string, 0, len(memberIDs))
	for _, id := range memberIDs {
		values = append(values, id.String())
	}
	sort.Strings(values)
	digest := sha256.Sum256([]byte(strings.Join(values, ":")))
	return hex.EncodeToString(digest[:])
}

func conversation(
	id pgtype.UUID, conversationType, title string, lastMessageAt pgtype.Timestamptz,
	version int64, membersJSON string, unreadCount int64,
	createdAt, updatedAt pgtype.Timestamptz,
) (Conversation, error) {
	conversationID, _ := ids.FromPG(id)
	members := make([]Member, 0)
	if err := json.Unmarshal([]byte(membersJSON), &members); err != nil {
		return Conversation{}, fmt.Errorf("decode conversation members: %w", err)
	}
	var lastMessage *time.Time
	if lastMessageAt.Valid {
		value := lastMessageAt.Time.UTC()
		lastMessage = &value
	}
	return Conversation{ID: conversationID.String(), Type: conversationType, Title: title,
		Members: members, UnreadCount: unreadCount, LastMessageAt: lastMessage,
		Version: version, CreatedAt: createdAt.Time.UTC(), UpdatedAt: updatedAt.Time.UTC()}, nil
}

func message(
	id, conversationID, senderID pgtype.UUID, senderName, kind, body string,
	replyID pgtype.UUID, editedAt pgtype.Timestamptz, pinned bool, reactionsJSON string,
	version int64, createdAt pgtype.Timestamptz,
) (Message, error) {
	messageID, _ := ids.FromPG(id)
	conversationUUID, _ := ids.FromPG(conversationID)
	senderUUID, _ := ids.FromPG(senderID)
	reactions := make([]Reaction, 0)
	if err := json.Unmarshal([]byte(reactionsJSON), &reactions); err != nil {
		return Message{}, fmt.Errorf("decode message reactions: %w", err)
	}
	return Message{ID: messageID.String(), ConversationID: conversationUUID.String(),
		SenderUserID: senderUUID.String(), SenderDisplayName: senderName, Kind: kind, Body: body,
		ReplyToMessageID: optionalID(replyID), EditedAt: optionalTime(editedAt), Pinned: pinned,
		Reactions: reactions, Version: version, CreatedAt: createdAt.Time.UTC()}, nil
}

func optionalUUID(value *ids.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return value.PG()
}

func optionalID(value pgtype.UUID) *string {
	id, ok := ids.FromPG(value)
	if !ok {
		return nil
	}
	text := id.String()
	return &text
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
