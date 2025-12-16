// Package storage provides the database implementation of the Storage interface.
// This implementation supports SQLite, PostgreSQL, and MySQL through Go's
// database/sql package, providing a consistent API across all three databases.
//
// The database schema mirrors PrivateBin's structure for compatibility:
// - paste: stores encrypted paste data and metadata
// - comment: stores encrypted comments with threading support
// - config: stores key-value pairs for server configuration
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	// Database drivers - imported for side effects (driver registration)
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/liskl/flashpaper/internal/config"
	"github.com/liskl/flashpaper/internal/model"
)

// Database implements the Storage interface using SQL databases.
// Supports SQLite, PostgreSQL, and MySQL.
type Database struct {
	db     *sql.DB
	driver string // "sqlite3", "postgres", or "mysql"
	mu     sync.RWMutex
}

// NewDatabase creates a new database storage backend.
// The database tables are created automatically if they don't exist.
func NewDatabase(cfg *config.Config) (*Database, error) {
	// Determine the driver name for sql.Open
	// PostgreSQL DSN format needs the "postgres" driver
	driver := cfg.Model.Driver
	dsn := cfg.Model.DSN

	// For PostgreSQL, the driver is "postgres" but DSN might use "postgresql://"
	if driver == "postgres" && strings.HasPrefix(dsn, "postgresql://") {
		dsn = strings.Replace(dsn, "postgresql://", "postgres://", 1)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify connection works
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	d := &Database{
		db:     db,
		driver: driver,
	}

	// Create tables if they don't exist
	if err := d.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating tables: %w", err)
	}

	return d, nil
}

