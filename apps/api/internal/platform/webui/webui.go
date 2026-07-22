// Package webui serves the compiled SPA from the application binary.
package webui

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The Docker build replaces the checked-in placeholder with the Angular
// production output before compiling the Go binary.
//
//go:embed assets
var embeddedAssets embed.FS

var fingerprintPattern = regexp.MustCompile(`-[A-Za-z0-9_]{8,}(?:\.[A-Za-z0-9]+)+$`)

type representation struct {
	path     string
	encoding string
	etag     string
	size     int64
}

type asset struct {
	name            string
	representations []representation
}

// Handler serves immutable fingerprinted assets and an always-revalidated SPA
// entry point. Metadata and ETags are calculated once at startup; request-time
// handling streams directly from the embedded filesystem.
type Handler struct {
	root   fs.FS
	assets map[string]asset
}

// New creates a handler backed by the production assets embedded in this
// package. The source tree intentionally contains only a build placeholder, so
// a development binary returns 404 for SPA routes until the web build is copied
// into assets.
func New() (*Handler, error) {
	root, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil, err
	}
	return NewFromFS(root)
}

// NewFromFS builds a handler from an arbitrary filesystem. It is exported so
// the asset contract can be tested without writing generated files to disk.
func NewFromFS(root fs.FS) (*Handler, error) {
	if root == nil {
		return nil, errors.New("webui: nil filesystem")
	}

	h := &Handler{root: root, assets: make(map[string]asset)}
	var names []string
	err := fs.WalkDir(root, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.HasSuffix(name, ".br") || strings.HasSuffix(name, ".gz") {
			return nil
		}
		names = append(names, name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(names)
	for _, name := range names {
		item := asset{name: name}
		for _, candidate := range []struct {
			suffix   string
			encoding string
		}{
			{suffix: ".br", encoding: "br"},
			{suffix: ".gz", encoding: "gzip"},
			{suffix: "", encoding: "identity"},
		} {
			fileName := name + candidate.suffix
			info, statErr := fs.Stat(root, fileName)
			if statErr != nil {
				if errors.Is(statErr, fs.ErrNotExist) && candidate.suffix != "" {
					continue
				}
				return nil, statErr
			}
			etag, hashErr := hashFile(root, fileName)
			if hashErr != nil {
				return nil, hashErr
			}
			item.representations = append(item.representations, representation{
				path:     fileName,
				encoding: candidate.encoding,
				etag:     etag,
				size:     info.Size(),
			})
		}
		h.assets[name] = item
	}

	return h, nil
}

func hashFile(root fs.FS, name string) (string, error) {
	file, err := root.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()

	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return "", err
	}
	// A 128-bit prefix is compact while retaining ample collision resistance for
	// cache validators generated from trusted build artifacts.
	sum := digest.Sum(nil)
	return `"` + hex.EncodeToString(sum[:16]) + `"`, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	name, ok := safeAssetPath(r.URL)
	if !ok || name == "api" || strings.HasPrefix(name, "api/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	item, found := h.assets[name]
	if !found {
		if !isSPANavigation(r, name) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		item, found = h.assets["index.html"]
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	rep, found := chooseRepresentation(item.representations, r.Header.Get("Accept-Encoding"))
	if !found {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Set("ETag", rep.etag)
	w.Header().Set("Content-Type", contentType(item.name))
	if rep.encoding != "identity" {
		w.Header().Set("Content-Encoding", rep.encoding)
	}
	if item.name == "index.html" || !fingerprintPattern.MatchString(item.name) {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	if etagMatches(r.Header.Get("If-None-Match"), rep.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(rep.size, 10))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	file, err := h.root.Open(rep.path)
	if err != nil {
		w.Header().Del("Content-Length")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, file)
}

func safeAssetPath(u *url.URL) (string, bool) {
	requested := u.Path
	if u.RawPath != "" {
		decoded, err := url.PathUnescape(u.RawPath)
		if err != nil {
			return "", false
		}
		requested = decoded
	}
	if strings.ContainsRune(requested, '\x00') || strings.Contains(requested, `\`) {
		return "", false
	}
	for _, segment := range strings.Split(requested, "/") {
		if segment == ".." {
			return "", false
		}
	}

	cleaned := strings.TrimPrefix(path.Clean("/"+requested), "/")
	if cleaned == "." || cleaned == "" {
		cleaned = "index.html"
	}
	if !fs.ValidPath(cleaned) {
		return "", false
	}
	return cleaned, true
}

func isSPANavigation(r *http.Request, name string) bool {
	if path.Ext(name) != "" {
		return false
	}
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func chooseRepresentation(representations []representation, header string) (representation, bool) {
	qualities := parseAcceptEncoding(header)
	bestQuality := -1.0
	bestPriority := -1
	var best representation
	for _, rep := range representations {
		quality := encodingQuality(rep.encoding, header, qualities)
		priority := map[string]int{"identity": 0, "gzip": 1, "br": 2}[rep.encoding]
		if quality > bestQuality || (quality == bestQuality && priority > bestPriority) {
			best = rep
			bestQuality = quality
			bestPriority = priority
		}
	}
	return best, bestQuality > 0
}

func parseAcceptEncoding(header string) map[string]float64 {
	values := make(map[string]float64)
	for _, field := range strings.Split(strings.ToLower(header), ",") {
		parts := strings.Split(strings.TrimSpace(field), ";")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(keyValue) != 2 || keyValue[0] != "q" {
				continue
			}
			parsed, err := strconv.ParseFloat(keyValue[1], 64)
			if err != nil || parsed < 0 || parsed > 1 {
				quality = 0
			} else {
				quality = parsed
			}
		}
		values[parts[0]] = quality
	}
	return values
}

func encodingQuality(encoding, header string, qualities map[string]float64) float64 {
	if header == "" {
		if encoding == "identity" {
			return 1
		}
		return 0
	}
	if quality, ok := qualities[encoding]; ok {
		return quality
	}
	if quality, ok := qualities["*"]; ok {
		return quality
	}
	if encoding == "identity" {
		return 1
	}
	return 0
}

func etagMatches(header, current string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == current {
			return true
		}
	}
	return false
}

func contentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json", ".map":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".webmanifest":
		return "application/manifest+json"
	case ".wasm":
		return "application/wasm"
	case ".txt", ".xml":
		return "text/plain; charset=utf-8"
	default:
		if detected := mime.TypeByExtension(path.Ext(name)); detected != "" {
			return detected
		}
		return "application/octet-stream"
	}
}
