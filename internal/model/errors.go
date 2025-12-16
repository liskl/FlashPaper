// Package model defines domain models and errors for FlashPaper.
// This package contains the core data structures (Paste, Comment) and
// custom error types used throughout the application.
package model

import "errors"

// Domain-specific errors for FlashPaper operations.
// These errors allow handlers to return appropriate HTTP status codes
// and messages to clients.
var (
	// ErrPasteNotFound is returned when a requested paste doesn't exist
	ErrPasteNotFound = errors.New("paste not found")

	// ErrPasteExpired is returned when a paste has passed its expiration time
	ErrPasteExpired = errors.New("paste has expired")

	// ErrPasteExists is returned when trying to create a paste with an existing ID
	ErrPasteExists = errors.New("paste already exists")

	// ErrCommentNotFound is returned when a requested comment doesn't exist
	ErrCommentNotFound = errors.New("comment not found")

	// ErrCommentExists is returned when trying to create a comment with an existing ID
	ErrCommentExists = errors.New("comment already exists")

	// ErrDiscussionDisabled is returned when trying to comment on a paste
	// that has discussions disabled
	ErrDiscussionDisabled = errors.New("discussion is disabled for this paste")

	// ErrDiscussionClosed is returned when the paste's configuration doesn't allow
	// new comments (e.g., burn-after-reading paste)
	ErrDiscussionClosed = errors.New("discussion is closed")

	// ErrInvalidDeleteToken is returned when the provided delete token doesn't match
	ErrInvalidDeleteToken = errors.New("invalid delete token")

	// ErrInvalidPasteID is returned when a paste ID fails validation
	ErrInvalidPasteID = errors.New("invalid paste ID format")

	// ErrInvalidCommentID is returned when a comment ID fails validation
	ErrInvalidCommentID = errors.New("invalid comment ID format")

	// ErrPasteTooLarge is returned when paste content exceeds the size limit
	ErrPasteTooLarge = errors.New("paste exceeds maximum size limit")

	// ErrRateLimited is returned when a client has exceeded the rate limit
	ErrRateLimited = errors.New("rate limit exceeded, please try again later")

	// ErrInvalidExpiration is returned when an invalid expiration option is provided
	ErrInvalidExpiration = errors.New("invalid expiration option")

	// ErrInvalidFormatter is returned when an invalid formatter is specified
	ErrInvalidFormatter = errors.New("invalid formatter")

	// ErrStorageFailure is returned when the storage backend encounters an error
	ErrStorageFailure = errors.New("storage operation failed")

	// ErrBurnAfterReadingWithDiscussion is returned when trying to enable both
	// burn-after-reading and discussion on the same paste
	ErrBurnAfterReadingWithDiscussion = errors.New("burn-after-reading and discussion cannot both be enabled")
)

// IsNotFound returns true if the error indicates a resource was not found.
// This is useful for handlers to determine the appropriate HTTP status code.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrPasteNotFound) || errors.Is(err, ErrCommentNotFound)
}

// IsConflict returns true if the error indicates a resource conflict.
// This is useful for handlers to return HTTP 409 Conflict.
func IsConflict(err error) bool {
	return errors.Is(err, ErrPasteExists) || errors.Is(err, ErrCommentExists)
}

// IsValidationError returns true if the error is due to invalid input.
// This is useful for handlers to return HTTP 400 Bad Request.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidPasteID) ||
		errors.Is(err, ErrInvalidCommentID) ||
		errors.Is(err, ErrPasteTooLarge) ||
		errors.Is(err, ErrInvalidExpiration) ||
		errors.Is(err, ErrInvalidFormatter) ||
		errors.Is(err, ErrBurnAfterReadingWithDiscussion)
}

// IsForbidden returns true if the error indicates an operation is not allowed.
// This is useful for handlers to return HTTP 403 Forbidden.
func IsForbidden(err error) bool {
	return errors.Is(err, ErrInvalidDeleteToken) ||
		errors.Is(err, ErrDiscussionDisabled) ||
		errors.Is(err, ErrDiscussionClosed)
}

// IsTooManyRequests returns true if the error indicates rate limiting.
// This is useful for handlers to return HTTP 429 Too Many Requests.
func IsTooManyRequests(err error) bool {
	return errors.Is(err, ErrRateLimited)
}
