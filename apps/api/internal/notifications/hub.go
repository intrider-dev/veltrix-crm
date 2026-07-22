package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const subscriberBuffer = 32

type Event struct {
	ID              string
	Type            string
	Data            []byte
	Audience        Audience
	RecipientUserID string
}

type Audience uint8

const (
	AudienceWorkspace Audience = iota + 1
	AudienceUser
)

type Hub struct {
	logger      *slog.Logger
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]string
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{logger: logger, subscribers: make(map[string]map[chan Event]string)}
}

func (hub *Hub) Subscribe(workspaceID, userID string) (<-chan Event, func()) {
	channel := make(chan Event, subscriberBuffer)
	hub.mu.Lock()
	if hub.subscribers[workspaceID] == nil {
		hub.subscribers[workspaceID] = make(map[chan Event]string)
	}
	hub.subscribers[workspaceID][channel] = userID
	hub.mu.Unlock()
	var once sync.Once
	return channel, func() {
		once.Do(func() {
			hub.mu.Lock()
			delete(hub.subscribers[workspaceID], channel)
			if len(hub.subscribers[workspaceID]) == 0 {
				delete(hub.subscribers, workspaceID)
			}
			close(channel)
			hub.mu.Unlock()
		})
	}
}

func (hub *Hub) Run(ctx context.Context, dispatcherURL string) error {
	for {
		if err := hub.listen(ctx, dispatcherURL); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			hub.logger.Error("SSE listener disconnected", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
	}
}

func (hub *Hub) listen(ctx context.Context, dispatcherURL string) error {
	connection, err := pgx.Connect(ctx, dispatcherURL)
	if err != nil {
		return fmt.Errorf("connect SSE listener: %w", err)
	}
	defer func() { _ = connection.Close(context.Background()) }()
	if _, err := connection.Exec(ctx, "LISTEN veltrix_sse"); err != nil {
		return fmt.Errorf("listen SSE channel: %w", err)
	}
	for {
		notification, err := connection.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		workspaceID, event, ok := parseNotification(notification.Payload)
		if !ok {
			hub.logger.Warn("ignored malformed SSE notification")
			continue
		}
		workspaceUUID, workspaceValid := parseEventUUID(workspaceID)
		eventUUID, eventValid := parseEventUUID(event.ID)
		if !workspaceValid || !eventValid {
			hub.logger.Warn("ignored SSE notification with invalid identifiers")
			continue
		}
		row, err := dbgen.New(connection).GetSSEEventForDispatch(ctx, dbgen.GetSSEEventForDispatchParams{
			WorkspaceID: workspaceUUID.PG(), EventID: eventUUID.PG(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("hydrate SSE notification: %w", err)
		}
		event.Type = row.EventType
		event.Data = append([]byte(nil), row.Data...)
		event.Audience = AudienceWorkspace
		if row.RecipientUserID.Valid {
			recipient, recipientValid := ids.FromPG(row.RecipientUserID)
			if !recipientValid {
				hub.logger.Warn("ignored SSE event with invalid recipient")
				continue
			}
			event.Audience = AudienceUser
			event.RecipientUserID = recipient.String()
		}
		hub.publish(workspaceID, event)
	}
}

func (hub *Hub) publish(workspaceID string, event Event) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for channel, userID := range hub.subscribers[workspaceID] {
		if !EventVisibleTo(event, userID) {
			continue
		}
		select {
		case channel <- event:
		default:
			// A bounded channel deliberately drops the live wake-up. The client
			// reconnects and recovers durable events using Last-Event-ID.
		}
	}
}

// EventVisibleTo is the in-process live-stream boundary. Audience metadata is
// hydrated from dedicated database columns, never inferred from event JSON.
func EventVisibleTo(event Event, userID string) bool {
	switch event.Audience {
	case AudienceWorkspace:
		// Notifications are always private; fail closed if a malformed row is
		// ever hydrated without its required recipient.
		return event.Type != "notification.created"
	case AudienceUser:
		user, userErr := ids.Parse(userID)
		recipient, recipientErr := ids.Parse(event.RecipientUserID)
		return userErr == nil && recipientErr == nil && recipient == user
	default:
		return false
	}
}

func parseNotification(payload string) (string, Event, bool) {
	parts := strings.SplitN(payload, ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", Event{}, false
	}
	return parts[0], Event{ID: parts[1], Type: parts[2]}, true
}

func parseEventUUID(value string) (ids.UUID, bool) {
	id, err := ids.Parse(value)
	return id, err == nil
}
