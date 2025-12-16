package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewComment_HasDefaults(t *testing.T) {
	c := NewComment("paste123")

	assert.Equal(t, "paste123", c.PasteID)
	assert.Equal(t, 2, c.Version)
	assert.NotZero(t, c.Meta.PostDate)
}

func TestComment_Validate_ValidComment(t *testing.T) {
	c := &Comment{
		PasteID: "paste123",
		Data:    "encrypted comment",
	}

	err := c.Validate()
	assert.NoError(t, err)
}

func TestComment_Validate_MissingPasteID(t *testing.T) {
	c := &Comment{
		Data: "encrypted comment",
	}

	err := c.Validate()
	assert.ErrorIs(t, err, ErrInvalidPasteID)
}

func TestComment_Validate_EmptyData(t *testing.T) {
	c := &Comment{
		PasteID: "paste123",
		Data:    "",
	}

	err := c.Validate()
	assert.Error(t, err)
}

func TestComment_IsReply_TopLevel(t *testing.T) {
	c := &Comment{
		PasteID: "paste123",
	}

	assert.False(t, c.IsReply())
}

func TestComment_IsReply_Reply(t *testing.T) {
	c := &Comment{
		PasteID:  "paste123",
		ParentID: "comment456",
	}

	assert.True(t, c.IsReply())
}

func TestComment_ForStorage(t *testing.T) {
	c := &Comment{
		ID:       "comment123",
		PasteID:  "paste123",
		ParentID: "parent456",
		Data:     "encrypted",
		Vizhash:  "vizhash_data",
		Version:  2,
		Meta: CommentMeta{
			PostDate: time.Now().Unix(),
			Icon:     "icon_data",
			Nickname: "anon",
		},
	}

	stored := c.ForStorage()

	assert.Equal(t, c.ID, stored.ID)
	assert.Equal(t, c.PasteID, stored.PasteID)
	assert.Equal(t, c.ParentID, stored.ParentID)
	assert.Equal(t, c.Data, stored.Data)
	assert.Equal(t, c.Vizhash, stored.Vizhash)
	assert.Equal(t, c.Meta.PostDate, stored.Meta.PostDate)
	// Icon is not stored (it's generated from vizhash)
	assert.Empty(t, stored.Meta.Icon)
}

func TestComment_ForResponse(t *testing.T) {
	c := &Comment{
		ID:       "comment123",
		PasteID:  "paste123",
		ParentID: "parent456",
		Data:     "encrypted",
		Vizhash:  "vizhash_data",
		Version:  2,
		Meta: CommentMeta{
			PostDate: time.Now().Unix(),
			Icon:     "icon_data",
			Nickname: "anon",
		},
	}

	response := c.ForResponse()

	assert.Equal(t, c.ID, response.ID)
	assert.Equal(t, c.PasteID, response.PasteID)
	assert.Equal(t, c.Data, response.Data)
	assert.Equal(t, c.Meta.Icon, response.Meta.Icon)
}

func TestComment_JSONSerialization(t *testing.T) {
	c := &Comment{
		ID:       "comment123",
		PasteID:  "paste123",
		ParentID: "parent456",
		Data:     "encrypted_content",
		Version:  2,
		Vizhash:  "vizhash_data",
		Meta: CommentMeta{
			PostDate: 1234567890,
		},
	}

	// Serialize
	data, err := json.Marshal(c)
	require.NoError(t, err)

	// Deserialize
	var c2 Comment
	err = json.Unmarshal(data, &c2)
	require.NoError(t, err)

	assert.Equal(t, c.ID, c2.ID)
	assert.Equal(t, c.PasteID, c2.PasteID)
	assert.Equal(t, c.ParentID, c2.ParentID)
	assert.Equal(t, c.Data, c2.Data)
	assert.Equal(t, c.Vizhash, c2.Vizhash)
}

func TestBuildCommentTree_Empty(t *testing.T) {
	tree := BuildCommentTree(nil)
	assert.Nil(t, tree)
}

func TestBuildCommentTree_TopLevelOnly(t *testing.T) {
	comments := []*Comment{
		{ID: "c1", PasteID: "p1", Data: "comment 1"},
		{ID: "c2", PasteID: "p1", Data: "comment 2"},
		{ID: "c3", PasteID: "p1", Data: "comment 3"},
	}

	tree := BuildCommentTree(comments)

	assert.Len(t, tree, 3)
	for _, node := range tree {
		assert.Empty(t, node.Replies)
	}
}

