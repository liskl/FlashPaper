package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateID_ReturnsCorrectLength(t *testing.T) {
	id, err := GenerateID()
	require.NoError(t, err)
	assert.Len(t, id, IDLength)
}

func TestGenerateID_ReturnsLowercaseHex(t *testing.T) {
	id, err := GenerateID()
	require.NoError(t, err)

	// Should pass validation (which checks for lowercase hex)
	assert.True(t, ValidateID(id))

	// Double-check: only lowercase hex characters
	for _, c := range id {
		isLowerHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		assert.True(t, isLowerHex, "Character %c is not lowercase hex", c)
	}
}

func TestGenerateID_ReturnsUniqueIDs(t *testing.T) {
	seen := make(map[string]bool)

	// Generate many IDs and check uniqueness
	for i := 0; i < 1000; i++ {
		id, err := GenerateID()
		require.NoError(t, err)

		assert.False(t, seen[id], "Generated duplicate ID: %s", id)
		seen[id] = true
	}
}

func TestValidateID_ValidIDs(t *testing.T) {
	validIDs := []string{
		"f468483c313401e8",
		"0000000000000000",
		"ffffffffffffffff",
		"abcdef0123456789",
		"1234567890abcdef",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			assert.True(t, ValidateID(id))
		})
	}
}

func TestValidateID_InvalidIDs(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"too short", "f468483c313401e"},
		{"too long", "f468483c313401e8a"},
		{"uppercase", "F468483C313401E8"},
		{"mixed case", "f468483C313401e8"},
		{"invalid chars", "g468483c313401e8"},
		{"special chars", "f468483c31340!e8"},
		{"spaces", "f468483c 13401e8"},
		{"path traversal", "../../../etc/pas"},
		{"sql injection", "'; DROP TABLE--"},
		{"unicode", "f468483c313401é8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, ValidateID(tt.id))
		})
	}
}

func TestValidateIDOrError_ValidID_ReturnsNil(t *testing.T) {
	err := ValidateIDOrError("f468483c313401e8")
	assert.NoError(t, err)
}

func TestValidateIDOrError_InvalidID_ReturnsError(t *testing.T) {
	err := ValidateIDOrError("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ID format")
}

func TestMustGenerateID_ReturnsValidID(t *testing.T) {
	// Should not panic
	id := MustGenerateID()
	assert.Len(t, id, IDLength)
	assert.True(t, ValidateID(id))
}

func TestGenerateID_GeneratesDistributedIDs(t *testing.T) {
	// Check that IDs have good distribution of first characters
	// This is important for filesystem storage which uses first chars for directory names
	firstChars := make(map[byte]int)

	for i := 0; i < 1000; i++ {
		id, err := GenerateID()
		require.NoError(t, err)
		firstChars[id[0]]++
	}

	// Should have at least some variety in first characters
	// With 16 possible hex chars, we should see multiple different ones
	assert.Greater(t, len(firstChars), 8, "First characters should be well-distributed")
}

// Benchmark ID generation
func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateID()
	}
}

// Benchmark ID validation
func BenchmarkValidateID(b *testing.B) {
	id := "f468483c313401e8"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateID(id)
	}
}
