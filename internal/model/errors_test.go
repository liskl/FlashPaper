// Package model provides tests for error helper functions.
package model

import (
	"errors"
	"fmt"
	"testing"
)

// TestIsNotFound tests the IsNotFound error classifier.
func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrPasteNotFound", ErrPasteNotFound, true},
		{"ErrCommentNotFound", ErrCommentNotFound, true},
		{"wrapped ErrPasteNotFound", fmt.Errorf("wrapper: %w", ErrPasteNotFound), true},
		{"wrapped ErrCommentNotFound", fmt.Errorf("wrapper: %w", ErrCommentNotFound), true},
		{"ErrPasteExists", ErrPasteExists, false},
		{"ErrRateLimited", ErrRateLimited, false},
		{"generic error", errors.New("some error"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFound(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestIsConflict tests the IsConflict error classifier.
func TestIsConflict(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrPasteExists", ErrPasteExists, true},
		{"ErrCommentExists", ErrCommentExists, true},
		{"wrapped ErrPasteExists", fmt.Errorf("wrapper: %w", ErrPasteExists), true},
		{"wrapped ErrCommentExists", fmt.Errorf("wrapper: %w", ErrCommentExists), true},
		{"ErrPasteNotFound", ErrPasteNotFound, false},
		{"ErrRateLimited", ErrRateLimited, false},
		{"generic error", errors.New("some error"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConflict(tt.err)
			if result != tt.expected {
				t.Errorf("IsConflict(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestIsValidationError tests the IsValidationError error classifier.
func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrInvalidPasteID", ErrInvalidPasteID, true},
		{"ErrInvalidCommentID", ErrInvalidCommentID, true},
		{"ErrPasteTooLarge", ErrPasteTooLarge, true},
		{"ErrInvalidExpiration", ErrInvalidExpiration, true},
		{"ErrInvalidFormatter", ErrInvalidFormatter, true},
		{"ErrBurnAfterReadingWithDiscussion", ErrBurnAfterReadingWithDiscussion, true},
		{"wrapped ErrInvalidPasteID", fmt.Errorf("wrapper: %w", ErrInvalidPasteID), true},
		{"ErrPasteNotFound", ErrPasteNotFound, false},
		{"ErrRateLimited", ErrRateLimited, false},
		{"generic error", errors.New("some error"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidationError(tt.err)
			if result != tt.expected {
				t.Errorf("IsValidationError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestIsForbidden tests the IsForbidden error classifier.
func TestIsForbidden(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrInvalidDeleteToken", ErrInvalidDeleteToken, true},
		{"ErrDiscussionDisabled", ErrDiscussionDisabled, true},
		{"ErrDiscussionClosed", ErrDiscussionClosed, true},
		{"wrapped ErrInvalidDeleteToken", fmt.Errorf("wrapper: %w", ErrInvalidDeleteToken), true},
		{"wrapped ErrDiscussionDisabled", fmt.Errorf("wrapper: %w", ErrDiscussionDisabled), true},
		{"ErrPasteNotFound", ErrPasteNotFound, false},
		{"ErrRateLimited", ErrRateLimited, false},
		{"generic error", errors.New("some error"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsForbidden(tt.err)
			if result != tt.expected {
				t.Errorf("IsForbidden(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestIsTooManyRequests tests the IsTooManyRequests error classifier.
func TestIsTooManyRequests(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrRateLimited", ErrRateLimited, true},
		{"wrapped ErrRateLimited", fmt.Errorf("wrapper: %w", ErrRateLimited), true},
		{"ErrPasteNotFound", ErrPasteNotFound, false},
		{"ErrInvalidDeleteToken", ErrInvalidDeleteToken, false},
		{"generic error", errors.New("some error"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTooManyRequests(tt.err)
			if result != tt.expected {
				t.Errorf("IsTooManyRequests(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestErrorMessages tests that all error messages are non-empty and unique.
func TestErrorMessages(t *testing.T) {
	allErrors := []error{
		ErrPasteNotFound,
		ErrPasteExpired,
		ErrPasteExists,
		ErrCommentNotFound,
		ErrCommentExists,
		ErrDiscussionDisabled,
		ErrDiscussionClosed,
		ErrInvalidDeleteToken,
		ErrInvalidPasteID,
		ErrInvalidCommentID,
		ErrPasteTooLarge,
		ErrRateLimited,
		ErrInvalidExpiration,
		ErrInvalidFormatter,
		ErrStorageFailure,
		ErrBurnAfterReadingWithDiscussion,
	}

	seen := make(map[string]bool)
	for _, err := range allErrors {
		msg := err.Error()

		// Check non-empty
		if msg == "" {
			t.Errorf("error has empty message: %v", err)
		}

		// Check unique
		if seen[msg] {
			t.Errorf("duplicate error message: %s", msg)
		}
		seen[msg] = true
	}
}
