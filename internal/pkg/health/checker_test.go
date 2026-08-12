package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChecker_Live(t *testing.T) {
	c := NewChecker(nil, nil, nil)
	assert.True(t, c.CheckLive())
}

func TestChecker_Ready_Disconnected(t *testing.T) {
	c := NewChecker(nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res := c.CheckReady(ctx)
	assert.Equal(t, "down", res.Status)
	assert.Equal(t, "down", res.Components["database"].Status)
	assert.Equal(t, "down", res.Components["redis"].Status)
	assert.Equal(t, "down", res.Components["nats"].Status)
}
