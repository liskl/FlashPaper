// Package storage provides the filesystem implementation of the Storage interface.
// This implementation stores pastes as files on disk, using a nested directory
// structure to avoid performance issues with too many files in a single directory.
//
// Directory structure:
//   data/
//     f4/
//       68/
//         f468483c313401e8           <- paste file
//         f468483c313401e8.discussion/
//           comment1.parent1.json    <- comment file
//
// Each paste file contains JSON with the encrypted data and metadata.
// Comments are stored in a .discussion subdirectory.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/model"
)

// Filesystem implements the Storage interface using the local filesystem.
type Filesystem struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFilesystem creates a new filesystem storage backend.
func NewFilesystem(cfg *config.Config) (*Filesystem, error) {
	baseDir := cfg.Model.Dir
	if baseDir == "" {
		baseDir = "data"
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	// Create config directory for key-value storage
	configDir := filepath.Join(baseDir, "_config")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}

	return &Filesystem{
		baseDir: baseDir,
	}, nil
}

// pastePath returns the file path for a paste.
// Uses nested directories: data/f4/68/f468483c313401e8
func (f *Filesystem) pastePath(id string) string {
	if len(id) < 4 {
		return filepath.Join(f.baseDir, id)
	}
	return filepath.Join(f.baseDir, id[:2], id[2:4], id)
}

// discussionDir returns the directory path for a paste's comments.
func (f *Filesystem) discussionDir(id string) string {
	return f.pastePath(id) + ".discussion"
}

// commentPath returns the file path for a comment.
func (f *Filesystem) commentPath(pasteID, parentID, commentID string) string {
	filename := fmt.Sprintf("%s.%s.json", commentID, parentID)
	return filepath.Join(f.discussionDir(pasteID), filename)
}

// configPath returns the path for a config value.
func (f *Filesystem) configPath(namespace, key string) string {
	return filepath.Join(f.baseDir, "_config", namespace+"_"+key)
}

// pasteStorageData is the structure stored in paste files.
type pasteStorageData struct {
	Data           string          `json:"data"`
	AttachmentName string          `json:"attachmentname,omitempty"`
	Attachment     string          `json:"attachment,omitempty"`
	AData          json.RawMessage `json:"adata,omitempty"`
	Version        int             `json:"v"`
	Meta           model.PasteMeta `json:"meta"`
}

// CreatePaste stores a new paste on the filesystem.
func (f *Filesystem) CreatePaste(id string, paste *model.Paste) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.pastePath(id)

	// Check if paste already exists
	if _, err := os.Stat(path); err == nil {
		return model.ErrPasteExists
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating paste directory: %w", err)
	}

	// Prepare storage data
	storageData := pasteStorageData{
		Data:           paste.Data,
		AttachmentName: paste.AttachmentName,
		Attachment:     paste.Attachment,
		AData:          paste.AData,
		Version:        paste.Version,
		Meta:           paste.Meta,
	}

	data, err := json.Marshal(storageData)
	if err != nil {
		return fmt.Errorf("serializing paste: %w", err)
	}

	// Write atomically using temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0640); err != nil {
		return fmt.Errorf("writing paste file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming paste file: %w", err)
	}

	return nil
}

// ReadPaste retrieves a paste from the filesystem.
func (f *Filesystem) ReadPaste(id string) (*model.Paste, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.pastePath(id)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, model.ErrPasteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading paste file: %w", err)
	}

	var storageData pasteStorageData
	if err := json.Unmarshal(data, &storageData); err != nil {
		return nil, fmt.Errorf("deserializing paste: %w", err)
	}

	paste := &model.Paste{
		ID:             id,
		Data:           storageData.Data,
		AttachmentName: storageData.AttachmentName,
		Attachment:     storageData.Attachment,
		AData:          storageData.AData,
		Version:        storageData.Version,
		Meta:           storageData.Meta,
	}

	// Check if expired
	if paste.IsExpired() {
		// Delete the expired paste
		f.mu.RUnlock()
		f.DeletePaste(id)
		f.mu.RLock()
		return nil, model.ErrPasteExpired
	}

	return paste, nil
}

// DeletePaste removes a paste and its comments from the filesystem.
func (f *Filesystem) DeletePaste(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.pastePath(id)
	discussionPath := f.discussionDir(id)

	// Check if paste exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return model.ErrPasteNotFound
	}

	// Delete discussion directory and contents
	if err := os.RemoveAll(discussionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting discussion directory: %w", err)
	}

	// Delete paste file
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("deleting paste file: %w", err)
	}

	return nil
}

// PasteExists checks if a paste exists on the filesystem.
func (f *Filesystem) PasteExists(id string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.pastePath(id)
	_, err := os.Stat(path)
	return err == nil
}

// commentStorageData is the structure stored in comment files.
type commentStorageData struct {
	Data     string          `json:"data"`
	AData    json.RawMessage `json:"adata,omitempty"`
	Version  int             `json:"v"`
	Vizhash  string          `json:"vizhash,omitempty"`
	PostDate int64           `json:"postdate"`
}

