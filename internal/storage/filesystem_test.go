// Package storage provides tests for filesystem storage implementation.
package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/model"
)

// testFilesystemConfig creates a config for filesystem testing.
func testFilesystemConfig(t *testing.T) *config.Config {
	tmpDir := t.TempDir()
	return &config.Config{
		Model: config.ModelConfig{
			Class: "Filesystem",
			Dir:   tmpDir, // Filesystem uses Dir, not DSN
		},
	}
}

func TestNewFilesystem_CreatesBaseDir(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// Verify we can use the filesystem
	assert.False(t, fs.PasteExists("nonexistent"))
}

func TestFilesystem_CreatePaste_Success(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	paste := &model.Paste{
		Data:    "encrypted content",
		Version: 2,
		Meta: model.PasteMeta{
			PostDate:       time.Now().Unix(),
			ExpireDate:     time.Now().Add(time.Hour).Unix(),
			OpenDiscussion: true,
			Formatter:      "plaintext",
			Salt:           "serversalt",
		},
	}

	err = fs.CreatePaste("f468483c313401e8", paste)
	require.NoError(t, err)

	// Verify paste exists
	assert.True(t, fs.PasteExists("f468483c313401e8"))
}

func TestFilesystem_CreatePaste_DuplicateID_ReturnsError(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	paste := &model.Paste{Data: "content"}

	err = fs.CreatePaste("testpasteid1234", paste)
	require.NoError(t, err)

	// Try to create with same ID
	err = fs.CreatePaste("testpasteid1234", paste)
	assert.ErrorIs(t, err, model.ErrPasteExists)
}

func TestFilesystem_ReadPaste_Success(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	original := &model.Paste{
		Data:    "test encrypted content",
		Version: 2,
		AData:   []byte(`["iv","salt",100000]`),
		Meta: model.PasteMeta{
			PostDate:       time.Now().Unix(),
			ExpireDate:     time.Now().Add(time.Hour).Unix(),
			OpenDiscussion: true,
			Formatter:      "markdown",
			Salt:           "testsalt",
		},
	}

	pasteID := "readpaste123456"
	err = fs.CreatePaste(pasteID, original)
	require.NoError(t, err)

	// Read it back
	read, err := fs.ReadPaste(pasteID)
	require.NoError(t, err)

	assert.Equal(t, original.Data, read.Data)
	assert.Equal(t, original.Version, read.Version)
	assert.Equal(t, original.Meta.OpenDiscussion, read.Meta.OpenDiscussion)
	assert.Equal(t, original.Meta.Formatter, read.Meta.Formatter)
}

