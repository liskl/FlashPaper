// Package storage provides the persistence layer for FlashPaper.
// It defines the Storage interface that abstracts different backends
// (database, filesystem) allowing the application to switch between
// them without changing business logic.
//
// The storage layer is responsible for:
// - Paste CRUD operations
// - Comment management
// - Key-value storage for config (rate limiting, server salt)
// - Expired paste purging
//
// All implementations must be safe for concurrent use.
package storage

import (
	"fmt"
	"io"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/model"
)

// Storage defines the contract for paste and comment persistence.
// Implementations must be safe for concurrent use.
// All methods that modify state should be atomic where possible.
type Storage interface {
	// Paste operations

	// CreatePaste stores a new paste with the given ID.
	// Returns model.ErrPasteExists if a paste with this ID already exists.
	// The paste's metadata (expiration, burn-after-reading) is stored
	// alongside the encrypted content.
	CreatePaste(id string, paste *model.Paste) error

	// ReadPaste retrieves a paste by ID.
	// Returns model.ErrPasteNotFound if the paste doesn't exist.
	// Returns model.ErrPasteExpired if the paste has expired (and deletes it).
	ReadPaste(id string) (*model.Paste, error)

	// DeletePaste removes a paste and all its comments.
	// Returns model.ErrPasteNotFound if the paste doesn't exist.
	DeletePaste(id string) error

	// PasteExists checks if a paste with the given ID exists.
	// This is a quick check that doesn't load the full paste data.
	PasteExists(id string) bool

	// Comment operations

	// CreateComment stores a new comment on a paste.
	// Returns model.ErrPasteNotFound if the paste doesn't exist.
	// Returns model.ErrCommentExists if a comment with this ID exists.
	CreateComment(pasteID, parentID, commentID string, comment *model.Comment) error

	// ReadComments retrieves all comments for a paste.
	// Returns an empty slice if no comments exist.
	ReadComments(pasteID string) ([]*model.Comment, error)

	// CommentExists checks if a comment exists.
	CommentExists(pasteID, parentID, commentID string) bool

	// Key-value storage for configuration and rate limiting

	// SetValue stores a string value with the given namespace and key.
	// Used for server salt, rate limiting timestamps, etc.
	SetValue(namespace, key, value string) error

	// GetValue retrieves a stored value.
	// Returns empty string if the key doesn't exist.
	GetValue(namespace, key string) (string, error)

	// Maintenance operations

	// GetExpiredPastes returns a list of expired paste IDs up to batchSize.
	// Used by the purge system to clean up old pastes.
	GetExpiredPastes(batchSize int) ([]string, error)

	// Purge deletes expired pastes up to batchSize.
	// Returns the number of pastes deleted.
	Purge(batchSize int) (int, error)

	// PurgeValues removes outdated rate limiting entries.
	// Entries older than maxAge seconds are removed.
	PurgeValues(namespace string, maxAge int64) error

	// Close releases any resources held by the storage backend.
	// Should be called when the application shuts down.
	Close() error
}

// StorageCloser combines Storage with io.Closer for resource management.
type StorageCloser interface {
	Storage
	io.Closer
}

// New creates a new storage backend based on configuration.
// The returned Storage should be closed when no longer needed.
func New(cfg *config.Config) (Storage, error) {
	switch cfg.Model.Class {
	case "Database":
		return NewDatabase(cfg)
	case "Filesystem":
		return NewFilesystem(cfg)
	default:
		return nil, fmt.Errorf("unknown storage class: %s", cfg.Model.Class)
	}
}

// Namespace constants for key-value storage.
// These prevent key collisions between different subsystems.
const (
	// NamespaceSalt stores the server salt for delete tokens
	NamespaceSalt = "salt"

	// NamespaceTraffic stores rate limiting timestamps per IP hash
	NamespaceTraffic = "traffic"

	// NamespacePurge stores the last purge timestamp
	NamespacePurge = "purge"
)