// CreateComment stores a new comment on the filesystem.
func (f *Filesystem) CreateComment(pasteID, parentID, commentID string, comment *model.Comment) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Verify paste exists
	pastePath := f.pastePath(pasteID)
	if _, err := os.Stat(pastePath); os.IsNotExist(err) {
		return model.ErrPasteNotFound
	}

	// Create discussion directory if needed
	discussionPath := f.discussionDir(pasteID)
	if err := os.MkdirAll(discussionPath, 0700); err != nil {
		return fmt.Errorf("creating discussion directory: %w", err)
	}

	commentPath := f.commentPath(pasteID, parentID, commentID)

	// Check if comment already exists
	if _, err := os.Stat(commentPath); err == nil {
		return model.ErrCommentExists
	}

	// Prepare storage data
	storageData := commentStorageData{
		Data:     comment.Data,
		AData:    comment.AData,
		Version:  comment.Version,
		Vizhash:  comment.Vizhash,
		PostDate: comment.Meta.PostDate,
	}

	data, err := json.Marshal(storageData)
	if err != nil {
		return fmt.Errorf("serializing comment: %w", err)
	}

	// Write atomically
	tmpPath := commentPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0640); err != nil {
		return fmt.Errorf("writing comment file: %w", err)
	}

	if err := os.Rename(tmpPath, commentPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming comment file: %w", err)
	}

	return nil
}

// ReadComments retrieves all comments for a paste.
func (f *Filesystem) ReadComments(pasteID string) ([]*model.Comment, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	discussionPath := f.discussionDir(pasteID)

	entries, err := os.ReadDir(discussionPath)
	if os.IsNotExist(err) {
		return nil, nil // No comments
	}
	if err != nil {
		return nil, fmt.Errorf("reading discussion directory: %w", err)
	}

	var comments []*model.Comment
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Parse filename: commentID.parentID.json
		name := strings.TrimSuffix(entry.Name(), ".json")
		parts := strings.Split(name, ".")
		if len(parts) != 2 {
			continue
		}
		commentID := parts[0]
		parentID := parts[1]

		// Read comment file
		data, err := os.ReadFile(filepath.Join(discussionPath, entry.Name()))
		if err != nil {
			continue // Skip unreadable files
		}

		var storageData commentStorageData
		if err := json.Unmarshal(data, &storageData); err != nil {
			continue // Skip invalid files
		}

		comment := &model.Comment{
			ID:       commentID,
			PasteID:  pasteID,
			ParentID: parentID,
			Data:     storageData.Data,
			AData:    storageData.AData,
			Version:  storageData.Version,
			Vizhash:  storageData.Vizhash,
			Meta: model.CommentMeta{
				PostDate: storageData.PostDate,
			},
		}
		comments = append(comments, comment)
	}

	// Sort by post date
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].Meta.PostDate < comments[j].Meta.PostDate
	})

	return comments, nil
}

// CommentExists checks if a comment exists.
func (f *Filesystem) CommentExists(pasteID, parentID, commentID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.commentPath(pasteID, parentID, commentID)
	_, err := os.Stat(path)
	return err == nil
}

// SetValue stores a key-value pair.
func (f *Filesystem) SetValue(namespace, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.configPath(namespace, key)

	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(value), 0640); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming config file: %w", err)
	}

	return nil
}

// GetValue retrieves a stored value.
func (f *Filesystem) GetValue(namespace, key string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.configPath(namespace, key)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading config file: %w", err)
	}
	return string(data), nil
}

// GetExpiredPastes returns a list of expired paste IDs.
func (f *Filesystem) GetExpiredPastes(batchSize int) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var expired []string
	now := time.Now().Unix()

	// Walk the data directory looking for paste files
	err := filepath.WalkDir(f.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		// Skip config directory
		if strings.Contains(path, "_config") {
			return nil
		}

		// Skip discussion files
		if strings.Contains(path, ".discussion") || strings.HasSuffix(path, ".json") {
			return nil
		}

		// Read paste to check expiration
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var storageData pasteStorageData
		if err := json.Unmarshal(data, &storageData); err != nil {
			return nil
		}

		if storageData.Meta.ExpireDate > 0 && storageData.Meta.ExpireDate < now {
			// Extract ID from path
			id := filepath.Base(path)
			expired = append(expired, id)
			if len(expired) >= batchSize {
				return filepath.SkipAll
			}
		}

		return nil
	})

	return expired, err
}

// Purge deletes expired pastes.
func (f *Filesystem) Purge(batchSize int) (int, error) {
	ids, err := f.GetExpiredPastes(batchSize)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, id := range ids {
		if err := f.DeletePaste(id); err != nil && err != model.ErrPasteNotFound {
			return count, err
		}
		count++
	}

	return count, nil
}

// PurgeValues removes old config entries.
func (f *Filesystem) PurgeValues(namespace string, maxAge int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	configDir := filepath.Join(f.baseDir, "_config")
	prefix := namespace + "_"
	cutoff := time.Now().Unix() - maxAge

	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil // Ignore errors
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		path := filepath.Join(configDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Try to parse as timestamp
		var timestamp int64
		if _, err := fmt.Sscanf(string(data), "%d", &timestamp); err == nil {
			if timestamp < cutoff {
				os.Remove(path)
			}
		}
	}

	return nil
}

// Close is a no-op for filesystem storage.
func (f *Filesystem) Close() error {
	return nil
}
