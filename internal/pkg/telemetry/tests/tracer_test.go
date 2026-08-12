package tests

import (
	"context"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestInitTracer_Success(t *testing.T) {
	tp, err := telemetry.InitTracer("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	tracer := telemetry.Tracer("test-module")
	assert.NotNil(t, tracer)

	ctx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()

	err = telemetry.ShutdownTracer(context.Background())
	assert.NoError(t, err)
}
