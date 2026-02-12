package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"anti-nsfw-service/internal/models"
	"anti-nsfw-service/internal/services"
)

// StatsHandler handles statistics requests
type StatsHandler struct {
	service *services.NSFWService
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(service *services.NSFWService) *StatsHandler {
	return &StatsHandler{
		service: service,
	}
}

// GetStats returns service statistics
func (h *StatsHandler) GetStats(c *gin.Context) {
	stats := h.service.GetStats()
	
	c.JSON(http.StatusOK, stats)
}