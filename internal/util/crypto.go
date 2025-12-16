// Package util provides cryptographic utilities and helper functions for FlashPaper.
// These utilities are used by the storage and handler layers for secure operations
// like generating delete tokens and server-side salts.
//
// Security note: All cryptographic operations use Go's crypto/rand for random
// number generation, which is cryptographically secure. The HMAC operations
// use SHA-256 which provides adequate security for the use cases here.
package util

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// SaltSize is the number of random bytes in a server salt.
// 32 bytes (256 bits) provides sufficient entropy for HMAC keying.
const SaltSize = 32

// GenerateSalt creates a new cryptographically random salt.
// The salt is returned as a base64-encoded string for storage convenience.
// This salt is used as the HMAC key for generating delete tokens.
func GenerateSalt() (string, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating random salt: %w", err)
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

// GenerateDeleteToken creates an HMAC-SHA256 token for paste deletion.
// The token is derived from the paste ID and server salt, allowing
// only users who know the token to delete the paste.
//
// Format: hex(HMAC-SHA256(pasteID, salt))
//
// This mirrors PrivateBin's deletion mechanism where the delete token
// is returned to the client at paste creation time.
func GenerateDeleteToken(pasteID, salt string) (string, error) {
	// Decode the base64 salt
	saltBytes, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		return "", fmt.Errorf("decoding salt: %w", err)
	}

	// Create HMAC-SHA256 of the paste ID
	h := hmac.New(sha256.New, saltBytes)
	h.Write([]byte(pasteID))
	token := h.Sum(nil)

	return hex.EncodeToString(token), nil
}

// ValidateDeleteToken securely compares a provided token against the expected token.
// Uses constant-time comparison to prevent timing attacks.
//
// Returns true if the token is valid, false otherwise.
func ValidateDeleteToken(providedToken, pasteID, salt string) bool {
	expectedToken, err := GenerateDeleteToken(pasteID, salt)
	if err != nil {
		return false
	}

	// Use constant-time comparison to prevent timing attacks
	// An attacker cannot determine how many bytes match based on response time
	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) == 1
}

// GenerateVizhash creates a visual hash for anonymous comment attribution.
// The hash is derived from the commenter's IP address and server salt,
// allowing consistent avatar display without storing the actual IP.
//
// Format: base64(SHA512-HMAC(ip, salt))
//
// This mirrors PrivateBin's vizhash mechanism for comment icons.
func GenerateVizhash(ip, salt string) (string, error) {
	// Decode the base64 salt
	saltBytes, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		return "", fmt.Errorf("decoding salt: %w", err)
	}

	// Create HMAC-SHA512 of the IP address
	// SHA-512 is used because the vizhash generates visual patterns
	// that benefit from the longer hash output
	h := hmac.New(sha512.New, saltBytes)
	h.Write([]byte(ip))
	hash := h.Sum(nil)

	return base64.StdEncoding.EncodeToString(hash), nil
}

// HashIP creates a SHA-256 hash of an IP address for rate limiting storage.
// The hash allows tracking rate limits per IP without storing raw IP addresses.
//
// Format: hex(SHA256(ip + salt))
func HashIP(ip, salt string) string {
	h := sha256.New()
	h.Write([]byte(ip))
	h.Write([]byte(salt))
	return hex.EncodeToString(h.Sum(nil))
}

// RandomBytes generates n cryptographically random bytes.
// Returns error if the system's random number generator fails.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating random bytes: %w", err)
	}
	return b, nil
}

// RandomHex generates a random hexadecimal string of the specified byte length.
// The returned string will be 2*n characters long (two hex chars per byte).
func RandomHex(n int) (string, error) {
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
