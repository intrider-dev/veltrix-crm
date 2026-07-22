package files

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

var storageKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_-]{0,499}$`)

type LocalStore struct {
	root *os.Root
}

func NewLocalStore(directory string) (*LocalStore, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("local storage directory is required")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open local storage root: %w", err)
	}
	return &LocalStore{root: root}, nil
}

func (store *LocalStore) Close() error { return store.root.Close() }

func (store *LocalStore) Put(
	ctx context.Context,
	key string,
	declaredMediaType string,
	reader io.Reader,
	maxBytes int64,
) (StoredBlob, error) {
	if !validStorageKey(key) || reader == nil {
		return StoredBlob{}, ErrInvalidStorageKey
	}
	if maxBytes < 1 {
		return StoredBlob{}, errors.New("positive object size limit is required")
	}
	parent := path.Dir(key)
	if parent != "." {
		if err := store.root.MkdirAll(parent, 0o750); err != nil {
			return StoredBlob{}, fmt.Errorf("create object directory: %w", err)
		}
	}
	if _, err := store.root.Stat(key); err == nil {
		return StoredBlob{}, ErrStorageKeyCollision
	} else if !errors.Is(err, os.ErrNotExist) {
		return StoredBlob{}, fmt.Errorf("check object collision: %w", err)
	}
	temporaryKey, err := temporaryStorageKey(key)
	if err != nil {
		return StoredBlob{}, err
	}
	file, err := store.root.OpenFile(temporaryKey, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return StoredBlob{}, fmt.Errorf("create temporary object: %w", err)
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = store.root.Remove(temporaryKey)
		}
	}()

	buffered := bufio.NewReaderSize(reader, 32<<10)
	sniff, peekErr := buffered.Peek(512)
	if peekErr != nil && !errors.Is(peekErr, io.EOF) && !errors.Is(peekErr, bufio.ErrBufferFull) {
		return StoredBlob{}, fmt.Errorf("inspect upload: %w", peekErr)
	}
	mediaType, err := validateMediaType(declaredMediaType, sniff)
	if err != nil {
		return StoredBlob{}, err
	}
	hasher := sha256.New()
	written, err := copyWithContext(ctx, io.MultiWriter(file, hasher), buffered, maxBytes+1)
	if err != nil {
		return StoredBlob{}, fmt.Errorf("stream upload: %w", err)
	}
	if written > maxBytes {
		return StoredBlob{}, ErrObjectTooLarge
	}
	if err := file.Sync(); err != nil {
		return StoredBlob{}, fmt.Errorf("sync uploaded object: %w", err)
	}
	if err := file.Close(); err != nil {
		return StoredBlob{}, fmt.Errorf("close uploaded object: %w", err)
	}
	if err := store.root.Rename(temporaryKey, key); err != nil {
		return StoredBlob{}, fmt.Errorf("commit uploaded object: %w", err)
	}
	committed = true
	var checksum [sha256.Size]byte
	copy(checksum[:], hasher.Sum(nil))
	return StoredBlob{SizeBytes: written, ChecksumSHA256: checksum, MediaType: mediaType}, nil
}

func (store *LocalStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if !validStorageKey(key) {
		return nil, ErrInvalidStorageKey
	}
	file, err := store.root.Open(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("open stored object: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("stat stored object: %w", err)
		}
		return nil, ErrInvalidStorageKey
	}
	return file, nil
}

func (store *LocalStore) Delete(_ context.Context, key string) error {
	if !validStorageKey(key) {
		return ErrInvalidStorageKey
	}
	err := store.root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete stored object: %w", err)
	}
	return nil
}

func validStorageKey(key string) bool {
	if !storageKeyPattern.MatchString(key) || strings.Contains(key, "//") {
		return false
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func temporaryStorageKey(key string) (string, error) {
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate temporary object key: %w", err)
	}
	return key + "-tmp-" + hex.EncodeToString(random[:]), nil
}

func validateMediaType(declared string, contents []byte) (string, error) {
	detected := strings.ToLower(strings.TrimSpace(strings.SplitN(http.DetectContentType(contents), ";", 2)[0]))
	declared, _, _ = strings.Cut(strings.ToLower(strings.TrimSpace(declared)), ";")
	if declared == "" || declared == "application/octet-stream" {
		declared = detected
	}
	if !allowedMediaType(declared) {
		return "", ErrUnsupportedMedia
	}
	if declared != detected && !(detected == "text/plain" && isSafeTextSubtype(declared)) {
		return "", ErrUnsupportedMedia
	}
	return declared, nil
}

func allowedMediaType(mediaType string) bool {
	switch mediaType {
	case "application/pdf", "application/json", "application/zip",
		"image/gif", "image/jpeg", "image/png", "image/webp",
		"text/calendar", "text/csv", "text/plain":
		return true
	default:
		return false
	}
}

func isSafeTextSubtype(mediaType string) bool {
	return mediaType == "application/json" || mediaType == "text/calendar" || mediaType == "text/csv" || mediaType == "text/plain"
}

func SanitizedDisplayName(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = path.Base(value)
	if value == "." || value == "" || len(value) > 255 {
		return "", errors.New("attachment display name must be between 1 and 255 bytes")
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return "", errors.New("attachment display name contains a control character")
		}
	}
	if _, _, err := mime.ParseMediaType("attachment; filename=" + fmt.Sprintf("%q", value)); err != nil {
		return "", errors.New("attachment display name is invalid")
	}
	return value, nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader, limit int64) (int64, error) {
	buffer := make([]byte, 32<<10)
	limited := io.LimitReader(source, limit)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		read, readErr := limited.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != read {
				return total, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}
