package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"anti-nsfw-service/internal/models"
	"anti-nsfw-service/internal/services"
)

// ModelHandler handles model-related requests
type ModelHandler struct {
	service *services.NSFWService
}

// NewModelHandler creates a new model handler
func NewModelHandler(service *services.NSFWService) *ModelHandler {
	return &ModelHandler{
		service: service,
	}
}

// GetModels returns the list of loaded models
func (h *ModelHandler) GetModels(c *gin.Context) {
	models := h.service.GetLoadedModels()
	
	c.JSON(http.StatusOK, models.ModelListResponse{
		Models: models,
	})
}