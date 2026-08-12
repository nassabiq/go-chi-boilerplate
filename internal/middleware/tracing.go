package middleware

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracing creates OpenTelemetry trace spans for incoming HTTP requests.
func Tracing(serviceName string) func(http.Handler) http.Handler {
	tracer := telemetry.Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			propagator := otel.GetTextMapPropagator()
			ctx := propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))

			spanName := fmt.Sprintf("%s %s", req.Method, req.URL.Path)
			ctx, span := tracer.Start(
				ctx,
				spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethod(req.Method),
					semconv.HTTPTarget(req.URL.Path),
					semconv.ClientAddress(req.RemoteAddr),
				),
			)
			defer span.End()

			if span.SpanContext().IsValid() {
				w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
			}

			wrapper := chimiddleware.NewWrapResponseWriter(w, req.ProtoMajor)

			next.ServeHTTP(wrapper, req.WithContext(ctx))

			status := wrapper.Status()
			span.SetAttributes(
				semconv.HTTPStatusCode(status),
			)

			routePattern := chi.RouteContext(req.Context()).RoutePattern()
			if routePattern != "" {
				span.SetAttributes(semconv.HTTPRoute(routePattern))
				span.SetName(fmt.Sprintf("%s %s", req.Method, routePattern))
			}

			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}
