// Package handler provides comment-related HTTP handlers.
// Comments allow threaded discussions on pastes when enabled.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/liskl/flashpaper/internal/model"
	"github.com/liskl/flashpaper/internal/util"
)

// createComment handles comment creation requests.
// Request format:
//
//	{
//	  "pasteid": "parentPasteID",
//	  "parentid": "parentCommentID" (optional, for replies),
//	  "data": "encrypted_comment",
//	  "adata": [...],
//	  "v": 2
//	}
func (h *Handler) createComment(w http.ResponseWriter, r *http.Request, req map[string]interface{}) {
	// Check if discussions are enabled globally
	if !h.config.Main.Discussion {
		h.jsonError(w, "Discussions are disabled", http.StatusForbidden)
		return
	}

	// Get paste ID
	pasteID, ok := req["pasteid"].(string)
	if !ok || pasteID == "" {
		h.jsonError(w, "No paste ID provided", http.StatusBadRequest)
		return
	}

	// Validate paste ID format
	if err := util.ValidateIDOrError(pasteID); err != nil {
		h.jsonError(w, "Invalid paste ID", http.StatusBadRequest)
		return
	}

	// Check if paste exists and has discussion enabled
	paste, err := h.store.ReadPaste(pasteID)
	if err != nil {
		if err == model.ErrPasteNotFound || err == model.ErrPasteExpired {
			h.jsonError(w, "Paste not found", http.StatusNotFound)
			return
		}
		h.jsonError(w, "Failed to read paste", http.StatusInternalServerError)
		return
	}

	// Verify discussion is enabled for this paste
	if !paste.HasDiscussion() {
		h.jsonError(w, "Discussion is disabled for this paste", http.StatusForbidden)
		return
	}

	// Cannot comment on burn-after-reading pastes
	if paste.IsBurnAfterReading() {
		h.jsonError(w, "Cannot comment on burn-after-reading pastes", http.StatusForbidden)
		return
	}

	// Get comment data
	data, ok := req["data"].(string)
	if !ok || data == "" {
		h.jsonError(w, "No comment data provided", http.StatusBadRequest)
		return
	}

	// Get parent ID (empty for top-level comments)
	parentID, _ := req["parentid"].(string)
	if parentID == "" {
		// For top-level comments, parent is the paste itself
		parentID = pasteID
	} else {
		// Validate parent ID format
		if err := util.ValidateIDOrError(parentID); err != nil {
			h.jsonError(w, "Invalid parent ID", http.StatusBadRequest)
			return
		}
	}

	// Create comment model
	comment := model.NewComment(pasteID)
	comment.Data = data
	comment.ParentID = parentID

	// Get version
	if v, ok := req["v"].(float64); ok {
		comment.Version = int(v)
	}

	// Get adata
	if adata, ok := req["adata"]; ok {
		adataJSON, err := json.Marshal(adata)
		if err == nil {
			comment.AData = adataJSON
		}
	}

	// Generate vizhash from client IP for anonymous identification
	clientIP := getClientIP(r, h.config.Traffic.Header)
	vizhash, err := util.GenerateVizhash(clientIP, h.salt)
	if err == nil {
		comment.Vizhash = vizhash
	}

	// Validate comment
	if err := comment.Validate(); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate unique comment ID
	var commentID string
	for attempts := 0; attempts < 10; attempts++ {
		commentID, err = util.GenerateID()
		if err != nil {
			h.jsonError(w, "Failed to generate comment ID", http.StatusInternalServerError)
			return
		}
		if !h.store.CommentExists(pasteID, parentID, commentID) {
			break
		}
	}

	// Store comment
	if err := h.store.CreateComment(pasteID, parentID, commentID, comment); err != nil {
		if err == model.ErrCommentExists {
			h.jsonError(w, "Comment ID collision, please try again", http.StatusConflict)
			return
		}
		if err == model.ErrPasteNotFound {
			h.jsonError(w, "Paste not found", http.StatusNotFound)
			return
		}
		h.jsonError(w, "Failed to store comment", http.StatusInternalServerError)
		return
	}

	// Build response
	response := map[string]interface{}{
		"id":       commentID,
		"url":      h.config.Main.BasePath + "/?" + pasteID,
		"postdate": comment.Meta.PostDate,
	}

	h.jsonSuccess(w, response)
}

// getClientIP extracts the client IP address from the request.
// If a header is configured (for reverse proxy setups), it uses that.
func getClientIP(r *http.Request, header string) string {
	// Check configured header first (for reverse proxy)
	if header != "" {
		if ip := r.Header.Get(header); ip != "" {
			// Handle comma-separated list (X-Forwarded-For can have multiple IPs)
			if idx := indexOf(ip, ','); idx != -1 {
				ip = ip[:idx]
			}
			return trimSpace(ip)
		}
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr

	// Remove port number if present
	if idx := lastIndexOf(ip, ':'); idx != -1 {
		// Check if this is IPv6 (has more than one colon)
		if countByte(ip, ':') > 1 {
			// IPv6 - check for bracket notation [::1]:8080
			if bracketIdx := lastIndexOf(ip, ']'); bracketIdx != -1 && bracketIdx < idx {
				ip = ip[1:bracketIdx] // Remove brackets and port
			}
		} else {
			// IPv4 - simple split
			ip = ip[:idx]
		}
	}

	return ip
}

// Helper functions

func lastIndexOf(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func countByte(s string, c byte) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			count++
		}
	}
	return count
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// RateLimitMiddleware checks rate limiting for paste/comment creation.
// This is called before createPaste and createComment.
func (h *Handler) checkRateLimit(r *http.Request) error {
	// If rate limiting is disabled, allow
	if h.config.Traffic.Limit <= 0 {
		return nil
	}

	// Get client IP
	clientIP := getClientIP(r, h.config.Traffic.Header)

	// Check if IP is exempted
	for _, exempted := range h.config.Traffic.Exempted {
		if clientIP == exempted {
			return nil
		}
	}

	// Hash IP for storage
	ipHash := util.HashIP(clientIP, h.salt)

	// Get last access time
	lastAccessStr, err := h.store.GetValue("traffic", ipHash)
	if err != nil {
		return nil // Allow on error
	}

	if lastAccessStr != "" {
		var lastAccess int64
		if _, err := parseIntStr(lastAccessStr, &lastAccess); err == nil {
			elapsed := time.Now().Unix() - lastAccess
			if elapsed < int64(h.config.Traffic.Limit) {
				return model.ErrRateLimited
			}
		}
	}

	// Update last access time
	_ = h.store.SetValue("traffic", ipHash, formatInt(time.Now().Unix()))

	return nil
}

// parseIntStr parses an integer from a string.
func parseIntStr(s string, out *int64) (bool, error) {
	var result int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		result = result*10 + int64(c-'0')
	}
	*out = result
	return true, nil
}

// formatInt formats an integer as a string.
func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}

	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte(n%10) + '0'
		n /= 10
	}
	return string(buf[i:])
}
