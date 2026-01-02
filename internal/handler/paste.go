// Package handler provides paste-related HTTP handlers.
// These implement the PrivateBin-compatible API for creating, reading,
// and deleting encrypted pastes.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

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
	ctx, span := h.tracer.Start(r.Context(), "Handler.createPaste")
	defer span.End()

	// Extract ciphertext
	ct, ok := req["ct"].(string)
	if !ok || ct == "" {
		span.SetStatus(codes.Error, "No paste data provided")
		h.jsonError(w, "No paste data provided", http.StatusBadRequest)
		return
	}

	// Check size limit
	if int64(len(ct)) > h.config.Main.SizeLimit {
		span.SetStatus(codes.Error, "Paste exceeds size limit")
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
		span.SetStatus(codes.Error, err.Error())
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate unique paste ID
	var pasteID string
	var err error
	for attempts := 0; attempts < 10; attempts++ {
		pasteID, err = util.GenerateID()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to generate paste ID")
			h.jsonError(w, "Failed to generate paste ID", http.StatusInternalServerError)
			return
		}
		if !h.store.PasteExists(ctx, pasteID) {
			break
		}
	}

	span.SetAttributes(attribute.String("flashpaper.paste.id", pasteID))

	// Store server salt for delete token
	paste.Meta.Salt = h.salt

	// Create paste in storage
	if err := h.store.CreatePaste(ctx, pasteID, paste); err != nil {
		span.RecordError(err)
		if err == model.ErrPasteExists {
			span.SetStatus(codes.Error, "Paste ID collision")
			h.jsonError(w, "Paste ID collision, please try again", http.StatusConflict)
			return
		}
		span.SetStatus(codes.Error, "Failed to store paste")
		h.jsonError(w, "Failed to store paste", http.StatusInternalServerError)
		return
	}

	// Generate delete token
	deleteToken, err := util.GenerateDeleteToken(pasteID, h.salt)
	if err != nil {
		// Paste is created but we couldn't generate token - still return success
		span.RecordError(err)
		deleteToken = ""
	}

	// Build response
	response := map[string]interface{}{
		"id":          pasteID,
		"url":         h.config.Main.BasePath + "/?" + pasteID,
		"deletetoken": deleteToken,
	}

	span.SetStatus(codes.Ok, "Paste created")
	h.jsonSuccess(w, response)
}

// getPaste handles paste retrieval requests.
// Returns the encrypted paste data and metadata.
func (h *Handler) getPaste(w http.ResponseWriter, r *http.Request, pasteID string) {
	ctx, span := h.tracer.Start(r.Context(), "Handler.getPaste",
		trace.WithAttributes(attribute.String("flashpaper.paste.id", pasteID)))
	defer span.End()

	// Validate paste ID format
	if err := util.ValidateIDOrError(pasteID); err != nil {
		span.SetStatus(codes.Error, "Invalid paste ID")
		h.jsonError(w, "Invalid paste ID", http.StatusBadRequest)
		return
	}

	// Read paste from storage
	paste, err := h.store.ReadPaste(ctx, pasteID)
	if err != nil {
		span.RecordError(err)
		switch err {
		case model.ErrPasteNotFound:
			span.SetStatus(codes.Error, "Paste not found")
			h.jsonError(w, "Paste not found", http.StatusNotFound)
		case model.ErrPasteExpired:
			span.SetStatus(codes.Error, "Paste expired")
			h.jsonError(w, "Paste has expired", http.StatusNotFound)
		default:
			span.SetStatus(codes.Error, "Failed to read paste")
			h.jsonError(w, "Failed to read paste", http.StatusInternalServerError)
		}
		return
	}

	// Handle burn-after-reading
	// Note: Delete happens AFTER sending response so client gets the data
	shouldDelete := paste.IsBurnAfterReading()
	span.SetAttributes(attribute.Bool("flashpaper.paste.burn_after_reading", shouldDelete))

	// Get comments if discussion is enabled
	var comments []*model.Comment
	if paste.HasDiscussion() {
		comments, _ = h.store.ReadComments(ctx, pasteID)
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
		span.SetAttributes(attribute.Int("flashpaper.paste.comment_count", len(comments)))
	}

	span.SetStatus(codes.Ok, "Paste retrieved")
	h.jsonSuccess(w, response)

	// Delete after response if burn-after-reading
	if shouldDelete {
		go func() {
			time.Sleep(100 * time.Millisecond) // Brief delay to ensure response is sent
			// Use background context since request context may be cancelled
			h.store.DeletePaste(context.Background(), pasteID)
		}()
	}
}

// deletePaste handles paste deletion requests.
// Requires the correct delete token for authentication.
func (h *Handler) deletePaste(w http.ResponseWriter, r *http.Request, req map[string]interface{}) {
	ctx, span := h.tracer.Start(r.Context(), "Handler.deletePaste")
	defer span.End()

	// Get paste ID
	pasteID, ok := req["pasteid"].(string)
	if !ok || pasteID == "" {
		span.SetStatus(codes.Error, "No paste ID provided")
		h.jsonError(w, "No paste ID provided", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("flashpaper.paste.id", pasteID))

	// Validate paste ID format
	if err := util.ValidateIDOrError(pasteID); err != nil {
		span.SetStatus(codes.Error, "Invalid paste ID")
		h.jsonError(w, "Invalid paste ID", http.StatusBadRequest)
		return
	}

	// Get delete token
	deleteToken, ok := req["deletetoken"].(string)
	if !ok || deleteToken == "" {
		span.SetStatus(codes.Error, "No delete token provided")
		h.jsonError(w, "No delete token provided", http.StatusBadRequest)
		return
	}

	// Validate delete token
	if !util.ValidateDeleteToken(deleteToken, pasteID, h.salt) {
		span.SetStatus(codes.Error, "Invalid delete token")
		h.jsonError(w, "Invalid delete token", http.StatusForbidden)
		return
	}

	// Delete paste
	if err := h.store.DeletePaste(ctx, pasteID); err != nil {
		span.RecordError(err)
		if err == model.ErrPasteNotFound {
			span.SetStatus(codes.Error, "Paste not found")
			h.jsonError(w, "Paste not found", http.StatusNotFound)
			return
		}
		span.SetStatus(codes.Error, "Failed to delete paste")
		h.jsonError(w, "Failed to delete paste", http.StatusInternalServerError)
		return
	}

	span.SetStatus(codes.Ok, "Paste deleted")
	h.jsonSuccess(w, map[string]interface{}{
		"id": pasteID,
	})
}
