package pagination

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const cursorVersion = 1

type cursorPayload struct {
	Version     int    `json:"v"`
	Timestamp   string `json:"t"`
	ID          string `json:"i"`
	Fingerprint string `json:"f"`
}

func Encode(timestamp time.Time, id ids.UUID, filter string) (string, error) {
	payload, err := json.Marshal(cursorPayload{
		Version:     cursorVersion,
		Timestamp:   timestamp.UTC().Format(time.RFC3339Nano),
		ID:          id.String(),
		Fingerprint: fingerprint(filter),
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func Decode(value, filter string) (time.Time, ids.UUID, error) {
	if value == "" {
		var maximum ids.UUID
		for index := range maximum {
			maximum[index] = 0xff
		}
		return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), maximum, nil
	}
	if len(value) > 512 {
		return time.Time{}, ids.UUID{}, errors.New("cursor too long")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, ids.UUID{}, errors.New("invalid cursor encoding")
	}
	var payload cursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return time.Time{}, ids.UUID{}, errors.New("invalid cursor payload")
	}
	if payload.Version != cursorVersion || payload.Fingerprint != fingerprint(filter) {
		return time.Time{}, ids.UUID{}, errors.New("cursor does not match query")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, payload.Timestamp)
	if err != nil {
		return time.Time{}, ids.UUID{}, errors.New("invalid cursor timestamp")
	}
	id, err := ids.Parse(payload.ID)
	if err != nil {
		return time.Time{}, ids.UUID{}, errors.New("invalid cursor id")
	}
	return timestamp.UTC(), id, nil
}

func fingerprint(filter string) string {
	digest := sha256.Sum256([]byte(filter))
	return base64.RawURLEncoding.EncodeToString(digest[:12])
}
