// Package telemetry provides tests for OpenTelemetry tracing instrumentation.
package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/liskl/flashpaper/internal/model"
	"github.com/liskl/flashpaper/internal/storage"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "localhost:4317", cfg.Endpoint)
	assert.Equal(t, "flashpaper", cfg.ServiceName)
	assert.Equal(t, "dev", cfg.Version)
	assert.Equal(t, "development", cfg.Environment)
	assert.True(t, cfg.Insecure)
	assert.Equal(t, 1.0, cfg.SampleRate)
}

func TestNewProvider_Disabled(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled: false,
	}

	provider, err := NewProvider(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Should return a no-op tracer provider
	tp := provider.TracerProvider()
	assert.NotNil(t, tp)

	// Tracer should work without panicking
	tracer := provider.Tracer("test")
	assert.NotNil(t, tracer)

	// Shutdown should succeed
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestNewProvider_ShutdownWithDeadline(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled: false,
	}

	provider, err := NewProvider(ctx, cfg)
	require.NoError(t, err)

	// Shutdown with a context that has no deadline - should add one
	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestProvider_TracerProvider(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled: false,
	}

	provider, err := NewProvider(ctx, cfg)
	require.NoError(t, err)

	tp := provider.TracerProvider()
	assert.NotNil(t, tp)

	// Verify it's a no-op provider when disabled
	tracer := tp.Tracer("test")
	assert.NotNil(t, tracer)
}

func TestProvider_Tracer(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Enabled: false,
	}

	provider, err := NewProvider(ctx, cfg)
	require.NoError(t, err)

	tracer := provider.Tracer("github.com/test/package")
	assert.NotNil(t, tracer)
}

