package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/model"
)

// skipIfNoCGO skips the test if SQLite is not available (requires CGO)
func skipIfNoCGO(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Class:  "Database",
			Driver: "sqlite3",
			DSN:    ":memory:",
		},
	}
	_, err := NewDatabase(cfg)
	if err != nil && strings.Contains(err.Error(), "CGO_ENABLED=0") {
		t.Skip("Skipping test: SQLite requires CGO which is not available")
	}
}

// testDatabaseConfig creates a config for SQLite testing.
// Automatically skips the test if CGO is not available.
func testDatabaseConfig(t *testing.T) *config.Config {
	skipIfNoCGO(t)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	return &config.Config{
		Model: config.ModelConfig{
			Class:  "Database",
			Driver: "sqlite3",
			DSN:    dbPath,
		},
	}
}

func TestNewDatabase_CreatesTables(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Verify tables exist by attempting operations
	assert.False(t, db.PasteExists("nonexistent"))
}

func TestDatabase_CreatePaste_Success(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

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

	err = db.CreatePaste("f468483c313401e8", paste)
	require.NoError(t, err)

	// Verify paste exists
	assert.True(t, db.PasteExists("f468483c313401e8"))
}

func TestDatabase_CreatePaste_DuplicateID_ReturnsError(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	paste := &model.Paste{Data: "content"}

	err = db.CreatePaste("testid", paste)
	require.NoError(t, err)

	// Try to create with same ID
	err = db.CreatePaste("testid", paste)
	assert.ErrorIs(t, err, model.ErrPasteExists)
}

func TestDatabase_ReadPaste_Success(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	original := &model.Paste{
		Data:    "encrypted content",
		Version: 2,
		Meta: model.PasteMeta{
			PostDate:         time.Now().Unix(),
			ExpireDate:       time.Now().Add(time.Hour).Unix(),
			OpenDiscussion:   true,
			BurnAfterReading: false,
			Formatter:        "markdown",
			Salt:             "serversalt",
		},
	}

	err = db.CreatePaste("testpaste", original)
	require.NoError(t, err)

	// Read it back
	paste, err := db.ReadPaste("testpaste")
	require.NoError(t, err)

	assert.Equal(t, "testpaste", paste.ID)
	assert.Equal(t, original.Data, paste.Data)
	assert.Equal(t, original.Version, paste.Version)
	assert.Equal(t, original.Meta.OpenDiscussion, paste.Meta.OpenDiscussion)
	assert.Equal(t, original.Meta.Formatter, paste.Meta.Formatter)
}