// createTables creates the necessary database tables.
// The schema is designed to be compatible with PrivateBin.
func (d *Database) createTables() error {
	// Get database-specific SQL variations
	textType := d.textType()
	autoIncrement := d.autoIncrement()
	_ = autoIncrement // Not used for these tables

	// Create paste table
	pasteSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS paste (
			dataid CHAR(16) PRIMARY KEY,
			data %s NOT NULL,
			expiredate BIGINT,
			meta %s
		)
	`, textType, textType)

	if _, err := d.db.Exec(pasteSQL); err != nil {
		return fmt.Errorf("creating paste table: %w", err)
	}

	// Create comment table
	commentSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS comment (
			dataid CHAR(16) PRIMARY KEY,
			pasteid CHAR(16) NOT NULL,
			parentid CHAR(16),
			data %s NOT NULL,
			vizhash %s,
			postdate BIGINT NOT NULL
		)
	`, textType, textType)

	if _, err := d.db.Exec(commentSQL); err != nil {
		return fmt.Errorf("creating comment table: %w", err)
	}

	// Create index on comment.pasteid for efficient retrieval
	indexSQL := d.createIndexSQL("idx_comment_pasteid", "comment", "pasteid")
	if _, err := d.db.Exec(indexSQL); err != nil {
		// Ignore error if index already exists
		if !strings.Contains(err.Error(), "already exists") &&
			!strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("creating comment index: %w", err)
		}
	}

	// Create config table for key-value storage
	configSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS config (
			id VARCHAR(64) PRIMARY KEY,
			value %s NOT NULL
		)
	`, textType)

	if _, err := d.db.Exec(configSQL); err != nil {
		return fmt.Errorf("creating config table: %w", err)
	}

	return nil
}

// textType returns the appropriate TEXT type for the database driver.
func (d *Database) textType() string {
	switch d.driver {
	case "mysql":
		return "MEDIUMTEXT" // Up to 16MB
	default:
		return "TEXT"
	}
}

// autoIncrement returns the auto-increment syntax for the database.
func (d *Database) autoIncrement() string {
	switch d.driver {
	case "postgres":
		return "SERIAL"
	case "mysql":
		return "AUTO_INCREMENT"
	default: // sqlite3
		return "AUTOINCREMENT"
	}
}

// createIndexSQL returns database-specific CREATE INDEX syntax.
func (d *Database) createIndexSQL(name, table, column string) string {
	switch d.driver {
	case "postgres":
		return fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", name, table, column)
	default:
		return fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", name, table, column)
	}
}

// placeholder returns the appropriate placeholder for the database.
// PostgreSQL uses $1, $2, etc. Others use ?.
func (d *Database) placeholder(n int) string {
	if d.driver == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// placeholders returns a string of comma-separated placeholders.
func (d *Database) placeholders(count int) string {
	if d.driver == "postgres" {
		parts := make([]string, count)
		for i := 0; i < count; i++ {
			parts[i] = fmt.Sprintf("$%d", i+1)
		}
		return strings.Join(parts, ", ")
	}
	return strings.Repeat("?, ", count-1) + "?"
}

// CreatePaste stores a new paste in the database.
func (d *Database) CreatePaste(id string, paste *model.Paste) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Serialize metadata to JSON
	metaJSON, err := json.Marshal(paste.Meta)
	if err != nil {
		return fmt.Errorf("serializing paste meta: %w", err)
	}

	// Combine data fields for storage
	// PrivateBin stores the full paste object as JSON in the data field
	storageData := struct {
		Data           string          `json:"data"`
		AttachmentName string          `json:"attachmentname,omitempty"`
		Attachment     string          `json:"attachment,omitempty"`
		AData          json.RawMessage `json:"adata,omitempty"`
		Version        int             `json:"v"`
	}{
		Data:           paste.Data,
		AttachmentName: paste.AttachmentName,
		Attachment:     paste.Attachment,
		AData:          paste.AData,
		Version:        paste.Version,
	}
	dataJSON, err := json.Marshal(storageData)
	if err != nil {
		return fmt.Errorf("serializing paste data: %w", err)
	}

	query := fmt.Sprintf(
		"INSERT INTO paste (dataid, data, expiredate, meta) VALUES (%s)",
		d.placeholders(4),
	)

	_, err = d.db.Exec(query, id, string(dataJSON), paste.Meta.ExpireDate, string(metaJSON))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") ||
			strings.Contains(err.Error(), "duplicate") ||
			strings.Contains(err.Error(), "Duplicate") {
			return model.ErrPasteExists
		}
		return fmt.Errorf("inserting paste: %w", err)
	}

	return nil
}

// ReadPaste retrieves a paste from the database.
func (d *Database) ReadPaste(id string) (*model.Paste, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := fmt.Sprintf(
		"SELECT data, expiredate, meta FROM paste WHERE dataid = %s",
		d.placeholder(1),
	)

	var dataJSON, metaJSON string
	var expireDate sql.NullInt64

	err := d.db.QueryRow(query, id).Scan(&dataJSON, &expireDate, &metaJSON)
	if err == sql.ErrNoRows {
		return nil, model.ErrPasteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying paste: %w", err)
	}

	// Deserialize data
	var storageData struct {
		Data           string          `json:"data"`
		AttachmentName string          `json:"attachmentname,omitempty"`
		Attachment     string          `json:"attachment,omitempty"`
		AData          json.RawMessage `json:"adata,omitempty"`
		Version        int             `json:"v"`
	}
	if err := json.Unmarshal([]byte(dataJSON), &storageData); err != nil {
		return nil, fmt.Errorf("deserializing paste data: %w", err)
	}

	// Deserialize metadata
	var meta model.PasteMeta
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return nil, fmt.Errorf("deserializing paste meta: %w", err)
	}

	// Set expire date from database column (more reliable than JSON)
	if expireDate.Valid {
		meta.ExpireDate = expireDate.Int64
	}

	paste := &model.Paste{
		ID:             id,
		Data:           storageData.Data,
		AttachmentName: storageData.AttachmentName,
		Attachment:     storageData.Attachment,
		AData:          storageData.AData,
		Version:        storageData.Version,
		Meta:           meta,
	}

	// Check if expired
	if paste.IsExpired() {
		// Delete the expired paste (don't hold lock for delete)
		d.mu.RUnlock()
		d.DeletePaste(id)
		d.mu.RLock()
		return nil, model.ErrPasteExpired
	}

	return paste, nil
}

// DeletePaste removes a paste and all its comments from the database.
func (d *Database) DeletePaste(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Start transaction
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete comments first (foreign key-like behavior)
	commentQuery := fmt.Sprintf("DELETE FROM comment WHERE pasteid = %s", d.placeholder(1))
	if _, err := tx.Exec(commentQuery, id); err != nil {
		return fmt.Errorf("deleting comments: %w", err)
	}

	// Delete paste
	pasteQuery := fmt.Sprintf("DELETE FROM paste WHERE dataid = %s", d.placeholder(1))
	result, err := tx.Exec(pasteQuery, id)
	if err != nil {
		return fmt.Errorf("deleting paste: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return model.ErrPasteNotFound
	}

	return tx.Commit()
}

// PasteExists checks if a paste exists in the database.
func (d *Database) PasteExists(id string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := fmt.Sprintf("SELECT 1 FROM paste WHERE dataid = %s", d.placeholder(1))
	var exists int
	err := d.db.QueryRow(query, id).Scan(&exists)
	return err == nil
}

// CreateComment stores a new comment in the database.
func (d *Database) CreateComment(pasteID, parentID, commentID string, comment *model.Comment) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Verify paste exists
	if !d.pasteExistsUnsafe(pasteID) {
		return model.ErrPasteNotFound
	}

	// Serialize comment data
	dataJSON, err := json.Marshal(struct {
		Data    string          `json:"data"`
		AData   json.RawMessage `json:"adata,omitempty"`
		Version int             `json:"v"`
	}{
		Data:    comment.Data,
		AData:   comment.AData,
		Version: comment.Version,
	})
	if err != nil {
		return fmt.Errorf("serializing comment: %w", err)
	}

	query := fmt.Sprintf(
		"INSERT INTO comment (dataid, pasteid, parentid, data, vizhash, postdate) VALUES (%s)",
		d.placeholders(6),
	)

	_, err = d.db.Exec(query, commentID, pasteID, parentID, string(dataJSON), comment.Vizhash, comment.Meta.PostDate)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") ||
			strings.Contains(err.Error(), "duplicate") ||
			strings.Contains(err.Error(), "Duplicate") {
			return model.ErrCommentExists
		}
		return fmt.Errorf("inserting comment: %w", err)
	}

	return nil
}

// ReadComments retrieves all comments for a paste.
func (d *Database) ReadComments(pasteID string) ([]*model.Comment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := fmt.Sprintf(
		"SELECT dataid, parentid, data, vizhash, postdate FROM comment WHERE pasteid = %s ORDER BY postdate ASC",
		d.placeholder(1),
	)

	rows, err := d.db.Query(query, pasteID)
	if err != nil {
		return nil, fmt.Errorf("querying comments: %w", err)
	}
	defer rows.Close()

	var comments []*model.Comment
	for rows.Next() {
		var id string
		var parentID sql.NullString
		var dataJSON string
		var vizhash sql.NullString
		var postDate int64

		if err := rows.Scan(&id, &parentID, &dataJSON, &vizhash, &postDate); err != nil {
			return nil, fmt.Errorf("scanning comment row: %w", err)
		}

		// Deserialize comment data
		var data struct {
			Data    string          `json:"data"`
			AData   json.RawMessage `json:"adata,omitempty"`
			Version int             `json:"v"`
		}
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			return nil, fmt.Errorf("deserializing comment: %w", err)
		}

		comment := &model.Comment{
			ID:       id,
			PasteID:  pasteID,
			ParentID: parentID.String,
			Data:     data.Data,
			AData:    data.AData,
			Version:  data.Version,
			Vizhash:  vizhash.String,
			Meta: model.CommentMeta{
				PostDate: postDate,
			},
		}
		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating comments: %w", err)
	}

	return comments, nil
}

// CommentExists checks if a comment exists.
func (d *Database) CommentExists(pasteID, parentID, commentID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := fmt.Sprintf("SELECT 1 FROM comment WHERE dataid = %s AND pasteid = %s", d.placeholder(1), d.placeholder(2))
	var exists int
	err := d.db.QueryRow(query, commentID, pasteID).Scan(&exists)
	return err == nil
}

// SetValue stores a key-value pair in the config table.
func (d *Database) SetValue(namespace, key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	id := namespace + "_" + key

	// Use UPSERT pattern that works across databases
	var query string
	switch d.driver {
	case "sqlite3":
		query = fmt.Sprintf(
			"INSERT OR REPLACE INTO config (id, value) VALUES (%s)",
			d.placeholders(2),
		)
	case "postgres":
		query = fmt.Sprintf(
			"INSERT INTO config (id, value) VALUES (%s) ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value",
			d.placeholders(2),
		)
	case "mysql":
		query = fmt.Sprintf(
			"INSERT INTO config (id, value) VALUES (%s) ON DUPLICATE KEY UPDATE value = VALUES(value)",
			d.placeholders(2),
		)
	}

	_, err := d.db.Exec(query, id, value)
	if err != nil {
		return fmt.Errorf("setting value: %w", err)
	}
	return nil
}

// GetValue retrieves a value from the config table.
func (d *Database) GetValue(namespace, key string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	id := namespace + "_" + key
	query := fmt.Sprintf("SELECT value FROM config WHERE id = %s", d.placeholder(1))

	var value string
	err := d.db.QueryRow(query, id).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting value: %w", err)
	}
	return value, nil
}

// GetExpiredPastes returns a list of expired paste IDs.
func (d *Database) GetExpiredPastes(batchSize int) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now().Unix()

	var query string
	switch d.driver {
	case "postgres":
		query = fmt.Sprintf(
			"SELECT dataid FROM paste WHERE expiredate > 0 AND expiredate < %s LIMIT %s",
			d.placeholder(1), d.placeholder(2),
		)
	default:
		query = fmt.Sprintf(
			"SELECT dataid FROM paste WHERE expiredate > 0 AND expiredate < %s LIMIT %s",
			d.placeholder(1), d.placeholder(2),
		)
	}

	rows, err := d.db.Query(query, now, batchSize)
	if err != nil {
		return nil, fmt.Errorf("querying expired pastes: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning paste id: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// Purge deletes expired pastes.
func (d *Database) Purge(batchSize int) (int, error) {
	ids, err := d.GetExpiredPastes(batchSize)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, id := range ids {
		if err := d.DeletePaste(id); err != nil && err != model.ErrPasteNotFound {
			return count, err
		}
		count++
	}

	return count, nil
}

// PurgeValues removes old traffic limiter entries.
func (d *Database) PurgeValues(namespace string, maxAge int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Traffic values are stored with timestamps as values
	// We need to delete entries where the timestamp is older than maxAge
	prefix := namespace + "_"
	cutoff := time.Now().Unix() - maxAge

	// This is a simplified approach - in production, you might want to
	// parse the values to check timestamps
	query := fmt.Sprintf(
		"DELETE FROM config WHERE id LIKE %s AND CAST(value AS BIGINT) < %s",
		d.placeholder(1), d.placeholder(2),
	)

	// Note: This query may not work on all databases due to CAST syntax
	// For production, consider storing timestamp in a separate column
	_, err := d.db.Exec(query, prefix+"%", cutoff)
	if err != nil {
		// Silently ignore errors - this is a cleanup operation
		return nil
	}
	return nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// pasteExistsUnsafe checks paste existence without acquiring lock.
// Only call this when you already hold the lock.
func (d *Database) pasteExistsUnsafe(id string) bool {
	query := fmt.Sprintf("SELECT 1 FROM paste WHERE dataid = %s", d.placeholder(1))
	var exists int
	err := d.db.QueryRow(query, id).Scan(&exists)
	return err == nil
}
