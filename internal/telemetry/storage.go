package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/liskl/flashpaper/internal/model"
	"github.com/liskl/flashpaper/internal/storage"
)

// InstrumentedStorage wraps a Storage implementation with tracing.
// All methods create spans with relevant attributes for observability.
type InstrumentedStorage struct {
	inner  storage.Storage
	tracer trace.Tracer
}

// NewInstrumentedStorage creates a traced storage wrapper.
func NewInstrumentedStorage(s storage.Storage, tp trace.TracerProvider) *InstrumentedStorage {
	return &InstrumentedStorage{
		inner:  s,
		tracer: tp.Tracer("github.com/liskl/flashpaper/internal/storage"),
	}
}

// CreatePaste wraps the storage CreatePaste with tracing.
func (s *InstrumentedStorage) CreatePaste(ctx context.Context, id string, paste *model.Paste) error {
	ctx, span := s.tracer.Start(ctx, "Storage.CreatePaste",
		trace.WithAttributes(
			attribute.String("db.operation", "INSERT"),
			attribute.String("flashpaper.paste.id", id),
		),
	)
	defer span.End()

	err := s.inner.CreatePaste(ctx, id, paste)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// ReadPaste wraps the storage ReadPaste with tracing.
func (s *InstrumentedStorage) ReadPaste(ctx context.Context, id string) (*model.Paste, error) {
	ctx, span := s.tracer.Start(ctx, "Storage.ReadPaste",
		trace.WithAttributes(
			attribute.String("db.operation", "SELECT"),
			attribute.String("flashpaper.paste.id", id),
		),
	)
	defer span.End()

	paste, err := s.inner.ReadPaste(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return paste, err
}

// DeletePaste wraps the storage DeletePaste with tracing.
func (s *InstrumentedStorage) DeletePaste(ctx context.Context, id string) error {
	ctx, span := s.tracer.Start(ctx, "Storage.DeletePaste",
		trace.WithAttributes(
			attribute.String("db.operation", "DELETE"),
			attribute.String("flashpaper.paste.id", id),
		),
	)
	defer span.End()

	err := s.inner.DeletePaste(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// PasteExists wraps the storage PasteExists with tracing.
func (s *InstrumentedStorage) PasteExists(ctx context.Context, id string) bool {
	_, span := s.tracer.Start(ctx, "Storage.PasteExists",
		trace.WithAttributes(
			attribute.String("db.operation", "SELECT"),
			attribute.String("flashpaper.paste.id", id),
		),
	)
	defer span.End()

	return s.inner.PasteExists(ctx, id)
}

// CreateComment wraps the storage CreateComment with tracing.
func (s *InstrumentedStorage) CreateComment(ctx context.Context, pasteID, parentID, commentID string, comment *model.Comment) error {
	ctx, span := s.tracer.Start(ctx, "Storage.CreateComment",
		trace.WithAttributes(
			attribute.String("db.operation", "INSERT"),
			attribute.String("flashpaper.paste.id", pasteID),
			attribute.String("flashpaper.comment.id", commentID),
		),
	)
	defer span.End()

	err := s.inner.CreateComment(ctx, pasteID, parentID, commentID, comment)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// ReadComments wraps the storage ReadComments with tracing.
func (s *InstrumentedStorage) ReadComments(ctx context.Context, pasteID string) ([]*model.Comment, error) {
	ctx, span := s.tracer.Start(ctx, "Storage.ReadComments",
		trace.WithAttributes(
			attribute.String("db.operation", "SELECT"),
			attribute.String("flashpaper.paste.id", pasteID),
		),
	)
	defer span.End()

	comments, err := s.inner.ReadComments(ctx, pasteID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	if comments != nil {
		span.SetAttributes(attribute.Int("flashpaper.comment.count", len(comments)))
	}
	return comments, err
}

// CommentExists wraps the storage CommentExists with tracing.
func (s *InstrumentedStorage) CommentExists(ctx context.Context, pasteID, parentID, commentID string) bool {
	_, span := s.tracer.Start(ctx, "Storage.CommentExists",
		trace.WithAttributes(
			attribute.String("db.operation", "SELECT"),
			attribute.String("flashpaper.paste.id", pasteID),
			attribute.String("flashpaper.comment.id", commentID),
		),
	)
	defer span.End()

	return s.inner.CommentExists(ctx, pasteID, parentID, commentID)
}

// SetValue wraps the storage SetValue with tracing.
func (s *InstrumentedStorage) SetValue(ctx context.Context, namespace, key, value string) error {
	ctx, span := s.tracer.Start(ctx, "Storage.SetValue",
		trace.WithAttributes(
			attribute.String("db.operation", "UPSERT"),
			attribute.String("flashpaper.kv.namespace", namespace),
		),
	)
	defer span.End()

	err := s.inner.SetValue(ctx, namespace, key, value)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// GetValue wraps the storage GetValue with tracing.
func (s *InstrumentedStorage) GetValue(ctx context.Context, namespace, key string) (string, error) {
	ctx, span := s.tracer.Start(ctx, "Storage.GetValue",
		trace.WithAttributes(
			attribute.String("db.operation", "SELECT"),
			attribute.String("flashpaper.kv.namespace", namespace),
		),
	)
	defer span.End()

	value, err := s.inner.GetValue(ctx, namespace, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return value, err
}

// GetExpiredPastes wraps the storage GetExpiredPastes with tracing.
func (s *InstrumentedStorage) GetExpiredPastes(ctx context.Context, batchSize int) ([]string, error) {
	ctx, span := s.tracer.Start(ctx, "Storage.GetExpiredPastes",
		trace.WithAttributes(
			attribute.String("db.operation", "SELECT"),
			attribute.Int("flashpaper.batch_size", batchSize),
		),
	)
	defer span.End()

	ids, err := s.inner.GetExpiredPastes(ctx, batchSize)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.Int("flashpaper.expired_count", len(ids)))
	return ids, err
}

// Purge wraps the storage Purge with tracing.
func (s *InstrumentedStorage) Purge(ctx context.Context, batchSize int) (int, error) {
	ctx, span := s.tracer.Start(ctx, "Storage.Purge",
		trace.WithAttributes(
			attribute.String("db.operation", "DELETE"),
			attribute.Int("flashpaper.purge.batch_size", batchSize),
		),
	)
	defer span.End()

	count, err := s.inner.Purge(ctx, batchSize)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.Int("flashpaper.purge.deleted_count", count))
	return count, err
}

// PurgeValues wraps the storage PurgeValues with tracing.
func (s *InstrumentedStorage) PurgeValues(ctx context.Context, namespace string, maxAge int64) error {
	ctx, span := s.tracer.Start(ctx, "Storage.PurgeValues",
		trace.WithAttributes(
			attribute.String("db.operation", "DELETE"),
			attribute.String("flashpaper.kv.namespace", namespace),
		),
	)
	defer span.End()

	err := s.inner.PurgeValues(ctx, namespace, maxAge)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// Close delegates to the underlying storage.
func (s *InstrumentedStorage) Close() error {
	return s.inner.Close()
}
