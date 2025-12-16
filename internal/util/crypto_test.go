package util

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSalt_ReturnsValidBase64(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(salt)
	require.NoError(t, err)

	// Should be SaltSize bytes when decoded
	assert.Len(t, decoded, SaltSize)
}

func TestGenerateSalt_ReturnsUniqueSalts(t *testing.T) {
	salts := make(map[string]bool)

	// Generate multiple salts and verify uniqueness
	for i := 0; i < 100; i++ {
		salt, err := GenerateSalt()
		require.NoError(t, err)

		// Should not have seen this salt before
		assert.False(t, salts[salt], "Generated duplicate salt")
		salts[salt] = true
	}
}

func TestGenerateDeleteToken_ReturnsConsistentToken(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	pasteID := "f468483c313401e8"

	// Generate token multiple times with same inputs
	token1, err := GenerateDeleteToken(pasteID, salt)
	require.NoError(t, err)

	token2, err := GenerateDeleteToken(pasteID, salt)
	require.NoError(t, err)

	// Same inputs should produce same token
	assert.Equal(t, token1, token2)
}

func TestGenerateDeleteToken_DifferentPasteIDs_DifferentTokens(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	token1, err := GenerateDeleteToken("paste1", salt)
	require.NoError(t, err)

	token2, err := GenerateDeleteToken("paste2", salt)
	require.NoError(t, err)

	// Different paste IDs should produce different tokens
	assert.NotEqual(t, token1, token2)
}

func TestGenerateDeleteToken_DifferentSalts_DifferentTokens(t *testing.T) {
	salt1, err := GenerateSalt()
	require.NoError(t, err)

	salt2, err := GenerateSalt()
	require.NoError(t, err)

	pasteID := "samepaste"

	token1, err := GenerateDeleteToken(pasteID, salt1)
	require.NoError(t, err)

	token2, err := GenerateDeleteToken(pasteID, salt2)
	require.NoError(t, err)

	// Different salts should produce different tokens
	assert.NotEqual(t, token1, token2)
}

func TestGenerateDeleteToken_ReturnsValidHex(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	token, err := GenerateDeleteToken("testpaste", salt)
	require.NoError(t, err)

	// Should be valid hex
	decoded, err := hex.DecodeString(token)
	require.NoError(t, err)

	// SHA-256 produces 32 bytes = 64 hex characters
	assert.Len(t, decoded, 32)
	assert.Len(t, token, 64)
}

func TestGenerateDeleteToken_InvalidSalt_ReturnsError(t *testing.T) {
	_, err := GenerateDeleteToken("testpaste", "not-valid-base64!!!")
	assert.Error(t, err)
}

func TestValidateDeleteToken_ValidToken_ReturnsTrue(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	pasteID := "f468483c313401e8"
	token, err := GenerateDeleteToken(pasteID, salt)
	require.NoError(t, err)

	// Validation should succeed with correct token
	valid := ValidateDeleteToken(token, pasteID, salt)
	assert.True(t, valid)
}

func TestValidateDeleteToken_InvalidToken_ReturnsFalse(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	pasteID := "f468483c313401e8"

	// Wrong token should fail validation
	valid := ValidateDeleteToken("wrongtoken", pasteID, salt)
	assert.False(t, valid)
}

func TestValidateDeleteToken_WrongPasteID_ReturnsFalse(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	token, err := GenerateDeleteToken("paste1", salt)
	require.NoError(t, err)

	// Token for different paste ID should fail
	valid := ValidateDeleteToken(token, "paste2", salt)
	assert.False(t, valid)
}

func TestValidateDeleteToken_WrongSalt_ReturnsFalse(t *testing.T) {
	salt1, err := GenerateSalt()
	require.NoError(t, err)

	salt2, err := GenerateSalt()
	require.NoError(t, err)

	pasteID := "testpaste"
	token, err := GenerateDeleteToken(pasteID, salt1)
	require.NoError(t, err)

	// Token validated with different salt should fail
	valid := ValidateDeleteToken(token, pasteID, salt2)
	assert.False(t, valid)
}

func TestValidateDeleteToken_InvalidSalt_ReturnsFalse(t *testing.T) {
	// Should not panic, just return false
	valid := ValidateDeleteToken("sometoken", "pasteid", "invalid-base64!!!")
	assert.False(t, valid)
}

