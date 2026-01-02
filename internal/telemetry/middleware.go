package telemetry

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPMiddleware returns middleware that traces HTTP requests.
// It wraps the handler with OpenTelemetry instrumentation that:
// - Creates a span for each HTTP request
// - Records HTTP method, path, status code, and timing
// - Propagates trace context from incoming requests
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, serviceName,
			otelhttp.WithSpanNameFormatter(formatSpanName),
		)
	}
}

// formatSpanName creates descriptive span names like "GET /" or "POST /".
func formatSpanName(operation string, r *http.Request) string {
	return r.Method + " " + r.URL.Path
}
