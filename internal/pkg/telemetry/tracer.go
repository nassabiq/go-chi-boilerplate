package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracerProvider *sdktrace.TracerProvider
	tracerOnce     sync.Once
)

// InitTracer initializes the OpenTelemetry TracerProvider and registers global propagators.
func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
	var err error
	tracerOnce.Do(func() {
		res, rerr := resource.Merge(
			resource.Default(),
			resource.NewSchemaless(
				semconv.ServiceName(serviceName),
			),
		)
		if rerr != nil {
			err = rerr
			return
		}

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
		)

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

		tracerProvider = tp
	})

	if err != nil {
		return nil, err
	}
	return tracerProvider, nil
}

// Tracer returns a named tracer for instrumentation.
func Tracer(name string) trace.Tracer {
	return otel.GetTracerProvider().Tracer(name)
}

// ShutdownTracer flushes any unexported spans and stops the TracerProvider.
func ShutdownTracer(ctx context.Context) error {
	if tracerProvider != nil {
		return tracerProvider.Shutdown(ctx)
	}
	return nil
}
