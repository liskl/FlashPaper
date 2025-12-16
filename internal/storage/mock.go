// Package storage provides a mock implementation of the Storage interface.
// This mock is used for testing handlers and other components that depend
// on storage without needing a real database or filesystem.
package storage

import (
	"sync"
	"time"

	"github.com/liskl/flashpaper/internal/model"
)

// Mock implements the Storage interface for testing.
// It stores data in memory and can be configured to return errors.
type Mock struct {
	mu       sync.RWMutex
	pastes   map[string]*model.Paste
	comments map[string][]*model.Comment
	values   map[string]string

	// Error injection for testing error handling
	CreatePasteErr   error
	ReadPasteErr     error
	DeletePasteErr   error
	CreateCommentErr error
	ReadCommentsErr  error
	SetValueErr      error
	GetValueErr      error
}

// NewMock creates a new mock storage instance.
func NewMock() *Mock {
	return &Mock{
		pastes:   make(map[string]*model.Paste),
		comments: make(map[string][]*model.Comment),
		values:   make(map[string]string),
	}
}

// CreatePaste stores a paste in memory.
func (m *Mock) CreatePaste(id string, paste *model.Paste) error {
	if m.CreatePasteErr != nil {
		return m.CreatePasteErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pastes[id]; exists {
		return model.ErrPasteExists
	}

	// Make a copy to prevent external modifications
	stored := *paste
	stored.ID = id
	m.pastes[id] = &stored
	return nil
}

// ReadPaste retrieves a paste from memory.
func (m *Mock) ReadPaste(id string) (*model.Paste, error) {
	if m.ReadPasteErr != nil {
		return nil, m.ReadPasteErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	paste, exists := m.pastes[id]
	if !exists {
		return nil, model.ErrPasteNotFound
	}

	if paste.IsExpired() {
		delete(m.pastes, id)
		return nil, model.ErrPasteExpired
	}

	// Return a copy
	result := *paste
	return &result, nil
}

// DeletePaste removes a paste from memory.
func (m *Mock) DeletePaste(id string) error {
	if m.DeletePasteErr != nil {
		return m.DeletePasteErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pastes[id]; !exists {
		return model.ErrPasteNotFound
	}

	delete(m.pastes, id)
	delete(m.comments, id)
	return nil
}

// PasteExists checks if a paste exists in memory.
func (m *Mock) PasteExists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.pastes[id]
	return exists
}

// CreateComment stores a comment in memory.
func (m *Mock) CreateComment(pasteID, parentID, commentID string, comment *model.Comment) error {
	if m.CreateCommentErr != nil {
		return m.CreateCommentErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pastes[pasteID]; !exists {
		return model.ErrPasteNotFound
	}

	// Check for duplicate
	for _, c := range m.comments[pasteID] {
		if c.ID == commentID {
			return model.ErrCommentExists
		}
	}

	// Make a copy
	stored := *comment
	stored.ID = commentID
	stored.PasteID = pasteID
	stored.ParentID = parentID
	m.comments[pasteID] = append(m.comments[pasteID], &stored)
	return nil
}

// ReadComments retrieves all comments for a paste.
func (m *Mock) ReadComments(pasteID string) ([]*model.Comment, error) {
	if m.ReadCommentsErr != nil {
		return nil, m.ReadCommentsErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	comments := m.comments[pasteID]
	if len(comments) == 0 {
		return nil, nil
	}

	// Return copies
	result := make([]*model.Comment, len(comments))
	for i, c := range comments {
		copied := *c
		result[i] = &copied
	}
	return result, nil
}

// CommentExists checks if a comment exists.
func (m *Mock) CommentExists(pasteID, parentID, commentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, c := range m.comments[pasteID] {
		if c.ID == commentID {
			return true
		}
	}
	return false
}

// SetValue stores a key-value pair.
func (m *Mock) SetValue(namespace, key, value string) error {
	if m.SetValueErr != nil {
		return m.SetValueErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.values[namespace+"_"+key] = value
	return nil
}

// GetValue retrieves a stored value.
func (m *Mock) GetValue(namespace, key string) (string, error) {
	if m.GetValueErr != nil {
		return "", m.GetValueErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.values[namespace+"_"+key], nil
}

// GetExpiredPastes returns expired paste IDs.
func (m *Mock) GetExpiredPastes(batchSize int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expired []string
	now := time.Now().Unix()

	for id, paste := range m.pastes {
		if paste.Meta.ExpireDate > 0 && paste.Meta.ExpireDate < now {
			expired = append(expired, id)
			if len(expired) >= batchSize {
				break
			}
		}
	}

	return expired, nil
}

// Purge deletes expired pastes.
func (m *Mock) Purge(batchSize int) (int, error) {
	ids, err := m.GetExpiredPastes(batchSize)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, id := range ids {
		if err := m.DeletePaste(id); err == nil {
			count++
		}
	}

	return count, nil
}

// PurgeValues removes old entries (no-op for mock).
func (m *Mock) PurgeValues(namespace string, maxAge int64) error {
	return nil
}

// Close is a no-op for mock storage.
func (m *Mock) Close() error {
	return nil
}

// Reset clears all data from the mock storage.
// Useful for test setup/teardown.
func (m *Mock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pastes = make(map[string]*model.Paste)
	m.comments = make(map[string][]*model.Comment)
	m.values = make(map[string]string)
	m.CreatePasteErr = nil
	m.ReadPasteErr = nil
	m.DeletePasteErr = nil
	m.CreateCommentErr = nil
	m.ReadCommentsErr = nil
	m.SetValueErr = nil
	m.GetValueErr = nil
}

// GetPasteCount returns the number of pastes stored.
// Useful for assertions in tests.
func (m *Mock) GetPasteCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pastes)
}

// GetCommentCount returns the number of comments for a paste.
func (m *Mock) GetCommentCount(pasteID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.comments[pasteID])
}