func TestFilesystem_ReadPaste_NotFound(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	_, err = fs.ReadPaste("nonexistent12345")
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestFilesystem_ReadPaste_Expired(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	paste := &model.Paste{
		Data: "expired content",
		Meta: model.PasteMeta{
			ExpireDate: time.Now().Add(-time.Hour).Unix(), // Expired an hour ago
		},
	}

	pasteID := "expiredpaste123"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	// Should return expired error
	_, err = fs.ReadPaste(pasteID)
	assert.ErrorIs(t, err, model.ErrPasteExpired)
}

func TestFilesystem_DeletePaste_Success(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	paste := &model.Paste{Data: "to delete"}
	pasteID := "deletepaste1234"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	// Delete it
	err = fs.DeletePaste(pasteID)
	require.NoError(t, err)

	// Verify it's gone
	assert.False(t, fs.PasteExists(pasteID))
}

func TestFilesystem_DeletePaste_NotFound(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	err = fs.DeletePaste("nonexistent12345")
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

// Comment tests

func TestFilesystem_CreateComment_Success(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// First create a paste
	paste := &model.Paste{Data: "paste for comments"}
	pasteID := "commentpaste123"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	// Create a comment
	comment := &model.Comment{
		Data:    "encrypted comment",
		Version: 2,
		Vizhash: "abc123hash",
		Meta: model.CommentMeta{
			PostDate: time.Now().Unix(),
		},
	}

	err = fs.CreateComment(pasteID, pasteID, "comment12345678", comment)
	require.NoError(t, err)

	// Verify comment exists
	assert.True(t, fs.CommentExists(pasteID, pasteID, "comment12345678"))
}

func TestFilesystem_CreateComment_PasteNotFound(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	comment := &model.Comment{Data: "orphan comment"}
	err = fs.CreateComment("nonexistent12345", "nonexistent12345", "comment12345678", comment)
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestFilesystem_CreateComment_DuplicateID(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// First create a paste
	paste := &model.Paste{Data: "paste data"}
	pasteID := "dupcomment12345"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	// Create a comment
	comment := &model.Comment{Data: "comment"}
	commentID := "commentdup12345"
	err = fs.CreateComment(pasteID, pasteID, commentID, comment)
	require.NoError(t, err)

	// Try to create again with same ID
	err = fs.CreateComment(pasteID, pasteID, commentID, comment)
	assert.ErrorIs(t, err, model.ErrCommentExists)
}

func TestFilesystem_ReadComments_Success(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// Create a paste
	paste := &model.Paste{Data: "paste with multiple comments"}
	pasteID := "multicomment123"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	// Create multiple comments
	now := time.Now().Unix()
	comment1 := &model.Comment{
		Data: "first comment",
		Meta: model.CommentMeta{PostDate: now},
	}
	comment2 := &model.Comment{
		Data: "second comment",
		Meta: model.CommentMeta{PostDate: now + 1},
	}

	err = fs.CreateComment(pasteID, pasteID, "comment10000001", comment1)
	require.NoError(t, err)
	err = fs.CreateComment(pasteID, pasteID, "comment20000002", comment2)
	require.NoError(t, err)

	// Read all comments
	comments, err := fs.ReadComments(pasteID)
	require.NoError(t, err)
	assert.Len(t, comments, 2)

	// Comments should be sorted by post date
	assert.Equal(t, "first comment", comments[0].Data)
	assert.Equal(t, "second comment", comments[1].Data)
}

func TestFilesystem_ReadComments_NoComments(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// Create a paste without comments
	paste := &model.Paste{Data: "lonely paste"}
	pasteID := "nocomments12345"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	// Read comments (should be empty, not error)
	comments, err := fs.ReadComments(pasteID)
	require.NoError(t, err)
	assert.Empty(t, comments)
}

func TestFilesystem_CommentExists(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// Create a paste and comment
	paste := &model.Paste{Data: "paste"}
	pasteID := "existscheck1234"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	comment := &model.Comment{Data: "comment"}
	commentID := "commentexists12"
	err = fs.CreateComment(pasteID, pasteID, commentID, comment)
	require.NoError(t, err)

	// Should exist
	assert.True(t, fs.CommentExists(pasteID, pasteID, commentID))

	// Should not exist
	assert.False(t, fs.CommentExists(pasteID, pasteID, "nonexistent1234"))
}

// Config/Value tests

func TestFilesystem_SetValue_Success(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	err = fs.SetValue("test", "key1", "value1")
	require.NoError(t, err)

	// Verify we can read it back
	value, err := fs.GetValue("test", "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", value)
}

func TestFilesystem_GetValue_NotFound(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	value, err := fs.GetValue("nonexistent", "key")
	require.NoError(t, err) // Not an error, just empty
	assert.Empty(t, value)
}

func TestFilesystem_SetValue_Overwrite(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	err = fs.SetValue("test", "key", "original")
	require.NoError(t, err)

	err = fs.SetValue("test", "key", "updated")
	require.NoError(t, err)

	value, err := fs.GetValue("test", "key")
	require.NoError(t, err)
	assert.Equal(t, "updated", value)
}

// Expiration tests

func TestFilesystem_GetExpiredPastes(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// Create expired paste
	expiredPaste := &model.Paste{
		Data: "expired",
		Meta: model.PasteMeta{
			ExpireDate: time.Now().Add(-time.Hour).Unix(),
		},
	}
	err = fs.CreatePaste("expired12345678", expiredPaste)
	require.NoError(t, err)

	// Create non-expired paste
	validPaste := &model.Paste{
		Data: "valid",
		Meta: model.PasteMeta{
			ExpireDate: time.Now().Add(time.Hour).Unix(),
		},
	}
	err = fs.CreatePaste("valid123456789a", validPaste)
	require.NoError(t, err)

	// Create never-expires paste
	neverPaste := &model.Paste{
		Data: "never expires",
		Meta: model.PasteMeta{
			ExpireDate: 0,
		},
	}
	err = fs.CreatePaste("never123456789a", neverPaste)
	require.NoError(t, err)

	// Get expired pastes
	expired, err := fs.GetExpiredPastes(10)
	require.NoError(t, err)

	// Should only contain the expired one
	assert.Len(t, expired, 1)
	assert.Contains(t, expired, "expired12345678")
}

func TestFilesystem_Purge(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// Create expired paste
	expiredPaste := &model.Paste{
		Data: "to be purged",
		Meta: model.PasteMeta{
			ExpireDate: time.Now().Add(-time.Hour).Unix(),
		},
	}
	err = fs.CreatePaste("purge123456789a", expiredPaste)
	require.NoError(t, err)

	// Verify the paste exists before purge
	assert.True(t, fs.PasteExists("purge123456789a"))

	// Purge - should delete at least 1 paste (our expired one)
	count, err := fs.Purge(10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1)

	// Verify it's gone - this is the key assertion
	assert.False(t, fs.PasteExists("purge123456789a"))
}

func TestFilesystem_Close(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)

	err = fs.Close()
	assert.NoError(t, err)
}

func TestFilesystem_DeletePaste_WithComments(t *testing.T) {
	cfg := testFilesystemConfig(t)
	fs, err := NewFilesystem(cfg)
	require.NoError(t, err)
	defer fs.Close()

	// Create paste with comments
	paste := &model.Paste{Data: "paste with comments"}
	pasteID := "delwithcomment1"
	err = fs.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	comment := &model.Comment{Data: "comment"}
	err = fs.CreateComment(pasteID, pasteID, "comment12345678", comment)
	require.NoError(t, err)

	// Delete paste (should also delete discussion directory)
	err = fs.DeletePaste(pasteID)
	require.NoError(t, err)

	// Both should be gone
	assert.False(t, fs.PasteExists(pasteID))
	assert.False(t, fs.CommentExists(pasteID, pasteID, "comment12345678"))
}