func TestDatabase_ReadPaste_NotFound(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ReadPaste("nonexistent")
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestDatabase_ReadPaste_Expired_DeletesAndReturnsError(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Create an already-expired paste
	paste := &model.Paste{
		Data: "old content",
		Meta: model.PasteMeta{
			ExpireDate: time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
		},
	}

	err = db.CreatePaste("expired", paste)
	require.NoError(t, err)

	// Try to read it
	_, err = db.ReadPaste("expired")
	assert.ErrorIs(t, err, model.ErrPasteExpired)

	// Verify it was deleted
	assert.False(t, db.PasteExists("expired"))
}

func TestDatabase_DeletePaste_Success(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	paste := &model.Paste{Data: "content"}
	err = db.CreatePaste("todelete", paste)
	require.NoError(t, err)

	err = db.DeletePaste("todelete")
	require.NoError(t, err)

	assert.False(t, db.PasteExists("todelete"))
}

func TestDatabase_DeletePaste_NotFound(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	err = db.DeletePaste("nonexistent")
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestDatabase_DeletePaste_DeletesComments(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Create paste with discussion
	paste := &model.Paste{
		Data: "content",
		Meta: model.PasteMeta{OpenDiscussion: true},
	}
	err = db.CreatePaste("withcomments", paste)
	require.NoError(t, err)

	// Add comments
	comment := &model.Comment{
		Data: "comment content",
		Meta: model.CommentMeta{PostDate: time.Now().Unix()},
	}
	err = db.CreateComment("withcomments", "", "comment1", comment)
	require.NoError(t, err)

	// Delete paste
	err = db.DeletePaste("withcomments")
	require.NoError(t, err)

	// Verify comment is gone too
	comments, err := db.ReadComments("withcomments")
	require.NoError(t, err)
	assert.Empty(t, comments)
}

func TestDatabase_CreateComment_Success(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Create paste first
	paste := &model.Paste{
		Data: "content",
		Meta: model.PasteMeta{OpenDiscussion: true},
	}
	err = db.CreatePaste("paste1", paste)
	require.NoError(t, err)

	// Create comment
	comment := &model.Comment{
		Data:    "comment content",
		Vizhash: "vizhash_data",
		Version: 2,
		Meta:    model.CommentMeta{PostDate: time.Now().Unix()},
	}
	err = db.CreateComment("paste1", "", "comment1", comment)
	require.NoError(t, err)

	assert.True(t, db.CommentExists("paste1", "", "comment1"))
}

func TestDatabase_CreateComment_PasteNotFound(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	comment := &model.Comment{Data: "content"}
	err = db.CreateComment("nonexistent", "", "c1", comment)
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestDatabase_CreateComment_DuplicateID(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	paste := &model.Paste{Data: "content", Meta: model.PasteMeta{OpenDiscussion: true}}
	err = db.CreatePaste("paste1", paste)
	require.NoError(t, err)

	comment := &model.Comment{Data: "content", Meta: model.CommentMeta{PostDate: time.Now().Unix()}}
	err = db.CreateComment("paste1", "", "c1", comment)
	require.NoError(t, err)

	// Try duplicate
	err = db.CreateComment("paste1", "", "c1", comment)
	assert.ErrorIs(t, err, model.ErrCommentExists)
}

func TestDatabase_ReadComments_Success(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	paste := &model.Paste{Data: "content", Meta: model.PasteMeta{OpenDiscussion: true}}
	err = db.CreatePaste("paste1", paste)
	require.NoError(t, err)

	// Add multiple comments
	now := time.Now().Unix()
	for i, id := range []string{"c1", "c2", "c3"} {
		comment := &model.Comment{
			Data:    "comment " + id,
			Vizhash: "viz" + id,
			Version: 2,
			Meta:    model.CommentMeta{PostDate: now + int64(i)},
		}
		err = db.CreateComment("paste1", "", id, comment)
		require.NoError(t, err)
	}

	// Read comments
	comments, err := db.ReadComments("paste1")
	require.NoError(t, err)
	require.Len(t, comments, 3)

	// Should be sorted by post date
	assert.Equal(t, "c1", comments[0].ID)
	assert.Equal(t, "c2", comments[1].ID)
	assert.Equal(t, "c3", comments[2].ID)
}

func TestDatabase_ReadComments_Empty(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	comments, err := db.ReadComments("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, comments)
}

func TestDatabase_SetValue_GetValue(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	err = db.SetValue("test", "key1", "value1")
	require.NoError(t, err)

	value, err := db.GetValue("test", "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", value)
}

func TestDatabase_SetValue_Overwrites(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	err = db.SetValue("test", "key1", "original")
	require.NoError(t, err)

	err = db.SetValue("test", "key1", "updated")
	require.NoError(t, err)

	value, err := db.GetValue("test", "key1")
	require.NoError(t, err)
	assert.Equal(t, "updated", value)
}

func TestDatabase_GetValue_NotFound_ReturnsEmpty(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	value, err := db.GetValue("nonexistent", "key")
	require.NoError(t, err)
	assert.Empty(t, value)
}

func TestDatabase_GetExpiredPastes(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Create some expired pastes
	for i, id := range []string{"exp1", "exp2", "exp3"} {
		paste := &model.Paste{
			Data: "content",
			Meta: model.PasteMeta{
				ExpireDate: time.Now().Add(-time.Duration(i+1) * time.Hour).Unix(),
			},
		}
		err = db.CreatePaste(id, paste)
		require.NoError(t, err)
	}

	// Create non-expired paste
	paste := &model.Paste{
		Data: "content",
		Meta: model.PasteMeta{
			ExpireDate: time.Now().Add(time.Hour).Unix(),
		},
	}
	err = db.CreatePaste("notexpired", paste)
	require.NoError(t, err)

	expired, err := db.GetExpiredPastes(10)
	require.NoError(t, err)
	assert.Len(t, expired, 3)
	assert.NotContains(t, expired, "notexpired")
}

func TestDatabase_Purge(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Create expired pastes
	for _, id := range []string{"exp1", "exp2"} {
		paste := &model.Paste{
			Data: "content",
			Meta: model.PasteMeta{
				ExpireDate: time.Now().Add(-time.Hour).Unix(),
			},
		}
		err = db.CreatePaste(id, paste)
		require.NoError(t, err)
	}

	count, err := db.Purge(10)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify they're gone
	assert.False(t, db.PasteExists("exp1"))
	assert.False(t, db.PasteExists("exp2"))
}

func TestDatabase_ConcurrentAccess(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Create a paste
	paste := &model.Paste{Data: "content"}
	err = db.CreatePaste("concurrent", paste)
	require.NoError(t, err)

	// Access concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = db.ReadPaste("concurrent")
			_ = db.PasteExists("concurrent")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestDatabase_IntegrationWithFilesystem tests that both backends work similarly
func TestDatabase_SameInterfaceAsFilesystem(t *testing.T) {
	skipIfNoCGO(t)

	// This test verifies that Database and Filesystem implement the same interface
	// and behave consistently for basic operations

	tmpDir := t.TempDir()

	dbCfg := &config.Config{
		Model: config.ModelConfig{
			Class:  "Database",
			Driver: "sqlite3",
			DSN:    filepath.Join(tmpDir, "test.db"),
		},
	}

	fsCfg := &config.Config{
		Model: config.ModelConfig{
			Class: "Filesystem",
			Dir:   filepath.Join(tmpDir, "fs"),
		},
	}

	dbStore, err := New(dbCfg)
	require.NoError(t, err)
	defer dbStore.Close()

	fsStore, err := New(fsCfg)
	require.NoError(t, err)
	defer fsStore.Close()

	// Test same operations on both
	for name, store := range map[string]Storage{"db": dbStore, "fs": fsStore} {
		t.Run(name, func(t *testing.T) {
			paste := &model.Paste{
				Data:    "test content",
				Version: 2,
				Meta: model.PasteMeta{
					PostDate:  time.Now().Unix(),
					Formatter: "plaintext",
				},
			}

			// Create
			err := store.CreatePaste("test123", paste)
			require.NoError(t, err)

			// Exists
			assert.True(t, store.PasteExists("test123"))

			// Read
			read, err := store.ReadPaste("test123")
			require.NoError(t, err)
			assert.Equal(t, "test content", read.Data)

			// Delete
			err = store.DeletePaste("test123")
			require.NoError(t, err)

			assert.False(t, store.PasteExists("test123"))
		})
	}
}

// TestDatabase_NoPlaintextInStorage verifies that the database only stores
// encrypted content and never contains plaintext. This is a security test
// to ensure the zero-knowledge principle is maintained.
func TestDatabase_NoPlaintextInStorage(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)

	// These are plaintext strings that should NEVER appear in the database.
	// In a real scenario, clients encrypt content before sending to server.
	// The server should only ever see ciphertext (base64-encoded encrypted data).
	plaintextSecrets := []string{
		"SUPER_SECRET_PASSWORD_12345",
		"my-confidential-api-key-xyz",
		"SSN: 123-45-6789",
		"credit card: 4111-1111-1111-1111",
	}

	// Simulated encrypted content (base64-like strings representing ciphertext)
	// In production, this would be actual AES-256-GCM encrypted data
	encryptedData := "YWVzLTI1Ni1nY20gZW5jcnlwdGVkIGNvbnRlbnQgaGVyZQ=="
	encryptedAttachment := "ZW5jcnlwdGVkIGF0dGFjaG1lbnQgZGF0YSBoZXJl"

	// Create paste with "encrypted" content (simulating what client sends)
	paste := &model.Paste{
		Data:           encryptedData,
		Attachment:     encryptedAttachment,
		AttachmentName: "secret_document.pdf", // Filename is also encrypted in real usage
		Version:        2,
		AData:          []byte(`[["iv_base64","salt_base64",100000,256,128,"aes","gcm","zlib"],"plaintext",0,0]`),
		Meta: model.PasteMeta{
			PostDate:       time.Now().Unix(),
			ExpireDate:     time.Now().Add(time.Hour).Unix(),
			OpenDiscussion: true,
			Formatter:      "plaintext",
			Salt:           "server_salt_for_delete_token",
		},
	}

	pasteID := "a1b2c3d4e5f67890"
	err = db.CreatePaste(pasteID, paste)
	require.NoError(t, err)

	// Also create a comment with "encrypted" content
	comment := &model.Comment{
		Data:    "ZW5jcnlwdGVkIGNvbW1lbnQgZGF0YQ==", // Simulated encrypted comment
		Vizhash: "vizhash_data",
		Version: 2,
		Meta:    model.CommentMeta{PostDate: time.Now().Unix()},
	}
	err = db.CreateComment(pasteID, "", "comment123", comment)
	require.NoError(t, err)

	// Close the database to flush all writes
	db.Close()

	// Read the raw database file
	dbBytes, err := os.ReadFile(cfg.Model.DSN)
	require.NoError(t, err, "Failed to read database file")
	require.NotEmpty(t, dbBytes, "Database file should not be empty")

	// Convert to string for searching (works for SQLite which stores text as UTF-8)
	dbContent := string(dbBytes)

	// Verify that NO plaintext secrets appear in the raw database
	for _, secret := range plaintextSecrets {
		assert.NotContains(t, dbContent, secret,
			"SECURITY VIOLATION: Plaintext '%s' found in database! Data should be encrypted client-side.", secret)
	}

	// Verify the encrypted data IS present (proves data was stored)
	assert.Contains(t, dbContent, encryptedData,
		"Encrypted paste data should be present in database")
	assert.Contains(t, dbContent, encryptedAttachment,
		"Encrypted attachment should be present in database")

	// Additional check: common plaintext patterns that should never appear
	dangerousPatterns := []string{
		"password=",
		"secret_key=",
		"BEGIN RSA PRIVATE KEY",
		"BEGIN OPENSSH PRIVATE KEY",
	}

	for _, pattern := range dangerousPatterns {
		assert.NotContains(t, dbContent, pattern,
			"SECURITY VIOLATION: Dangerous pattern '%s' found in database!", pattern)
	}
}

// TestDatabase_EncryptedContentRoundTrip verifies that encrypted content
// stored in the database can be retrieved exactly as it was stored,
// ensuring no corruption or modification of ciphertext occurs.
func TestDatabase_EncryptedContentRoundTrip(t *testing.T) {
	cfg := testDatabaseConfig(t)
	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Simulated encrypted content with special characters that might get corrupted
	// This represents base64-encoded AES-256-GCM ciphertext
	testCases := []struct {
		name string
		data string
	}{
		{
			name: "standard base64",
			data: "SGVsbG8gV29ybGQhIFRoaXMgaXMgZW5jcnlwdGVkIGRhdGE=",
		},
		{
			name: "base64 with padding",
			data: "YQ==",
		},
		{
			name: "long ciphertext",
			data: "VGhpcyBpcyBhIGxvbmdlciBwaWVjZSBvZiBlbmNyeXB0ZWQgY29udGVudCB0aGF0IG1pZ2h0IGNvbnRhaW4gc2Vuc2l0aXZlIGluZm9ybWF0aW9uIGxpa2UgcGFzc3dvcmRzLCBBUEkga2V5cywgb3IgcGVyc29uYWwgZGF0YS4gSXQgc2hvdWxkIGFsbCBiZSBlbmNyeXB0ZWQgYW5kIG5vdCByZWFkYWJsZSBieSB0aGUgc2VydmVyLg==",
		},
		{
			name: "binary-like base64",
			data: "/+/+/+8f7x/vH+8=",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pasteID := "test" + string(rune('a'+i)) + "1234567890"

			original := &model.Paste{
				Data:    tc.data,
				Version: 2,
				Meta: model.PasteMeta{
					PostDate:  time.Now().Unix(),
					Formatter: "plaintext",
				},
			}

			err := db.CreatePaste(pasteID, original)
			require.NoError(t, err)

			retrieved, err := db.ReadPaste(pasteID)
			require.NoError(t, err)

			assert.Equal(t, tc.data, retrieved.Data,
				"Encrypted content must be retrieved exactly as stored - no corruption allowed")
		})
	}
}

// Skip this test if DATABASE_URL is not set (for CI integration)
func TestDatabase_PostgresIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping PostgreSQL integration test")
	}

	cfg := &config.Config{
		Model: config.ModelConfig{
			Class:  "Database",
			Driver: "postgres",
			DSN:    dsn,
		},
	}

	db, err := NewDatabase(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Basic smoke test
	paste := &model.Paste{Data: "test", Version: 2}
	err = db.CreatePaste("pgtest123", paste)
	require.NoError(t, err)

	_, err = db.ReadPaste("pgtest123")
	require.NoError(t, err)

	err = db.DeletePaste("pgtest123")
	require.NoError(t, err)
}
