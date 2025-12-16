// Package model defines the Comment data structure for FlashPaper discussions.
// Comments are encrypted messages attached to pastes, allowing anonymous discussions.
// Like pastes, comment content is encrypted client-side, so the server only stores ciphertext.
package model

import (
	"encoding/json"
	"time"
)

// Comment represents an encrypted comment on a paste.
// Comments form a threaded discussion where each comment can have a parent
// (for replies) or be a top-level comment on the paste.
type Comment struct {
	// ID is the unique identifier (16 hex characters)
	ID string `json:"id"`

	// PasteID is the ID of the paste this comment belongs to
	PasteID string `json:"pasteid"`

	// ParentID is the ID of the parent comment (for replies)
	// Empty string for top-level comments on the paste
	ParentID string `json:"parentid,omitempty"`

	// Data is the encrypted comment content (base64-encoded ciphertext)
	Data string `json:"data"`

	// Meta contains comment metadata
	Meta CommentMeta `json:"meta,omitempty"`

	// AData (Authenticated Data) contains encryption parameters
	// Same format as paste AData for encryption verification
	AData json.RawMessage `json:"adata,omitempty"`

	// Version is the API version (currently 2)
	Version int `json:"v,omitempty"`

	// Vizhash is a visual hash of the commenter's IP for avatar generation
	// This allows anonymous identification without storing actual IPs
	Vizhash string `json:"vizhash,omitempty"`
}

// CommentMeta contains metadata about a comment.
type CommentMeta struct {
	// PostDate is the Unix timestamp when the comment was created
	PostDate int64 `json:"postdate,omitempty"`

	// Icon is the avatar/icon data for the commenter
	// Generated from vizhash for display purposes
	Icon string `json:"icon,omitempty"`

	// Nickname is an optional encrypted nickname
	Nickname string `json:"nickname,omitempty"`
}

// NewComment creates a new Comment with default values.
func NewComment(pasteID string) *Comment {
	return &Comment{
		PasteID: pasteID,
		Version: 2,
		Meta: CommentMeta{
			PostDate: time.Now().Unix(),
		},
	}
}

// Validate checks if the comment data is valid.
func (c *Comment) Validate() error {
	// PasteID is required
	if c.PasteID == "" {
		return ErrInvalidPasteID
	}

	// Data is required
	if c.Data == "" {
		return ErrCommentNotFound // Empty comment treated as invalid
	}

	return nil
}

// IsReply returns true if this comment is a reply to another comment.
func (c *Comment) IsReply() bool {
	return c.ParentID != ""
}

// ForStorage returns a copy of the comment suitable for storage.
func (c *Comment) ForStorage() *Comment {
	return &Comment{
		ID:       c.ID,
		PasteID:  c.PasteID,
		ParentID: c.ParentID,
		Data:     c.Data,
		AData:    c.AData,
		Version:  c.Version,
		Vizhash:  c.Vizhash,
		Meta: CommentMeta{
			PostDate: c.Meta.PostDate,
			Nickname: c.Meta.Nickname,
		},
	}
}

// ForResponse returns a copy of the comment suitable for API response.
func (c *Comment) ForResponse() *Comment {
	return &Comment{
		ID:       c.ID,
		PasteID:  c.PasteID,
		ParentID: c.ParentID,
		Data:     c.Data,
		AData:    c.AData,
		Version:  c.Version,
		Vizhash:  c.Vizhash,
		Meta: CommentMeta{
			PostDate: c.Meta.PostDate,
			Icon:     c.Meta.Icon,
			Nickname: c.Meta.Nickname,
		},
	}
}

// CommentThread represents a threaded view of comments.
// This is a helper structure for organizing comments into a tree.
type CommentThread struct {
	Comment  *Comment
	Replies  []*CommentThread
}

// BuildCommentTree organizes a flat list of comments into a threaded tree.
// Top-level comments (with empty ParentID) become roots, and replies are
// nested under their parent comments.
func BuildCommentTree(comments []*Comment) []*CommentThread {
	if len(comments) == 0 {
		return nil
	}

	// Create a map of comment ID to thread node
	nodes := make(map[string]*CommentThread)
	for _, c := range comments {
		nodes[c.ID] = &CommentThread{
			Comment: c,
			Replies: make([]*CommentThread, 0),
		}
	}

	// Build the tree structure
	var roots []*CommentThread
	for _, c := range comments {
		node := nodes[c.ID]
		if c.ParentID == "" || c.ParentID == c.PasteID {
			// Top-level comment (parent is the paste itself)
			roots = append(roots, node)
		} else if parent, ok := nodes[c.ParentID]; ok {
			// Reply to another comment
			parent.Replies = append(parent.Replies, node)
		} else {
			// Orphan comment (parent not found) - treat as top-level
			roots = append(roots, node)
		}
	}

	return roots
}

// FlattenCommentTree converts a tree of comments back to a flat list.
// Comments are returned in tree order (parent before children).
func FlattenCommentTree(threads []*CommentThread) []*Comment {
	var result []*Comment

	var flatten func([]*CommentThread)
	flatten = func(nodes []*CommentThread) {
		for _, node := range nodes {
			result = append(result, node.Comment)
			flatten(node.Replies)
		}
	}

	flatten(threads)
	return result
}
