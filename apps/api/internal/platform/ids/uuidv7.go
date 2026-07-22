package ids

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type UUID [16]byte

func NewV7() (UUID, error) {
	return NewV7At(time.Now().UTC())
}

func NewV7At(now time.Time) (UUID, error) {
	var id UUID
	milliseconds := uint64(now.UnixMilli())
	id[0] = byte(milliseconds >> 40)
	id[1] = byte(milliseconds >> 32)
	id[2] = byte(milliseconds >> 24)
	id[3] = byte(milliseconds >> 16)
	id[4] = byte(milliseconds >> 8)
	id[5] = byte(milliseconds)
	if _, err := rand.Read(id[6:]); err != nil {
		return UUID{}, fmt.Errorf("generate uuid randomness: %w", err)
	}
	id[6] = (id[6] & 0x0f) | 0x70
	id[8] = (id[8] & 0x3f) | 0x80
	return id, nil
}

func Parse(value string) (UUID, error) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return UUID{}, errors.New("invalid UUID format")
	}
	compact := value[:8] + value[9:13] + value[14:18] + value[19:23] + value[24:]
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != 16 {
		return UUID{}, errors.New("invalid UUID format")
	}
	var id UUID
	copy(id[:], decoded)
	return id, nil
}

func MustParse(value string) UUID {
	id, err := Parse(value)
	if err != nil {
		panic(err)
	}
	return id
}

func (id UUID) String() string {
	var output [36]byte
	hex.Encode(output[0:8], id[0:4])
	output[8] = '-'
	hex.Encode(output[9:13], id[4:6])
	output[13] = '-'
	hex.Encode(output[14:18], id[6:8])
	output[18] = '-'
	hex.Encode(output[19:23], id[8:10])
	output[23] = '-'
	hex.Encode(output[24:36], id[10:16])
	return string(output[:])
}

func (id UUID) PG() pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}

func FromPG(value pgtype.UUID) (UUID, bool) {
	if !value.Valid {
		return UUID{}, false
	}
	return UUID(value.Bytes), true
}
