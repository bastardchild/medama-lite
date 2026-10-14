package services

import (
	"hash/crc32"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-faster/errors"
	"github.com/medama-io/medama/client"
)

const (
	// ScriptCacheControl is the cache policy for the tracker scripts. Scripts
	// are revalidated every 6 hours and served stale for a day while
	// revalidating.
	ScriptCacheControl = "public, max-age=21600, stale-while-revalidate=86400"
	// AssetCacheControl is the cache policy for immutable assets.
	AssetCacheControl = "public, max-age=31536000, immutable"
)

// TrackerHandler serves the frozen tracker scripts from the embedded
// filesystem. It replaces the previous SPA handler: the React dashboard is no
// longer shipped, and pages are rendered server-side by the `ui` package.
type TrackerHandler struct {
	files     map[string][]byte
	fileETags map[string]string

	runtimeConfig *RuntimeConfig
}

// SetupTrackerHandler registers the tracker script routes on the mux.
//
// It handles:
//   - GET /script.js      -> the script variant selected in tenant settings
//   - GET /scripts/{file} -> any frozen tracker build (default, page-events, ...)
func SetupTrackerHandler(mux *http.ServeMux, runtimeConfig *RuntimeConfig) error {
	tracker, err := client.Tracker()
	if err != nil {
		return errors.Wrap(err, "failed to open embedded tracker scripts")
	}

	handler, err := NewTrackerHandler(tracker, runtimeConfig)
	if err != nil {
		return err
	}

	mux.Handle("/script.js", handler)
	mux.Handle("/scripts/", handler)

	return nil
}

// NewTrackerHandler builds a handler backed by the given tracker filesystem.
func NewTrackerHandler(tracker fs.FS, runtimeConfig *RuntimeConfig) (*TrackerHandler, error) {
	handler := &TrackerHandler{
		files:         make(map[string][]byte),
		fileETags:     make(map[string]string),
		runtimeConfig: runtimeConfig,
	}

	if err := handler.precompute(tracker); err != nil {
		return nil, errors.Wrap(err, "failed to precompute tracker assets")
	}

	return handler, nil
}

func (h *TrackerHandler) precompute(tracker fs.FS) error {
	return fs.WalkDir(tracker, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		content, err := readFile(tracker, name)
		if err != nil {
			return err
		}

		key := trackerKey(name)
		h.files[key] = content
		h.fileETags[key] = generateETag(content)

		return nil
	})
}

// trackerKey normalises an embedded path into the request path it is served at.
func trackerKey(name string) string {
	name = path.Clean("/" + strings.TrimPrefix(name, "./"))
	if !strings.HasPrefix(name, "/scripts/") {
		return "/scripts/" + strings.TrimPrefix(name, "/")
	}

	return name
}

func (h *TrackerHandler) serveFile(w http.ResponseWriter, r *http.Request, filePath string) {
	content, ok := h.files[filePath]
	if !ok {
		http.NotFound(w, r)

		return
	}

	etag := h.fileETags[filePath]
	w.Header().Set("ETag", etag)

	if isScriptFile(filePath) {
		w.Header().Set("Cache-Control", ScriptCacheControl)
	} else {
		w.Header().Set("Cache-Control", AssetCacheControl)
	}

	// Return 304 if the ETag matches.
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)

		return
	}

	if _, err := w.Write(content); err != nil {
		return
	}
}

func (h *TrackerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uPath := path.Clean(r.URL.Path)

	// /script.js is an alias that resolves to the script variant configured in
	// the tenant settings (e.g. /scripts/default.js or /scripts/page-events.js).
	if uPath == "/script.js" {
		h.serveFile(w, r, h.runtimeConfig.ScriptFileName)

		return
	}

	if strings.HasPrefix(uPath, "/scripts/") {
		h.serveFile(w, r, uPath)

		return
	}

	http.NotFound(w, r)
}

func isScriptFile(filePath string) bool {
	return strings.HasPrefix(filePath, "/scripts/") || filePath == "/script.js"
}

func readFile(filesystem fs.FS, file string) ([]byte, error) {
	f, err := filesystem.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

func generateETag(content []byte) string {
	crc := crc32.ChecksumIEEE(content)
	return strconv.FormatUint(uint64(crc), 16)
}
