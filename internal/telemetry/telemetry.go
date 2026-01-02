// Package telemetry provides OpenTelemetry tracing instrumentation.
// It handles TracerProvider initialization, HTTP middleware, and
// storage layer instrumentation for distributed tracing.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds OpenTelemetry configuration.
type Config struct {
	// Enabled enables or disables OpenTelemetry tracing
	Enabled bool

	// Endpoint is the OTLP gRPC collector endpoint (e.g., "localhost:4317")
	Endpoint string

	// ServiceName is the service name reported in traces
	ServiceName string

	// Version is the service version
	Version string

	// Environment is the deployment environment (development, staging, production)
	Environment string

	// Insecure uses insecure gRPC connection (no TLS)
	Insecure bool

	// SampleRate is the trace sampling rate (0.0-1.0)
	SampleRate float64
}

// DefaultConfig returns default OTEL configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		Endpoint:    "localhost:4317",
		ServiceName: "flashpaper",
		Version:     "dev",
		Environment: "development",
		Insecure:    true,
		SampleRate:  1.0,
	}
}

// Provider wraps the OTEL TracerProvider with shutdown capability.
type Provider struct {
	tp       trace.TracerProvider
	shutdown func(context.Context) error
}

// NewProvider creates a new OpenTelemetry provider.
// If OTEL is disabled in config, returns a no-op provider.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		return &Provider{
			tp:       noop.NewTracerProvider(),
			shutdown: func(context.Context) error { return nil },
		}, nil
	}

	// Create OTLP gRPC exporter
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	// Build resource with semantic conventions
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	// Configure sampler
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global TracerProvider and configure propagation
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		tp: tp,
		shutdown: func(ctx context.Context) error {
			return tp.Shutdown(ctx)
		},
	}, nil
}

// TracerProvider returns the underlying OTEL TracerProvider.
func (p *Provider) TracerProvider() trace.TracerProvider {
	return p.tp
}

// Tracer returns a Tracer for the given instrumentation name.
func (p *Provider) Tracer(name string) trace.Tracer {
	return p.tp.Tracer(name)
}

// Shutdown gracefully shuts down the tracer provider.
// This flushes any pending spans to the exporter.
func (p *Provider) Shutdown(ctx context.Context) error {
	// Use a reasonable default timeout if context has no deadline
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	return p.shutdown(ctx)
}
