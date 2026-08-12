package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			wrapper := chimiddleware.NewWrapResponseWriter(w, req.ProtoMajor)

			next.ServeHTTP(wrapper, req)
			duration := time.Since(start)
			requestId := chimiddleware.GetReqID(req.Context())
			clientIp := chimiddleware.GetClientIP(req.Context())

			routePattern := chi.RouteContext(req.Context()).RoutePattern()

			attrs := []slog.Attr{
				slog.String("request_id", requestId),
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.String("route", routePattern),
				slog.String("client_ip", clientIp),
				slog.Int("status", wrapper.Status()),
				slog.Int("bytes", wrapper.BytesWritten()),
				slog.Duration("duration", duration),
			}

			span := trace.SpanFromContext(req.Context())
			if span.SpanContext().IsValid() {
				attrs = append(attrs,
					slog.String("trace_id", span.SpanContext().TraceID().String()),
					slog.String("span_id", span.SpanContext().SpanID().String()),
				)
			}

			switch {
			case wrapper.Status() >= http.StatusInternalServerError:
				logger.LogAttrs(req.Context(), slog.LevelError, "http request", attrs...)
			case wrapper.Status() >= http.StatusBadRequest:
				logger.LogAttrs(req.Context(), slog.LevelWarn, "http request", attrs...)
			default:
				logger.LogAttrs(req.Context(), slog.LevelInfo, "http requests", attrs...)
			}
		})
	}
}