func TestBuildCommentTree_WithReplies(t *testing.T) {
	comments := []*Comment{
		{ID: "c1", PasteID: "p1", Data: "top level"},
		{ID: "c2", PasteID: "p1", ParentID: "c1", Data: "reply to c1"},
		{ID: "c3", PasteID: "p1", ParentID: "c1", Data: "another reply to c1"},
		{ID: "c4", PasteID: "p1", ParentID: "c2", Data: "nested reply"},
	}

	tree := BuildCommentTree(comments)

	// Should have 1 top-level comment
	require.Len(t, tree, 1)
	assert.Equal(t, "c1", tree[0].Comment.ID)

	// c1 should have 2 replies
	require.Len(t, tree[0].Replies, 2)

	// Find c2 in replies
	var c2Node *CommentThread
	for _, r := range tree[0].Replies {
		if r.Comment.ID == "c2" {
			c2Node = r
			break
		}
	}
	require.NotNil(t, c2Node)

	// c2 should have 1 nested reply
	require.Len(t, c2Node.Replies, 1)
	assert.Equal(t, "c4", c2Node.Replies[0].Comment.ID)
}

func TestBuildCommentTree_ParentIsPaste(t *testing.T) {
	// In PrivateBin, top-level comments sometimes have ParentID set to the paste ID
	comments := []*Comment{
		{ID: "c1", PasteID: "p1", ParentID: "p1", Data: "top level"},
		{ID: "c2", PasteID: "p1", ParentID: "c1", Data: "reply"},
	}

	tree := BuildCommentTree(comments)

	require.Len(t, tree, 1)
	assert.Equal(t, "c1", tree[0].Comment.ID)
	require.Len(t, tree[0].Replies, 1)
	assert.Equal(t, "c2", tree[0].Replies[0].Comment.ID)
}

func TestBuildCommentTree_OrphanComments(t *testing.T) {
	// Comments with non-existent parents should become top-level
	comments := []*Comment{
		{ID: "c1", PasteID: "p1", ParentID: "nonexistent", Data: "orphan"},
	}

	tree := BuildCommentTree(comments)

	require.Len(t, tree, 1)
	assert.Equal(t, "c1", tree[0].Comment.ID)
}

func TestFlattenCommentTree_Empty(t *testing.T) {
	result := FlattenCommentTree(nil)
	assert.Nil(t, result)
}

func TestFlattenCommentTree_PreservesOrder(t *testing.T) {
	// Build a tree and flatten it
	comments := []*Comment{
		{ID: "c1", PasteID: "p1", Data: "top level"},
		{ID: "c2", PasteID: "p1", ParentID: "c1", Data: "reply"},
		{ID: "c3", PasteID: "p1", Data: "another top level"},
	}

	tree := BuildCommentTree(comments)
	flat := FlattenCommentTree(tree)

	// Should have all 3 comments
	assert.Len(t, flat, 3)

	// c1 should come before c2 (parent before child)
	c1Idx := -1
	c2Idx := -1
	for i, c := range flat {
		if c.ID == "c1" {
			c1Idx = i
		}
		if c.ID == "c2" {
			c2Idx = i
		}
	}
	assert.Less(t, c1Idx, c2Idx)
}

func TestFlattenCommentTree_DeepNesting(t *testing.T) {
	// Create deeply nested comments
	comments := []*Comment{
		{ID: "c1", PasteID: "p1", Data: "level 1"},
		{ID: "c2", PasteID: "p1", ParentID: "c1", Data: "level 2"},
		{ID: "c3", PasteID: "p1", ParentID: "c2", Data: "level 3"},
		{ID: "c4", PasteID: "p1", ParentID: "c3", Data: "level 4"},
	}

	tree := BuildCommentTree(comments)
	flat := FlattenCommentTree(tree)

	assert.Len(t, flat, 4)

	// Verify order: c1 -> c2 -> c3 -> c4
	assert.Equal(t, "c1", flat[0].ID)
	assert.Equal(t, "c2", flat[1].ID)
	assert.Equal(t, "c3", flat[2].ID)
	assert.Equal(t, "c4", flat[3].ID)
}

func TestCommentMeta_JSONOmitsEmpty(t *testing.T) {
	c := &Comment{
		ID:      "c1",
		PasteID: "p1",
		Data:    "encrypted",
		Meta:    CommentMeta{PostDate: 1234567890},
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	// Should not contain empty meta fields
	assert.NotContains(t, string(data), `"icon":""`)
	assert.NotContains(t, string(data), `"nickname":""`)
}
