// Package model defines the Paste data structure and related operations.
// Pastes are the core data unit in FlashPaper - they contain encrypted content
// that is stored on the server but can only be decrypted client-side.
package model

import (
	"encoding/json"
	"time"
)

// Formatter constants define how paste content should be displayed.
// These values are sent to the client to determine rendering behavior.
const (
	FormatterPlainText       = "plaintext"
	FormatterSyntaxHighlight = "syntaxhighlighting"
	FormatterMarkdown        = "markdown"
)

// Paste represents an encrypted paste stored in FlashPaper.
// The actual content is encrypted client-side using AES-256-GCM,
// so the server only sees ciphertext and metadata.
type Paste struct {
	// ID is the unique identifier (16 hex characters)
	ID string `json:"id,omitempty"`

	// Data is the encrypted paste content (base64-encoded ciphertext)
	// This is the "ct" field in PrivateBin's API
	Data string `json:"data"`

	// AttachmentName is the filename of an attached file (if any)
	AttachmentName string `json:"attachmentname,omitempty"`

	// Attachment is the encrypted attachment content (base64-encoded)
	Attachment string `json:"attachment,omitempty"`

	// Meta contains paste metadata (expiration, burn settings, etc.)
	Meta PasteMeta `json:"meta"`

	// AData (Authenticated Data) contains encryption parameters and settings
	// This is transmitted alongside the ciphertext and verified during decryption
	// Format: [[iv, salt, iterations, keysize, tagsize, algo, mode, compression], formatter, opendiscussion, burnafterreading]
	AData json.RawMessage `json:"adata"`

	// Version is the API version (currently 2 for PrivateBin compatibility)
	Version int `json:"v,omitempty"`

	// DeleteToken is the token needed to delete this paste
	// Only returned on paste creation, not on retrieval
	DeleteToken string `json:"deletetoken,omitempty"`

	// URL is the paste URL (returned on creation)
	URL string `json:"url,omitempty"`

	// Comments contains all comments on this paste (populated on retrieval)
	Comments []*Comment `json:"comments,omitempty"`

	// CommentCount is the total number of comments
	CommentCount int `json:"comment_count,omitempty"`

	// CommentOffset is used for comment pagination
	CommentOffset int `json:"comment_offset,omitempty"`
}

// PasteMeta contains metadata about a paste.
// Some fields are only stored server-side, others are returned to clients.
type PasteMeta struct {
	// PostDate is the Unix timestamp when the paste was created
	PostDate int64 `json:"postdate,omitempty"`

	// ExpireDate is the Unix timestamp when the paste expires (0 = never)
	ExpireDate int64 `json:"expire_date,omitempty"`

	// Expire is the expiration option string (e.g., "1day", "1week")
	// Used during creation, not stored
	Expire string `json:"expire,omitempty"`

	// BurnAfterReading indicates the paste should be deleted after first view
	BurnAfterReading bool `json:"burnafterreading,omitempty"`

	// OpenDiscussion indicates if comments are enabled
	OpenDiscussion bool `json:"opendiscussion,omitempty"`

	// Formatter specifies how to render the paste (plaintext, syntaxhighlighting, markdown)
	Formatter string `json:"formatter,omitempty"`

	// Salt is the server-side salt used for delete token generation
	// Never exposed to clients
	Salt string `json:"-"`

	// TimeToLive is used during creation to specify expiration
	TimeToLive int64 `json:"time_to_live,omitempty"`
}

// NewPaste creates a new Paste with default values.
func NewPaste() *Paste {
	return &Paste{
		Version: 2,
		Meta: PasteMeta{
			PostDate:  time.Now().Unix(),
			Formatter: FormatterPlainText,
		},
	}
}

// IsExpired checks if the paste has passed its expiration time.
// Pastes with ExpireDate of 0 never expire.
func (p *Paste) IsExpired() bool {
	if p.Meta.ExpireDate == 0 {
		return false // Never expires
	}
	return time.Now().Unix() > p.Meta.ExpireDate
}

// IsBurnAfterReading returns true if the paste should be deleted after reading.
func (p *Paste) IsBurnAfterReading() bool {
	return p.Meta.BurnAfterReading
}

