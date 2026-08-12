package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/health"
)

type HealthHandler struct {
	checker *health.Checker
}

func NewHealthHandler(checker *health.Checker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

// Live godoc
// @Summary Liveness probe
// @Description Returns 200 OK if server process is running
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/live [get]
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "live",
	})
}

// Ready godoc
// @Summary Readiness probe
// @Description Checks connectivity to PostgreSQL, Redis, and NATS
// @Tags Health
// @Produce json
// @Success 200 {object} health.ReadinessResponse
// @Failure 503 {object} health.ReadinessResponse
// @Router /health/ready [get]
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result := h.checker.CheckReady(ctx)

	w.Header().Set("Content-Type", "application/json")
	if result.Status == "up" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(result)
}
