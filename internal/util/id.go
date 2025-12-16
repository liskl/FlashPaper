// Package util provides ID generation utilities for FlashPaper.
// Paste and comment IDs follow the PrivateBin format: 16 hexadecimal characters
// (64 bits of entropy), which provides sufficient uniqueness for the use case
// while keeping URLs short and readable.
package util

import (
	"fmt"
	"regexp"
)

// IDLength is the length of paste and comment IDs in characters.
// PrivateBin uses 16 hex characters = 8 bytes = 64 bits of entropy.
const IDLength = 16

// idPattern validates the format of paste/comment IDs.
// IDs must be exactly 16 lowercase hexadecimal characters.
var idPattern = regexp.MustCompile(`^[a-f0-9]{16}$`)

// GenerateID creates a new unique paste or comment ID.
// The ID is 16 lowercase hexadecimal characters (64 bits of random data).
//
// Example output: "f468483c313401e8"
//
// Note: Callers should check for ID collisions in the storage layer
// before using the generated ID, as there is a (very small) probability
// of collision.
func GenerateID() (string, error) {
	// Generate 8 random bytes (64 bits)
	return RandomHex(IDLength / 2)
}

// ValidateID checks if an ID has the correct format.
// Valid IDs are exactly 16 lowercase hexadecimal characters.
//
// This validation prevents:
// - Path traversal attacks (../../../etc/passwd)
// - SQL injection (though we use parameterized queries)
// - Invalid storage paths
func ValidateID(id string) bool {
	return idPattern.MatchString(id)
}

// ValidateIDOrError returns an error if the ID is invalid.
// This is a convenience wrapper around ValidateID for use in handlers.
func ValidateIDOrError(id string) error {
	if !ValidateID(id) {
		return fmt.Errorf("invalid ID format: must be 16 hexadecimal characters")
	}
	return nil
}

// MustGenerateID generates an ID or panics if it fails.
// This is useful for test code but should not be used in production.
func MustGenerateID() string {
	id, err := GenerateID()
	if err != nil {
		panic(fmt.Sprintf("failed to generate ID: %v", err))
	}
	return id
}