// HasDiscussion returns true if discussions (comments) are enabled.
func (p *Paste) HasDiscussion() bool {
	return p.Meta.OpenDiscussion
}

// Validate checks if the paste data is valid.
// This should be called before storing a new paste.
func (p *Paste) Validate() error {
	// Data is required
	if p.Data == "" {
		return ErrPasteTooLarge // Empty paste treated as invalid
	}

	// Validate formatter if specified
	if p.Meta.Formatter != "" {
		switch p.Meta.Formatter {
		case FormatterPlainText, FormatterSyntaxHighlight, FormatterMarkdown:
			// Valid
		default:
			return ErrInvalidFormatter
		}
	}

	// Cannot have both burn-after-reading and discussion enabled
	// This would create a logical conflict: discussion requires persistence,
	// but burn-after-reading deletes on first view
	if p.Meta.BurnAfterReading && p.Meta.OpenDiscussion {
		return ErrBurnAfterReadingWithDiscussion
	}

	return nil
}

// SetExpiration sets the expiration time based on duration.
// A duration of 0 means the paste never expires.
func (p *Paste) SetExpiration(d time.Duration) {
	if d == 0 {
		p.Meta.ExpireDate = 0 // Never expires
	} else {
		p.Meta.ExpireDate = time.Now().Add(d).Unix()
	}
}

// ForStorage returns a copy of the paste suitable for storage.
// This removes fields that shouldn't be persisted (like URL, comments).
func (p *Paste) ForStorage() *Paste {
	return &Paste{
		ID:             p.ID,
		Data:           p.Data,
		AttachmentName: p.AttachmentName,
		Attachment:     p.Attachment,
		AData:          p.AData,
		Version:        p.Version,
		Meta: PasteMeta{
			PostDate:         p.Meta.PostDate,
			ExpireDate:       p.Meta.ExpireDate,
			BurnAfterReading: p.Meta.BurnAfterReading,
			OpenDiscussion:   p.Meta.OpenDiscussion,
			Formatter:        p.Meta.Formatter,
			Salt:             p.Meta.Salt,
		},
	}
}

// ForResponse returns a copy of the paste suitable for API response.
// This removes sensitive server-side fields (like salt).
func (p *Paste) ForResponse() *Paste {
	return &Paste{
		ID:             p.ID,
		Data:           p.Data,
		AttachmentName: p.AttachmentName,
		Attachment:     p.Attachment,
		AData:          p.AData,
		Version:        p.Version,
		URL:            p.URL,
		Comments:       p.Comments,
		CommentCount:   p.CommentCount,
		CommentOffset:  p.CommentOffset,
		Meta: PasteMeta{
			PostDate:         p.Meta.PostDate,
			BurnAfterReading: p.Meta.BurnAfterReading,
			OpenDiscussion:   p.Meta.OpenDiscussion,
			Formatter:        p.Meta.Formatter,
			// Note: ExpireDate and Salt are NOT included
		},
	}
}

// ParseAData extracts formatter, opendiscussion, and burnafterreading from AData.
// AData format: [[encryption params], formatter, opendiscussion, burnafterreading]
// This is used when AData is provided but individual meta fields are not.
func (p *Paste) ParseAData() error {
	if len(p.AData) == 0 {
		return nil
	}

	// AData is an array: [spec, formatter, opendiscussion, burnafterreading]
	var adata []interface{}
	if err := json.Unmarshal(p.AData, &adata); err != nil {
		return err
	}

	if len(adata) < 4 {
		return nil // Not enough elements
	}

	// Index 1: formatter (string)
	if formatter, ok := adata[1].(string); ok {
		p.Meta.Formatter = formatter
	}

	// Index 2: opendiscussion (number, 0 or 1)
	if discussion, ok := adata[2].(float64); ok {
		p.Meta.OpenDiscussion = discussion == 1
	}

	// Index 3: burnafterreading (number, 0 or 1)
	if burn, ok := adata[3].(float64); ok {
		p.Meta.BurnAfterReading = burn == 1
	}

	return nil
}
