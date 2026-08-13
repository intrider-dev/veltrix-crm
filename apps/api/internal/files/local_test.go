package files

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestLocalStoreRoundTrip(t *testing.T) {
	t.Parallel()
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	contents := []byte("first,last\nAda,Lovelace\n")
	blob, err := store.Put(context.Background(), "workspace/object/blob", "text/csv", bytes.NewReader(contents), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if blob.SizeBytes != int64(len(contents)) || blob.ChecksumSHA256 != sha256.Sum256(contents) || blob.MediaType != "text/csv" {
		t.Fatalf("unexpected blob metadata: %#v", blob)
	}
	reader, err := store.Open(context.Background(), "workspace/object/blob")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(got, contents) {
		t.Fatalf("read = %q, %v", got, err)
	}
}

func TestLocalStoreRejectsTraversalAndOversize(t *testing.T) {
	t.Parallel()
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, key := range []string{"../escape", "/absolute", "safe/../escape", `safe\\escape`, "safe//escape"} {
		if _, err := store.Put(context.Background(), key, "text/plain", strings.NewReader("data"), 16); !errors.Is(err, ErrInvalidStorageKey) {
			t.Fatalf("key %q returned %v, want ErrInvalidStorageKey", key, err)
		}
	}
	if _, err := store.Put(context.Background(), "safe/large", "text/plain", strings.NewReader("too large"), 3); !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("oversize returned %v, want ErrObjectTooLarge", err)
	}
}

func TestLocalStoreRejectsActiveContent(t *testing.T) {
	t.Parallel()
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.Put(
		context.Background(), "safe/page", "text/html",
		strings.NewReader("<!doctype html><script>alert(1)</script>"), 1024,
	); !errors.Is(err, ErrUnsupportedMedia) {
		t.Fatalf("active content returned %v, want ErrUnsupportedMedia", err)
	}
}

func TestSanitizedDisplayName(t *testing.T) {
	t.Parallel()
	name, err := SanitizedDisplayName(`C:\\fakepath\\report.csv`)
	if err != nil || name != "report.csv" {
		t.Fatalf("SanitizedDisplayName() = %q, %v", name, err)
	}
	if _, err := SanitizedDisplayName("report\r\n.csv"); err == nil {
		t.Fatal("control characters must be rejected")
	}
}
