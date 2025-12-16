// Package handler provides HTTP request handlers for FlashPaper.
// These handlers implement the PrivateBin-compatible API for creating,
// reading, and deleting encrypted pastes and comments.
package handler

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"

	flashpaper "github.com/liskl/flashpaper"
	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/storage"
	"github.com/liskl/flashpaper/internal/util"
)

// Handler contains dependencies for HTTP handlers.
type Handler struct {
	config   *config.Config
	store    storage.Storage
	salt     string             // Server salt for delete tokens
	template *template.Template // Parsed HTML template
	staticFS fs.FS              // Embedded static files (JS, CSS)
}

// New creates a new Handler with the given configuration and storage.
func New(cfg *config.Config, store storage.Storage) *Handler {
	h := &Handler{
		config: cfg,
		store:  store,
	}

	// Initialize or retrieve server salt
	h.initSalt()

	// Initialize embedded templates
	h.initTemplates()

	// Initialize static file serving
	h.initStaticFS()

	return h
}

// initTemplates parses the embedded HTML templates.
// Templates use Go's html/template for safe HTML rendering.
func (h *Handler) initTemplates() {
	templateFS, err := flashpaper.TemplateFS()
	if err != nil {
		// Log error but continue - will use fallback in serveUI
		return
	}

	h.template, _ = template.ParseFS(templateFS, "*.html")
}

// initStaticFS sets up the embedded static file system.
func (h *Handler) initStaticFS() {
	staticFS, err := flashpaper.StaticFS()
	if err != nil {
		return
	}
	h.staticFS = staticFS
}

// initSalt retrieves or generates the server salt.
// The salt is used for generating delete tokens and must persist across restarts.
func (h *Handler) initSalt() {
	// Try to get existing salt
	salt, err := h.store.GetValue(storage.NamespaceSalt, "server")
	if err == nil && salt != "" {
		h.salt = salt
		return
	}

	// Generate new salt
	salt, err = util.GenerateSalt()
	if err != nil {
		// Fall back to a less secure but functional salt
		salt = "flashpaper-fallback-salt-change-me"
	}

	// Store for future use
	_ = h.store.SetValue(storage.NamespaceSalt, "server", salt)
	h.salt = salt
}

// Routes returns the chi router with all API routes configured.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Health check endpoint
	r.Get("/health", h.healthCheck)

	// Main paste operations
	// PrivateBin uses query string for paste ID: /?pasteID
	r.Get("/", h.handleGet)
	r.Post("/", h.handlePost)
	r.Put("/", h.handlePost)    // PrivateBin also accepts PUT
	r.Delete("/", h.handleDelete)

	// Static files served from embedded filesystem
	// JS files: /js/flashpaper.js
	// CSS files: /css/style.css
	if h.staticFS != nil {
		fileServer := http.FileServer(http.FS(h.staticFS))
		r.Handle("/js/*", fileServer)
		r.Handle("/css/*", fileServer)
	}

	return r
}

// healthCheck returns a simple health status.
func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleGet handles GET requests - either serve the UI or return paste data.
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	// Check for paste ID in query string
	pasteID := r.URL.RawQuery
	if pasteID == "" {
		// No paste ID - serve the main UI
		h.serveUI(w, r)
		return
	}

	// Validate and clean paste ID (remove any extra parameters)
	if idx := indexOf(pasteID, '&'); idx != -1 {
		pasteID = pasteID[:idx]
	}

	// Check if this is a JSON API request
	if isJSONRequest(r) {
		h.getPaste(w, r, pasteID)
		return
	}

	// Serve UI with paste ID (client will fetch paste data)
	h.serveUI(w, r)
}

// handlePost handles POST/PUT requests - create paste or comment.
func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request) {
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/x-www-form-urlencoded" {
		h.jsonError(w, "Invalid content type", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Check if this is a delete request (has deletetoken)
	if _, hasDelete := req["deletetoken"]; hasDelete {
		h.deletePaste(w, r, req)
		return
	}

	// Check if this is a comment (has pasteid in body)
	if _, hasParent := req["pasteid"]; hasParent {
		h.createComment(w, r, req)
		return
	}

	// Otherwise, create new paste
	h.createPaste(w, r, req)
}

// handleDelete handles DELETE requests.
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.deletePaste(w, r, req)
}

// TemplateData contains data passed to the HTML template.
type TemplateData struct {
	Name        string // Application name
	BasePath    string // Base URL path
	Version     string // Application version
	Discussion  bool   // Whether discussions are globally enabled
	BurnEnabled bool   // Whether burn-after-reading is enabled
}

// serveUI serves the main HTML page using the embedded template.
func (h *Handler) serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Prepare template data
	data := TemplateData{
		Name:        h.config.Main.Name,
		BasePath:    h.config.Main.BasePath,
		Version:     "1.0.0",
		Discussion:  h.config.Main.Discussion,
		BurnEnabled: h.config.Main.BurnAfterReadingSelected,
	}

	// Try to execute template
	if h.template != nil {
		if err := h.template.ExecuteTemplate(w, "index.html", data); err == nil {
			return
		}
	}

	// Fallback to basic HTML if template fails
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>FlashPaper</title>
    <meta charset="utf-8">
</head>
<body>
    <h1>FlashPaper</h1>
    <p>Zero-knowledge encrypted pastebin</p>
    <p>Error loading template. Please check your installation.</p>
</body>
</html>`))
}

// jsonError sends a JSON error response matching PrivateBin format.
func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  1,
		"message": message,
	})
}

// jsonSuccess sends a JSON success response.
func (h *Handler) jsonSuccess(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	data["status"] = 0
	json.NewEncoder(w).Encode(data)
}

// isJSONRequest checks if the request expects JSON response.
func isJSONRequest(r *http.Request) bool {
	// Check X-Requested-With header (PrivateBin uses this)
	if r.Header.Get("X-Requested-With") == "JSONHttpRequest" {
		return true
	}

	// Check Accept header
	accept := r.Header.Get("Accept")
	return accept == "application/json" || accept == "application/json, text/javascript, */*; q=0.01"
}

// indexOf returns the index of the first occurrence of c in s, or -1.
func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
