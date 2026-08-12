package telemetry

import (
	"fmt"
	"net/http"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	InstrumentationScope = "github.com/nassabiq/go-chi-boilerplate"
)

// Provider holds the OpenTelemetry MeterProvider and standard HTTP Prometheus handler.
type Provider struct {
	meterProvider *sdkmetric.MeterProvider
	metrics       *Metrics
	handler       http.Handler
}

var (
	defaultProvider *Provider
	initOnce        sync.Once
	initErr         error
)

// InitTelemetry initializes the OpenTelemetry MeterProvider with Prometheus exporter,
// registers it globally, and constructs domain metrics (idempotent via sync.Once).
func InitTelemetry() (*Provider, error) {
	initOnce.Do(func() {
		reg := prom.NewRegistry()
		exporter, err := prometheus.New(prometheus.WithRegisterer(reg))
		if err != nil {
			initErr = fmt.Errorf("create prometheus exporter: %w", err)
			return
		}

		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(exporter),
		)
		otel.SetMeterProvider(meterProvider)

		meter := meterProvider.Meter(InstrumentationScope)
		metrics, err := NewMetrics(meter)
		if err != nil {
			initErr = fmt.Errorf("create domain metrics: %w", err)
			return
		}

		defaultProvider = &Provider{
			meterProvider: meterProvider,
			metrics:       metrics,
			handler:       promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		}
	})

	if initErr != nil {
		return nil, initErr
	}
	return defaultProvider, nil
}

// Metrics returns the domain metrics collection.
func (p *Provider) Metrics() *Metrics {
	if p == nil {
		return nil
	}
	return p.metrics
}

// MeterProvider returns the underlying SDK MeterProvider.
func (p *Provider) MeterProvider() *sdkmetric.MeterProvider {
	if p == nil {
		return nil
	}
	return p.meterProvider
}

// Meter returns a named meter from the provider.
func (p *Provider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	if p == nil || p.meterProvider == nil {
		return otel.GetMeterProvider().Meter(name, opts...)
	}
	return p.meterProvider.Meter(name, opts...)
}

// Handler returns the Prometheus HTTP metrics handler for this provider.
func (p *Provider) Handler() http.Handler {
	if p != nil && p.handler != nil {
		return p.handler
	}
	return promhttp.Handler()
}

// PrometheusHandler returns an HTTP handler serving standard Prometheus metrics format.
func PrometheusHandler() http.Handler {
	p, err := InitTelemetry()
	if err == nil && p != nil && p.handler != nil {
		return p.handler
	}
	return promhttp.Handler()
}
