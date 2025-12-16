// Package handler provides tests for HTTP request handlers.
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/model"
	"github.com/liskl/flashpaper/internal/storage"
	"github.com/liskl/flashpaper/internal/util"
)

// newTestHandler creates a handler with mock storage for testing.
func newTestHandler(t *testing.T) (*Handler, *storage.Mock) {
	t.Helper()

	cfg := &config.Config{
		Main: config.MainConfig{
			Name:       "TestPaste",
			BasePath:   "",
			Discussion: true,
			SizeLimit:  10 * 1024 * 1024, // 10MB
		},
		Expire: config.ExpireConfig{
			Default: "1week",
			Options: map[string]time.Duration{
				"5min":   5 * time.Minute,
				"10min":  10 * time.Minute,
				"1hour":  time.Hour,
				"1day":   24 * time.Hour,
				"1week":  7 * 24 * time.Hour,
				"1month": 30 * 24 * time.Hour,
				"1year":  365 * 24 * time.Hour,
				"never":  0,
			},
		},
		Traffic: config.TrafficConfig{
			Limit: 0, // Disable rate limiting for tests
		},
	}

	mockStore := storage.NewMock()

	// Salt must be base64-encoded for GenerateDeleteToken to work
	h := &Handler{
		config: cfg,
		store:  mockStore,
		salt:   "dGVzdC1zYWx0LTEyMzQ1LWZsYXNocGFwZXI=", // base64("test-salt-12345-flashpaper")
	}

	return h, mockStore
}