// TestHTTPMiddleware tests the HTTP middleware wrapper.
func TestHTTPMiddleware(t *testing.T) {
	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	middleware := HTTPMiddleware("test-service")
	wrappedHandler := middleware(handler)

	// Make a request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestHTTPMiddleware_DifferentMethods(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMiddleware("test-service")
	wrappedHandler := middleware(handler)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/test", nil)
			rr := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestFormatSpanName(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"GET", "/", "GET /"},
		{"POST", "/", "POST /"},
		{"GET", "/api/pastes", "GET /api/pastes"},
		{"DELETE", "/api/pastes/123", "DELETE /api/pastes/123"},
		{"PUT", "/users/profile", "PUT /users/profile"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			result := formatSpanName("", req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock storage for testing InstrumentedStorage
type mockStorage struct {
	storage.Mock
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		Mock: *storage.NewMock(),
	}
}

func TestNewInstrumentedStorage(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()

	instrumented := NewInstrumentedStorage(mock, tp)

	assert.NotNil(t, instrumented)
	assert.NotNil(t, instrumented.inner)
	assert.NotNil(t, instrumented.tracer)
}

func TestInstrumentedStorage_CreatePaste(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()
	paste := &model.Paste{Data: "test content"}

	err := instrumented.CreatePaste(ctx, "testpaste1234567", paste)
	assert.NoError(t, err)

	// Verify paste was created in underlying storage
	assert.True(t, mock.PasteExists(ctx, "testpaste1234567"))
}

func TestInstrumentedStorage_CreatePaste_Error(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()
	paste := &model.Paste{Data: "test content"}

	// Create first paste
	err := instrumented.CreatePaste(ctx, "testpaste1234567", paste)
	require.NoError(t, err)

	// Try to create duplicate - should error
	err = instrumented.CreatePaste(ctx, "testpaste1234567", paste)
	assert.ErrorIs(t, err, model.ErrPasteExists)
}

func TestInstrumentedStorage_ReadPaste(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()
	original := &model.Paste{Data: "test content"}

	err := instrumented.CreatePaste(ctx, "readpaste123456", original)
	require.NoError(t, err)

	read, err := instrumented.ReadPaste(ctx, "readpaste123456")
	assert.NoError(t, err)
	assert.Equal(t, original.Data, read.Data)
}

func TestInstrumentedStorage_ReadPaste_NotFound(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	_, err := instrumented.ReadPaste(ctx, "nonexistent12345")
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestInstrumentedStorage_DeletePaste(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()
	paste := &model.Paste{Data: "to delete"}

	err := instrumented.CreatePaste(ctx, "deletepaste1234", paste)
	require.NoError(t, err)

	err = instrumented.DeletePaste(ctx, "deletepaste1234")
	assert.NoError(t, err)

	// Verify deleted
	assert.False(t, instrumented.PasteExists(ctx, "deletepaste1234"))
}

func TestInstrumentedStorage_DeletePaste_NotFound(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	err := instrumented.DeletePaste(ctx, "nonexistent12345")
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestInstrumentedStorage_PasteExists(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Should not exist initially
	assert.False(t, instrumented.PasteExists(ctx, "existscheck12345"))

	// Create paste
	paste := &model.Paste{Data: "test"}
	err := instrumented.CreatePaste(ctx, "existscheck12345", paste)
	require.NoError(t, err)

	// Should exist now
	assert.True(t, instrumented.PasteExists(ctx, "existscheck12345"))
}

func TestInstrumentedStorage_CreateComment(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Create paste first
	paste := &model.Paste{Data: "paste with comments"}
	err := instrumented.CreatePaste(ctx, "commentpaste123", paste)
	require.NoError(t, err)

	// Create comment
	comment := &model.Comment{Data: "test comment"}
	err = instrumented.CreateComment(ctx, "commentpaste123", "commentpaste123", "comment12345678", comment)
	assert.NoError(t, err)

	// Verify exists
	assert.True(t, instrumented.CommentExists(ctx, "commentpaste123", "commentpaste123", "comment12345678"))
}

func TestInstrumentedStorage_CreateComment_PasteNotFound(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	comment := &model.Comment{Data: "orphan comment"}
	err := instrumented.CreateComment(ctx, "nonexistent12345", "nonexistent12345", "comment12345678", comment)
	assert.ErrorIs(t, err, model.ErrPasteNotFound)
}

func TestInstrumentedStorage_ReadComments(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Create paste
	paste := &model.Paste{Data: "paste with comments"}
	err := instrumented.CreatePaste(ctx, "readcomments123", paste)
	require.NoError(t, err)

	// Create comments
	comment1 := &model.Comment{Data: "comment 1"}
	comment2 := &model.Comment{Data: "comment 2"}
	err = instrumented.CreateComment(ctx, "readcomments123", "readcomments123", "comment00000001", comment1)
	require.NoError(t, err)
	err = instrumented.CreateComment(ctx, "readcomments123", "readcomments123", "comment00000002", comment2)
	require.NoError(t, err)

	// Read comments
	comments, err := instrumented.ReadComments(ctx, "readcomments123")
	assert.NoError(t, err)
	assert.Len(t, comments, 2)
}

func TestInstrumentedStorage_ReadComments_Empty(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Create paste without comments
	paste := &model.Paste{Data: "lonely paste"}
	err := instrumented.CreatePaste(ctx, "nocomments12345", paste)
	require.NoError(t, err)

	comments, err := instrumented.ReadComments(ctx, "nocomments12345")
	assert.NoError(t, err)
	assert.Empty(t, comments)
}

func TestInstrumentedStorage_CommentExists(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Create paste
	paste := &model.Paste{Data: "test"}
	err := instrumented.CreatePaste(ctx, "commentexist123", paste)
	require.NoError(t, err)

	// Comment should not exist yet
	assert.False(t, instrumented.CommentExists(ctx, "commentexist123", "commentexist123", "comment12345678"))

	// Create comment
	comment := &model.Comment{Data: "test"}
	err = instrumented.CreateComment(ctx, "commentexist123", "commentexist123", "comment12345678", comment)
	require.NoError(t, err)

	// Now it should exist
	assert.True(t, instrumented.CommentExists(ctx, "commentexist123", "commentexist123", "comment12345678"))
}

func TestInstrumentedStorage_SetValue(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	err := instrumented.SetValue(ctx, "test", "key1", "value1")
	assert.NoError(t, err)

	// Verify value was set
	value, err := instrumented.GetValue(ctx, "test", "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)
}

func TestInstrumentedStorage_GetValue(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Set value
	err := instrumented.SetValue(ctx, "test", "mykey", "myvalue")
	require.NoError(t, err)

	// Get value
	value, err := instrumented.GetValue(ctx, "test", "mykey")
	assert.NoError(t, err)
	assert.Equal(t, "myvalue", value)
}

func TestInstrumentedStorage_GetValue_NotFound(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	value, err := instrumented.GetValue(ctx, "nonexistent", "key")
	assert.NoError(t, err) // Mock returns empty string, no error
	assert.Empty(t, value)
}

func TestInstrumentedStorage_GetExpiredPastes(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Get expired pastes (should be empty)
	expired, err := instrumented.GetExpiredPastes(ctx, 10)
	assert.NoError(t, err)
	assert.Empty(t, expired)
}

func TestInstrumentedStorage_Purge(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Purge (should return 0 for mock)
	count, err := instrumented.Purge(ctx, 10)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestInstrumentedStorage_PurgeValues(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Set some values
	err := instrumented.SetValue(ctx, "traffic", "ip1", "123")
	require.NoError(t, err)

	// Purge namespace
	err = instrumented.PurgeValues(ctx, "traffic", 0)
	assert.NoError(t, err)
}

func TestInstrumentedStorage_Close(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	err := instrumented.Close()
	assert.NoError(t, err)
}

// Test with actual tracing (no-op tracer but exercises span creation)
func TestInstrumentedStorage_TracingIntegration(t *testing.T) {
	mock := newMockStorage()
	tp := noop.NewTracerProvider()
	instrumented := NewInstrumentedStorage(mock, tp)

	ctx := context.Background()

	// Exercise all traced operations
	paste := &model.Paste{Data: "integration test"}
	pasteID := "integration1234"

	// Create
	err := instrumented.CreatePaste(ctx, pasteID, paste)
	require.NoError(t, err)

	// Read
	_, err = instrumented.ReadPaste(ctx, pasteID)
	require.NoError(t, err)

	// Exists
	exists := instrumented.PasteExists(ctx, pasteID)
	assert.True(t, exists)

	// Comment operations
	comment := &model.Comment{Data: "test"}
	err = instrumented.CreateComment(ctx, pasteID, pasteID, "comment12345678", comment)
	require.NoError(t, err)

	instrumented.CommentExists(ctx, pasteID, pasteID, "comment12345678")

	_, err = instrumented.ReadComments(ctx, pasteID)
	require.NoError(t, err)

	// Key-value operations
	err = instrumented.SetValue(ctx, "ns", "key", "val")
	require.NoError(t, err)

	_, err = instrumented.GetValue(ctx, "ns", "key")
	require.NoError(t, err)

	// Cleanup operations
	_, err = instrumented.GetExpiredPastes(ctx, 10)
	require.NoError(t, err)

	_, err = instrumented.Purge(ctx, 10)
	require.NoError(t, err)

	err = instrumented.PurgeValues(ctx, "ns", 0)
	require.NoError(t, err)

	// Delete
	err = instrumented.DeletePaste(ctx, pasteID)
	require.NoError(t, err)

	// Close
	err = instrumented.Close()
	require.NoError(t, err)
}
