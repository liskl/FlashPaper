// Package handler provides paste-related HTTP handlers.
// These implement the PrivateBin-compatible API for creating, reading,
// and deleting encrypted pastes.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/liskl/flashpaper/internal/model"
	"github.com/liskl/flashpaper/internal/util"
)

// createPaste handles paste creation requests.
// Request format (PrivateBin v2):
//
//	{
//	  "v": 2,
//	  "ct": "base64_ciphertext",
//	  "adata": [[iv, salt, iter, ks, ts, algo, mode, compression], formatter, opendiscussion, burnafterreading],
//	  "meta": {"expire": "1day"}
//	}
func (h *Handler) createPaste(w http.ResponseWriter, r *http.Request, req map[string]interface{}) {
	// Extract ciphertext
	ct, ok := req["ct"].(string)
	if !ok || ct == "" {
		h.jsonError(w, "No paste data provided", http.StatusBadRequest)
		return
	}

	// Check size limit
	if int64(len(ct)) > h.config.Main.SizeLimit {
		h.jsonError(w, "Paste exceeds size limit", http.StatusBadRequest)
		return
	}

	// Create paste model
	paste := model.NewPaste()
	paste.Data = ct

	// Get version (default to 2)
	if v, ok := req["v"].(float64); ok {
		paste.Version = int(v)
	}

	// Get adata (authenticated data containing encryption params and settings)
	if adata, ok := req["adata"]; ok {
		adataJSON, err := json.Marshal(adata)
		if err == nil {
			paste.AData = adataJSON
			// Parse adata to extract settings
			paste.ParseAData()
		}
	}

	// Get meta options
	if meta, ok := req["meta"].(map[string]interface{}); ok {
		// Expiration
		if expire, ok := meta["expire"].(string); ok {
			duration := h.config.GetExpireDuration(expire)
			paste.SetExpiration(duration)
		}
	}

	// Handle attachment if present
	if attachment, ok := req["attachment"].(string); ok {
		paste.Attachment = attachment
	}
	if attachmentName, ok := req["attachmentname"].(string); ok {
		paste.AttachmentName = attachmentName
	}

	// Validate paste
	if err := paste.Validate(); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate unique paste ID
	var pasteID string
	var err error
	for attempts := 0; attempts < 10; attempts++ {
		pasteID, err = util.GenerateID()
		if err != nil {
			h.jsonError(w, "Failed to generate paste ID", http.StatusInternalServerError)
			return
		}
		if !h.store.PasteExists(pasteID) {
			break
		}
	}

	// Store server salt for delete token
	paste.Meta.Salt = h.salt

	// Create paste in storage
	if err := h.store.CreatePaste(pasteID, paste); err != nil {
		if err == model.ErrPasteExists {
			h.jsonError(w, "Paste ID collision, please try again", http.StatusConflict)
			return
		}
		h.jsonError(w, "Failed to store paste", http.StatusInternalServerError)
		return
	}

	// Generate delete token
	deleteToken, err := util.GenerateDeleteToken(pasteID, h.salt)
	if err != nil {
		// Paste is created but we couldn't generate token - still return success
		deleteToken = ""
	}

	// Build response
	response := map[string]interface{}{
		"id":          pasteID,
		"url":         h.config.Main.BasePath + "/?" + pasteID,
		"deletetoken": deleteToken,
	}

	h.jsonSuccess(w, response)
}

// getPaste handles paste retrieval requests.
// Returns the encrypted paste data and metadata.
func (h *Handler) getPaste(w http.ResponseWriter, r *http.Request, pasteID string) {
	// Validate paste ID format
	if err := util.ValidateIDOrError(pasteID); err != nil {
		h.jsonError(w, "Invalid paste ID", http.StatusBadRequest)
		return
	}

	// Read paste from storage
	paste, err := h.store.ReadPaste(pasteID)
	if err != nil {
		switch err {
		case model.ErrPasteNotFound:
			h.jsonError(w, "Paste not found", http.StatusNotFound)
		case model.ErrPasteExpired:
			h.jsonError(w, "Paste has expired", http.StatusNotFound)
		default:
			h.jsonError(w, "Failed to read paste", http.StatusInternalServerError)
		}
		return
	}

	// Handle burn-after-reading
	// Note: Delete happens AFTER sending response so client gets the data
	shouldDelete := paste.IsBurnAfterReading()

	// Get comments if discussion is enabled
	var comments []*model.Comment
	if paste.HasDiscussion() {
		comments, _ = h.store.ReadComments(pasteID)
	}

	// Build response matching PrivateBin format
	response := map[string]interface{}{
		"id":   pasteID,
		"url":  h.config.Main.BasePath + "/?" + pasteID,
		"ct":   paste.Data,
		"adata": paste.AData,
		"v":    paste.Version,
		"meta": map[string]interface{}{
			"postdate":       paste.Meta.PostDate,
			"opendiscussion": paste.Meta.OpenDiscussion,
		},
	}

	// Add attachment if present
	if paste.Attachment != "" {
		response["attachment"] = paste.Attachment
	}
	if paste.AttachmentName != "" {
		response["attachmentname"] = paste.AttachmentName
	}

	// Add comments if any
	if len(comments) > 0 {
		commentData := make([]map[string]interface{}, len(comments))
		for i, c := range comments {
			commentData[i] = map[string]interface{}{
				"id":       c.ID,
				"parentid": c.ParentID,
				"pasteid":  c.PasteID,
				"data":     c.Data,
				"adata":    c.AData,
				"v":        c.Version,
				"meta": map[string]interface{}{
					"postdate": c.Meta.PostDate,
					"vizhash":  c.Vizhash,
				},
			}
		}
		response["comments"] = commentData
		response["comment_count"] = len(comments)
	}

	h.jsonSuccess(w, response)

	// Delete after response if burn-after-reading
	if shouldDelete {
		go func() {
			time.Sleep(100 * time.Millisecond) // Brief delay to ensure response is sent
			h.store.DeletePaste(pasteID)
		}()
	}
}

// deletePaste handles paste deletion requests.
// Requires the correct delete token for authentication.
func (h *Handler) deletePaste(w http.ResponseWriter, r *http.Request, req map[string]interface{}) {
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

	// Get delete token
	deleteToken, ok := req["deletetoken"].(string)
	if !ok || deleteToken == "" {
		h.jsonError(w, "No delete token provided", http.StatusBadRequest)
		return
	}

	// Validate delete token
	if !util.ValidateDeleteToken(deleteToken, pasteID, h.salt) {
		h.jsonError(w, "Invalid delete token", http.StatusForbidden)
		return
	}

	// Delete paste
	if err := h.store.DeletePaste(pasteID); err != nil {
		if err == model.ErrPasteNotFound {
			h.jsonError(w, "Paste not found", http.StatusNotFound)
			return
		}
		h.jsonError(w, "Failed to delete paste", http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"id": pasteID,
	})
}