func TestGenerateVizhash_ReturnsConsistentHash(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	ip := "192.168.1.100"

	hash1, err := GenerateVizhash(ip, salt)
	require.NoError(t, err)

	hash2, err := GenerateVizhash(ip, salt)
	require.NoError(t, err)

	// Same inputs should produce same hash
	assert.Equal(t, hash1, hash2)
}

func TestGenerateVizhash_DifferentIPs_DifferentHashes(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	hash1, err := GenerateVizhash("192.168.1.100", salt)
	require.NoError(t, err)

	hash2, err := GenerateVizhash("192.168.1.101", salt)
	require.NoError(t, err)

	// Different IPs should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func TestGenerateVizhash_ReturnsValidBase64(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)

	hash, err := GenerateVizhash("192.168.1.100", salt)
	require.NoError(t, err)

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(hash)
	require.NoError(t, err)

	// SHA-512 produces 64 bytes
	assert.Len(t, decoded, 64)
}

func TestGenerateVizhash_InvalidSalt_ReturnsError(t *testing.T) {
	_, err := GenerateVizhash("192.168.1.100", "not-valid-base64!!!")
	assert.Error(t, err)
}

func TestHashIP_ReturnsConsistentHash(t *testing.T) {
	salt := "testsalt"
	ip := "192.168.1.100"

	hash1 := HashIP(ip, salt)
	hash2 := HashIP(ip, salt)

	assert.Equal(t, hash1, hash2)
}

func TestHashIP_DifferentIPs_DifferentHashes(t *testing.T) {
	salt := "testsalt"

	hash1 := HashIP("192.168.1.100", salt)
	hash2 := HashIP("192.168.1.101", salt)

	assert.NotEqual(t, hash1, hash2)
}

func TestHashIP_DifferentSalts_DifferentHashes(t *testing.T) {
	ip := "192.168.1.100"

	hash1 := HashIP(ip, "salt1")
	hash2 := HashIP(ip, "salt2")

	assert.NotEqual(t, hash1, hash2)
}

func TestHashIP_ReturnsValidHex(t *testing.T) {
	hash := HashIP("192.168.1.100", "salt")

	// Should be valid hex
	decoded, err := hex.DecodeString(hash)
	require.NoError(t, err)

	// SHA-256 produces 32 bytes = 64 hex characters
	assert.Len(t, decoded, 32)
	assert.Len(t, hash, 64)
}

func TestRandomBytes_ReturnsRequestedLength(t *testing.T) {
	lengths := []int{8, 16, 32, 64, 128}

	for _, length := range lengths {
		t.Run(string(rune(length)), func(t *testing.T) {
			b, err := RandomBytes(length)
			require.NoError(t, err)
			assert.Len(t, b, length)
		})
	}
}

func TestRandomBytes_ReturnsUniqueBytes(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		b, err := RandomBytes(16)
		require.NoError(t, err)

		key := string(b)
		assert.False(t, seen[key], "Generated duplicate bytes")
		seen[key] = true
	}
}

func TestRandomHex_ReturnsCorrectLength(t *testing.T) {
	// RandomHex returns 2 hex characters per byte
	lengths := []int{4, 8, 16, 32}

	for _, length := range lengths {
		t.Run(string(rune(length)), func(t *testing.T) {
			h, err := RandomHex(length)
			require.NoError(t, err)
			assert.Len(t, h, length*2) // 2 hex chars per byte
		})
	}
}

func TestRandomHex_ReturnsValidHex(t *testing.T) {
	h, err := RandomHex(16)
	require.NoError(t, err)

	// Should be valid hex
	_, err = hex.DecodeString(h)
	require.NoError(t, err)
}

func TestRandomHex_ReturnsUniqueStrings(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		h, err := RandomHex(8)
		require.NoError(t, err)

		assert.False(t, seen[h], "Generated duplicate hex string")
		seen[h] = true
	}
}

// Benchmark constant-time comparison
func BenchmarkValidateDeleteToken(b *testing.B) {
	salt, _ := GenerateSalt()
	pasteID := "f468483c313401e8"
	token, _ := GenerateDeleteToken(pasteID, salt)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateDeleteToken(token, pasteID, salt)
	}
}
