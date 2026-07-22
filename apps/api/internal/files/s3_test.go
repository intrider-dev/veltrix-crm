package files

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestS3StoreLifecycleUsesSignedPathStyleRequests(t *testing.T) {
	t.Parallel()
	object := []byte("name,email\nDemo User,user@example.invalid\n")
	var stored []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/crm-files/workspace/attachment/blob" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test-access/") {
			t.Errorf("missing SigV4 authorization: %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("X-Amz-Content-Sha256") == "" || request.Header.Get("X-Amz-Date") != "20260721T120000Z" {
			t.Error("missing deterministic signing headers")
		}
		switch request.Method {
		case http.MethodPut:
			stored, _ = io.ReadAll(request.Body)
			writer.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if stored == nil {
				http.NotFound(writer, request)
				return
			}
			_, _ = writer.Write(stored)
		case http.MethodDelete:
			stored = nil
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	store, err := NewS3Store(S3Config{
		Endpoint: server.URL, Region: "us-east-1", Bucket: "crm-files",
		AccessKey: "test-access", SecretKey: "test-secret", AllowInsecure: true,
		HTTPClient: server.Client(), Now: func() time.Time { return time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	blob, err := store.Put(context.Background(), "workspace/attachment/blob", "text/csv", strings.NewReader(string(object)), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if blob.SizeBytes != int64(len(object)) || blob.MediaType != "text/csv" {
		t.Fatalf("unexpected blob metadata: %+v", blob)
	}
	reader, err := store.Open(context.Background(), "workspace/attachment/blob")
	if err != nil {
		t.Fatal(err)
	}
	loaded, _ := io.ReadAll(reader)
	_ = reader.Close()
	if string(loaded) != string(object) {
		t.Fatalf("loaded object mismatch: %q", loaded)
	}
	if err := store.Delete(context.Background(), "workspace/attachment/blob"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(context.Background(), "workspace/attachment/blob"); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestS3StoreRejectsUnsafeConfigurationAndOversizedObject(t *testing.T) {
	t.Parallel()
	if _, err := NewS3Store(S3Config{Endpoint: "http://minio:9000", Region: "us-east-1", Bucket: "files", AccessKey: "a", SecretKey: "b"}); err == nil {
		t.Fatal("expected insecure endpoint rejection")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	store, err := NewS3Store(S3Config{
		Endpoint: server.URL, Region: "us-east-1", Bucket: "files", AccessKey: "a", SecretKey: "b",
		AllowInsecure: true, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "safe/blob", "text/plain", strings.NewReader("too large"), 3); err != ErrObjectTooLarge {
		t.Fatalf("error = %v, want ErrObjectTooLarge", err)
	}
	if _, err := store.Open(context.Background(), "../escape"); err != ErrInvalidStorageKey {
		t.Fatalf("error = %v, want ErrInvalidStorageKey", err)
	}
}
