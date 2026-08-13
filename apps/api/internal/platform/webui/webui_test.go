package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testHandler(t *testing.T) *Handler {
	t.Helper()
	handler, err := NewFromFS(fstest.MapFS{
		"index.html":               {Data: []byte("<main>app</main>")},
		"main-ABCDEF12.js":         {Data: []byte("identity")},
		"main-ABCDEF12.js.gz":      {Data: []byte("gzip")},
		"main-ABCDEF12.js.br":      {Data: []byte("brotli")},
		"i18n/en/common.json":      {Data: []byte(`{"common.ok":"OK"}`)},
		"assets/not-finger.css":    {Data: []byte("body{}")},
		"assets/icon-ABCDEF12.svg": {Data: []byte("<svg></svg>")},
	})
	if err != nil {
		t.Fatalf("NewFromFS() error = %v", err)
	}
	return handler
}

func TestServesBrotliThenGzipByQuality(t *testing.T) {
	handler := testHandler(t)

	request := httptest.NewRequest(http.MethodGet, "/main-ABCDEF12.js", nil)
	request.Header.Set("Accept-Encoding", "gzip;q=0.6, br")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if got := recorder.Body.String(); got != "brotli" {
		t.Fatalf("body = %q, want brotli", got)
	}

	request = httptest.NewRequest(http.MethodGet, "/main-ABCDEF12.js", nil)
	request.Header.Set("Accept-Encoding", "br;q=0, gzip")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
}

func TestIdentityAndContentType(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/i18n/en/common.json", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestFingerprintCachingAndConditionalRequest(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/main-ABCDEF12.js", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	etag := recorder.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag is empty")
	}

	request = httptest.NewRequest(http.MethodGet, "/main-ABCDEF12.js", nil)
	request.Header.Set("If-None-Match", "W/"+etag)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want %d", recorder.Code, http.StatusNotModified)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("conditional body length = %d, want 0", recorder.Body.Len())
	}
}

func TestSPAFallbackAndIndexRevalidation(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/contacts/019abc", nil)
	request.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "<main>app</main>" {
		t.Fatalf("body = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestMissingAssetDoesNotFallBack(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/missing.js", nil)
	request.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestRejectsTraversalAndAPIFallback(t *testing.T) {
	handler := testHandler(t)
	for _, target := range []string{"/..%2Findex.html", "/assets%5Cindex.html", "/api/v1/not-real"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.Header.Set("Accept", "text/html")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("target %q status = %d, want %d", target, recorder.Code, http.StatusNotFound)
		}
	}
}

func TestHeadAndUnsupportedMethod(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodHead, "/main-ABCDEF12.js", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
		t.Fatalf("HEAD status/body = %d/%d", recorder.Code, recorder.Body.Len())
	}
	if recorder.Header().Get("Content-Length") == "" {
		t.Fatal("HEAD Content-Length is empty")
	}

	request = httptest.NewRequest(http.MethodPost, "/", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", recorder.Code)
	}
	if recorder.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", recorder.Header().Get("Allow"))
	}
}

func TestNoAcceptableRepresentation(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/main-ABCDEF12.js", nil)
	request.Header.Set("Accept-Encoding", "identity;q=0, br;q=0, gzip;q=0")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotAcceptable)
	}
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil || len(body) != 0 {
		t.Fatalf("body = %q, error = %v", body, err)
	}
}
