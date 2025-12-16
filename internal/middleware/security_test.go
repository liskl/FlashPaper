// Package middleware provides tests for HTTP middleware.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liskl/flashpaper/internal/config"
)

// TestSecurityHeaders tests that all security headers are set correctly.
func TestSecurityHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	middleware := SecurityHeaders(cfg)

	// Create a simple handler that just returns 200 OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the handler with the security middleware
	wrapped := middleware(handler)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// Execute the request
	wrapped.ServeHTTP(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Test all expected security headers
	tests := []struct {
		header   string
		expected string
	}{
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "no-referrer"},
		{"Cache-Control", "no-store, no-cache, must-revalidate"},
		{"Pragma", "no-cache"},
		{"Permissions-Policy", "geolocation=(), microphone=(), camera=()"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := rr.Header().Get(tt.header)
			if got != tt.expected {
				t.Errorf("%s: expected %q, got %q", tt.header, tt.expected, got)
			}
		})
	}

	// Check CSP header contains expected directives
	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header is missing")
	}

	// Verify key CSP directives are present
	cspDirectives := []string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self'",
		"frame-ancestors 'none'",
	}

	for _, directive := range cspDirectives {
		if !containsSubstring(csp, directive) {
			t.Errorf("CSP missing directive: %s", directive)
		}
	}
}

// TestSecurityHeaders_ChainedHandlers tests that the middleware properly chains.
func TestSecurityHeaders_ChainedHandlers(t *testing.T) {
	cfg := config.DefaultConfig()
	middleware := SecurityHeaders(cfg)

	// Track if inner handler was called
	handlerCalled := false

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("test response"))
	})

	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/paste", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	// Verify inner handler was called
	if !handlerCalled {
		t.Error("inner handler was not called")
	}

	// Verify response from inner handler is preserved
	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	if rr.Body.String() != "test response" {
		t.Errorf("expected body 'test response', got %q", rr.Body.String())
	}

	// Security headers should still be set
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options header not set")
	}
}

// TestSecurityHeaders_DifferentMethods tests middleware works with different HTTP methods.
func TestSecurityHeaders_DifferentMethods(t *testing.T) {
	cfg := config.DefaultConfig()
	middleware := SecurityHeaders(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware(handler)

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()

			wrapped.ServeHTTP(rr, req)

			// All methods should get security headers
			if rr.Header().Get("X-Frame-Options") != "DENY" {
				t.Errorf("%s: X-Frame-Options not set", method)
			}
		})
	}
}

// containsSubstring checks if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
