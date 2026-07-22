package files

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
)

var (
	ErrInvalidStorageKey   = errors.New("invalid storage key")
	ErrObjectTooLarge      = errors.New("object exceeds size limit")
	ErrUnsupportedMedia    = errors.New("unsupported media type")
	ErrStorageKeyCollision = errors.New("storage key already exists")
)

type StoredBlob struct {
	SizeBytes      int64
	ChecksumSHA256 [sha256.Size]byte
	MediaType      string
}

type BlobStore interface {
	Put(context.Context, string, string, io.Reader, int64) (StoredBlob, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

type ScanResult string

const (
	ScanClean       ScanResult = "clean"
	ScanRejected    ScanResult = "rejected"
	ScanUnavailable ScanResult = "unavailable"
)

// Scanner is intentionally transport-neutral. An antivirus sidecar can be
// introduced later without granting it direct access to CRM authorization.
type Scanner interface {
	Scan(context.Context, BlobStore, string, StoredBlob) (ScanResult, error)
}

type UnavailableScanner struct{}

func (UnavailableScanner) Scan(context.Context, BlobStore, string, StoredBlob) (ScanResult, error) {
	return ScanUnavailable, nil
}
