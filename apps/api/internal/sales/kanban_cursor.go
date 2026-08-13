package sales

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type kanbanCursorPayload struct {
	Version     int    `json:"v"`
	Position    int32  `json:"p"`
	ID          string `json:"i"`
	Fingerprint string `json:"f"`
}

func encodeKanbanCursor(position int32, id ids.UUID, filter string) (string, error) {
	payload, err := json.Marshal(kanbanCursorPayload{
		Version: 1, Position: position, ID: id.String(), Fingerprint: salesCursorFingerprint(filter),
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeKanbanCursor(value, filter string) (int32, ids.UUID, error) {
	if value == "" {
		return -1, ids.UUID{}, nil
	}
	if len(value) > 512 {
		return 0, ids.UUID{}, errors.New("cursor too long")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, ids.UUID{}, errors.New("invalid cursor encoding")
	}
	var payload kanbanCursorPayload
	if json.Unmarshal(decoded, &payload) != nil || payload.Version != 1 || payload.Fingerprint != salesCursorFingerprint(filter) || payload.Position < 0 {
		return 0, ids.UUID{}, errors.New("invalid cursor payload")
	}
	id, err := ids.Parse(payload.ID)
	if err != nil {
		return 0, ids.UUID{}, errors.New("invalid cursor id")
	}
	return payload.Position, id, nil
}

func salesCursorFingerprint(filter string) string {
	digest := sha256.Sum256([]byte(filter))
	return base64.RawURLEncoding.EncodeToString(digest[:12])
}
