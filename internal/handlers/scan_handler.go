package handlers

import (
	"context"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"anti-nsfw-service/internal/models"
	"anti-nsfw-service/internal/services"
)

// ScanHandler handles image scanning requests
type ScanHandler struct {
	service  *services.NSFWService
	validate *validator.Validate
}

// NewScanHandler creates a new scan handler
func NewScanHandler(service *services.NSFWService) *ScanHandler {
	return &ScanHandler{
		service:  service,
		validate: validator.New(),
	}
}

// Scan handles single image scanning requests
func (h *ScanHandler) Scan(c *gin.Context) {
	var req models.ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body: " + err.Error(),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Validation failed: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Request.Context().Deadline)
	defer cancel()

	resp, err := h.service.ScanImage(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to scan image: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ScanBatch handles batch image scanning requests
func (h *ScanHandler) ScanBatch(c *gin.Context) {
	var req models.BatchScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body: " + err.Error(),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Validation failed: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Request.Context().Deadline)
	defer cancel()

	resp, err := h.service.ScanBatch(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to scan images: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Detect handles image detection requests (for NudeNet models)
func (h *ScanHandler) Detect(c *gin.Context) {
	var req models.ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body: " + err.Error(),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Validation failed: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Request.Context().Deadline)
	defer cancel()

	resp, err := h.service.DetectImage(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to detect image: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ScanMultipart handles multipart form image uploads
func (h *ScanHandler) ScanMultipart(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Failed to get file from form: " + err.Error(),
		})
		return
	}

	model := c.PostForm("model")
	if model == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Model parameter is required",
		})
		return
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to open file: " + err.Error(),
		})
		return
	}
	defer src.Close()

	// Read the file contents
	fileBytes := make([]byte, file.Size)
	_, err = src.Read(fileBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to read file: " + err.Error(),
		})
		return
	}

	// Convert to base64 string
	imageBase64 := base64.StdEncoding.EncodeToString(fileBytes)

	req := &models.ScanRequest{
		Model:       model,
		ImageBase64: imageBase64,
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Request.Context().Deadline)
	defer cancel()

	resp, err := h.service.ScanImage(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to scan image: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}