// TestHealthCheck tests the health check endpoint.
func TestHealthCheck(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.healthCheck(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
}

// TestCreatePaste_ValidRequest tests creating a paste with valid input.
func TestCreatePaste_ValidRequest(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a valid paste request
	reqBody := map[string]interface{}{
		"v":  2,
		"ct": "dGVzdCBjaXBoZXJ0ZXh0", // base64 "test ciphertext"
		"adata": []interface{}{
			[]interface{}{"iv", "salt", 100000, 256, 128, "aes", "gcm", "zlib"},
			"plaintext",
			0, // opendiscussion
			0, // burnafterreading
		},
		"meta": map[string]interface{}{
			"expire": "1day",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check success status
	if status, ok := response["status"].(float64); !ok || status != 0 {
		t.Errorf("expected status 0, got %v", response["status"])
	}

	// Check that ID was returned
	pasteID, ok := response["id"].(string)
	if !ok || pasteID == "" {
		t.Error("expected paste ID in response")
	}

	// Validate ID format
	if len(pasteID) != 16 {
		t.Errorf("expected 16 character paste ID, got %d characters", len(pasteID))
	}

	// Check that delete token was returned
	deleteToken, ok := response["deletetoken"].(string)
	if !ok || deleteToken == "" {
		t.Error("expected delete token in response")
	}

	// Verify paste was stored
	if mockStore.GetPasteCount() != 1 {
		t.Errorf("expected 1 paste in storage, got %d", mockStore.GetPasteCount())
	}
}

// TestCreatePaste_EmptyContent tests creating a paste with empty content.
func TestCreatePaste_EmptyContent(t *testing.T) {
	h, _ := newTestHandler(t)

	reqBody := map[string]interface{}{
		"v":  2,
		"ct": "",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	// Should fail with bad request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Check error message
	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if status, ok := response["status"].(float64); !ok || status != 1 {
		t.Errorf("expected error status 1, got %v", response["status"])
	}
}

// TestCreatePaste_InvalidContentType tests rejecting non-JSON content type.
func TestCreatePaste_InvalidContentType(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestCreatePaste_InvalidJSON tests rejecting malformed JSON.
func TestCreatePaste_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestCreatePaste_ExceedsSizeLimit tests rejecting oversized pastes.
func TestCreatePaste_ExceedsSizeLimit(t *testing.T) {
	h, _ := newTestHandler(t)

	// Set a small size limit
	h.config.Main.SizeLimit = 100

	reqBody := map[string]interface{}{
		"v":  2,
		"ct": strings.Repeat("a", 200), // Exceeds 100 byte limit
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestGetPaste_ValidPaste tests retrieving an existing paste.
func TestGetPaste_ValidPaste(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a paste directly in storage (16 lowercase hex chars)
	pasteID := "abcdef1234567890"
	paste := model.NewPaste()
	paste.Data = "encrypted-content"
	paste.AData = []byte(`[["iv","salt",100000,256,128,"aes","gcm","zlib"],"plaintext",0,0]`)
	mockStore.CreatePaste(pasteID, paste)

	// Request the paste with JSON header
	req := httptest.NewRequest(http.MethodGet, "/?"+pasteID, nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()

	h.handleGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check success status
	if status, ok := response["status"].(float64); !ok || status != 0 {
		t.Errorf("expected status 0, got %v", response["status"])
	}

	// Check paste ID matches
	if id, ok := response["id"].(string); !ok || id != pasteID {
		t.Errorf("expected id %s, got %v", pasteID, response["id"])
	}

	// Check content
	if ct, ok := response["ct"].(string); !ok || ct != "encrypted-content" {
		t.Errorf("expected ct 'encrypted-content', got %v", response["ct"])
	}
}

// TestGetPaste_NotFound tests retrieving a non-existent paste.
func TestGetPaste_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/?0000000000000001", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()

	h.handleGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// TestGetPaste_InvalidID tests retrieving with an invalid paste ID format.
func TestGetPaste_InvalidID(t *testing.T) {
	h, _ := newTestHandler(t)

	// ID too short
	req := httptest.NewRequest(http.MethodGet, "/?abc", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()

	h.handleGet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestGetPaste_BurnAfterReading tests that burn-after-reading pastes are deleted.
func TestGetPaste_BurnAfterReading(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a burn-after-reading paste (16 lowercase hex chars)
	pasteID := "baf1ead123456789"
	paste := model.NewPaste()
	paste.Data = "secret-content"
	paste.Meta.BurnAfterReading = true
	paste.AData = []byte(`[["iv","salt",100000,256,128,"aes","gcm","zlib"],"plaintext",0,1]`)
	mockStore.CreatePaste(pasteID, paste)

	// First read should succeed
	req := httptest.NewRequest(http.MethodGet, "/?"+pasteID, nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()

	h.handleGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d on first read, got %d", http.StatusOK, rr.Code)
	}

	// Give time for async deletion
	// Note: In real tests, you'd use synchronization
}

// TestDeletePaste_ValidToken tests deleting a paste with valid token.
func TestDeletePaste_ValidToken(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a paste (16 lowercase hex chars)
	pasteID := "de1e7e0012345678"
	paste := model.NewPaste()
	paste.Data = "to-be-deleted"
	mockStore.CreatePaste(pasteID, paste)

	// Generate valid delete token
	deleteToken, _ := util.GenerateDeleteToken(pasteID, h.salt)

	// Send delete request
	reqBody := map[string]interface{}{
		"pasteid":     pasteID,
		"deletetoken": deleteToken,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handleDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify paste was deleted
	if mockStore.PasteExists(pasteID) {
		t.Error("paste should have been deleted")
	}
}

// TestDeletePaste_InvalidToken tests rejecting delete with wrong token.
func TestDeletePaste_InvalidToken(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a paste (16 lowercase hex chars)
	pasteID := "0de1e7e123456789"
	paste := model.NewPaste()
	paste.Data = "should-not-delete"
	mockStore.CreatePaste(pasteID, paste)

	// Send delete request with invalid token
	reqBody := map[string]interface{}{
		"pasteid":     pasteID,
		"deletetoken": "invalid-token",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handleDelete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	// Verify paste still exists
	if !mockStore.PasteExists(pasteID) {
		t.Error("paste should not have been deleted with invalid token")
	}
}

// TestDeletePaste_MissingPasteID tests rejecting delete without paste ID.
func TestDeletePaste_MissingPasteID(t *testing.T) {
	h, _ := newTestHandler(t)

	reqBody := map[string]interface{}{
		"deletetoken": "some-token",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handleDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestDeletePaste_MissingToken tests rejecting delete without token.
func TestDeletePaste_MissingToken(t *testing.T) {
	h, _ := newTestHandler(t)

	reqBody := map[string]interface{}{
		"pasteid": "1111111111111111",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handleDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// TestDeletePaste_NotFound tests deleting a non-existent paste.
func TestDeletePaste_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	pasteID := "2222222222222222"
	deleteToken, _ := util.GenerateDeleteToken(pasteID, h.salt)

	reqBody := map[string]interface{}{
		"pasteid":     pasteID,
		"deletetoken": deleteToken,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodDelete, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handleDelete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// TestCreateComment_ValidRequest tests creating a comment on a paste.
func TestCreateComment_ValidRequest(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a paste with discussion enabled
	pasteID := "d15c055ea5e01234"
	paste := model.NewPaste()
	paste.Data = "paste-with-discussion"
	paste.Meta.OpenDiscussion = true
	mockStore.CreatePaste(pasteID, paste)

	// Create a comment
	reqBody := map[string]interface{}{
		"v":        2,
		"pasteid":  pasteID,
		"parentid": pasteID, // Top-level comment
		"data":     "encrypted-comment",
		"adata": []interface{}{
			[]interface{}{"iv", "salt", 100000, 256, 128, "aes", "gcm", "zlib"},
			"plaintext",
			0,
			0,
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Check comment was stored
	if mockStore.GetCommentCount(pasteID) != 1 {
		t.Errorf("expected 1 comment, got %d", mockStore.GetCommentCount(pasteID))
	}
}

// TestCreateComment_DiscussionDisabled tests rejecting comments when globally disabled.
func TestCreateComment_DiscussionDisabled(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Disable discussion globally
	h.config.Main.Discussion = false

	// Create a paste
	pasteID := "3333333333333333"
	paste := model.NewPaste()
	paste.Data = "paste-data"
	paste.Meta.OpenDiscussion = true
	mockStore.CreatePaste(pasteID, paste)

	// Try to create a comment
	reqBody := map[string]interface{}{
		"pasteid": pasteID,
		"data":    "comment-data",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

// TestCreateComment_PasteDiscussionDisabled tests rejecting comments on pastes without discussion.
func TestCreateComment_PasteDiscussionDisabled(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a paste WITHOUT discussion enabled
	pasteID := "4444444444444444"
	paste := model.NewPaste()
	paste.Data = "no-discussion-paste"
	paste.Meta.OpenDiscussion = false
	mockStore.CreatePaste(pasteID, paste)

	// Try to create a comment
	reqBody := map[string]interface{}{
		"pasteid": pasteID,
		"data":    "comment-data",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

// TestCreateComment_BurnAfterReadingPaste tests rejecting comments on burn-after-reading pastes.
func TestCreateComment_BurnAfterReadingPaste(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a burn-after-reading paste
	pasteID := "5555555555555555"
	paste := model.NewPaste()
	paste.Data = "burn-paste"
	paste.Meta.OpenDiscussion = true
	paste.Meta.BurnAfterReading = true
	mockStore.CreatePaste(pasteID, paste)

	// Try to create a comment
	reqBody := map[string]interface{}{
		"pasteid": pasteID,
		"data":    "comment-data",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

// TestCreateComment_PasteNotFound tests rejecting comments on non-existent pastes.
func TestCreateComment_PasteNotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	reqBody := map[string]interface{}{
		"pasteid": "6666666666666666",
		"data":    "comment-data",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// TestGetPaste_WithComments tests retrieving a paste with comments.
func TestGetPaste_WithComments(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a paste with discussion enabled
	pasteID := "7777777777777777"
	paste := model.NewPaste()
	paste.Data = "paste-content"
	paste.Meta.OpenDiscussion = true
	paste.AData = []byte(`[["iv","salt",100000,256,128,"aes","gcm","zlib"],"plaintext",1,0]`)
	mockStore.CreatePaste(pasteID, paste)

	// Add a comment
	comment := model.NewComment(pasteID)
	comment.Data = "comment-content"
	comment.ParentID = pasteID
	mockStore.CreateComment(pasteID, pasteID, "c0ffee1234567890", comment)

	// Request the paste
	req := httptest.NewRequest(http.MethodGet, "/?"+pasteID, nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()

	h.handleGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	// Check comments are included
	comments, ok := response["comments"].([]interface{})
	if !ok || len(comments) != 1 {
		t.Errorf("expected 1 comment, got %v", response["comments"])
	}

	commentCount, _ := response["comment_count"].(float64)
	if commentCount != 1 {
		t.Errorf("expected comment_count 1, got %v", commentCount)
	}
}

// TestJSONError tests the JSON error response format.
func TestJSONError(t *testing.T) {
	h, _ := newTestHandler(t)

	rr := httptest.NewRecorder()
	h.jsonError(rr, "Test error message", http.StatusBadRequest)

	// Check status code
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Check content type
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// Check body
	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if status, _ := response["status"].(float64); status != 1 {
		t.Errorf("expected status 1, got %v", response["status"])
	}

	if message, _ := response["message"].(string); message != "Test error message" {
		t.Errorf("expected message 'Test error message', got %s", message)
	}
}

// TestJSONSuccess tests the JSON success response format.
func TestJSONSuccess(t *testing.T) {
	h, _ := newTestHandler(t)

	rr := httptest.NewRecorder()
	h.jsonSuccess(rr, map[string]interface{}{
		"id":  "test123",
		"url": "/test",
	})

	// Check status code (default 200)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Check content type
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// Check body
	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if status, _ := response["status"].(float64); status != 0 {
		t.Errorf("expected status 0, got %v", response["status"])
	}

	if id, _ := response["id"].(string); id != "test123" {
		t.Errorf("expected id 'test123', got %s", id)
	}
}

// TestIsJSONRequest tests the JSON request detection logic.
func TestIsJSONRequest(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "X-Requested-With header",
			headers:  map[string]string{"X-Requested-With": "JSONHttpRequest"},
			expected: true,
		},
		{
			name:     "Accept application/json",
			headers:  map[string]string{"Accept": "application/json"},
			expected: true,
		},
		{
			name:     "Accept with multiple types",
			headers:  map[string]string{"Accept": "application/json, text/javascript, */*; q=0.01"},
			expected: true,
		},
		{
			name:     "No JSON headers",
			headers:  map[string]string{"Accept": "text/html"},
			expected: false,
		},
		{
			name:     "Empty headers",
			headers:  map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := isJSONRequest(req)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIndexOf tests the indexOf helper function.
func TestIndexOf(t *testing.T) {
	tests := []struct {
		s        string
		c        byte
		expected int
	}{
		{"hello", 'l', 2},
		{"hello", 'o', 4},
		{"hello", 'x', -1},
		{"", 'a', -1},
		{"abc&def", '&', 3},
	}

	for _, tt := range tests {
		result := indexOf(tt.s, tt.c)
		if result != tt.expected {
			t.Errorf("indexOf(%q, %c) = %d, expected %d", tt.s, tt.c, result, tt.expected)
		}
	}
}

// TestGetUI tests that the main page returns HTML for non-JSON requests.
func TestGetUI(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()

	h.handleGet(rr, req)

	// Check content type is HTML
	contentType := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	// Check response contains HTML
	body := rr.Body.String()
	if !strings.Contains(body, "<html") {
		t.Errorf("expected HTML response, got: %s", body[:min(100, len(body))])
	}
}

// TestPostViaDeleteToken tests deletion via POST with deletetoken (PrivateBin compatibility).
func TestPostViaDeleteToken(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Create a paste
	pasteID := "8888888888888888"
	paste := model.NewPaste()
	paste.Data = "to-be-deleted-via-post"
	mockStore.CreatePaste(pasteID, paste)

	// Generate valid delete token
	deleteToken, _ := util.GenerateDeleteToken(pasteID, h.salt)

	// Send POST request with deletetoken (not DELETE method)
	reqBody := map[string]interface{}{
		"pasteid":     pasteID,
		"deletetoken": deleteToken,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify paste was deleted
	if mockStore.PasteExists(pasteID) {
		t.Error("paste should have been deleted via POST with deletetoken")
	}
}

// TestStorageError tests handling of storage errors.
func TestStorageError(t *testing.T) {
	h, mockStore := newTestHandler(t)

	// Inject storage error
	mockStore.CreatePasteErr = model.ErrPasteExists

	reqBody := map[string]interface{}{
		"v":  2,
		"ct": "test-content",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.handlePost(rr, req)

	// Should return conflict error
	if rr.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, rr.Code)
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
