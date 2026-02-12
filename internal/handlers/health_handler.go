package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"anti-nsfw-service/internal/models"
)

// HealthHandler handles health check requests
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health returns the health status of the service
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// Ready returns the readiness status of the service
func (h *HealthHandler) Ready(c *gin.Context) {
	// In a real implementation, you would check if all dependencies are ready
	// For now, we'll just return that the service is ready
	c.JSON(http.StatusOK, models.ReadyResponse{
		Status: "ok",
	})
}