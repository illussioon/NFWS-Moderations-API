package models

import (
	"time"
)

// ScanRequest represents the request for image scanning
type ScanRequest struct {
	Model       string `json:"model" validate:"required"`
	ImageBase64 string `json:"image_base64,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

// BatchScanRequest represents the request for batch image scanning
type BatchScanRequest struct {
	Model  string           `json:"model" validate:"required"`
	Images []BatchImageItem `json:"images" validate:"required,min=1,max=10"`
}

type BatchImageItem struct {
	ID          string `json:"id" validate:"required"`
	ImageBase64 string `json:"image_base64,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

// DetectionResult represents detection results for NudeNet models
type DetectionResult struct {
	Class      string    `json:"class"`
	Confidence float64   `json:"confidence"`
	Box        []float64 `json:"box"` // [x1, y1, x2, y2]
}

// ScanResponse represents the response for image scanning
type ScanResponse struct {
	Model             string           `json:"model"`
	NSFWScore         float64          `json:"nsfw_score"`
	SafeScore          float64          `json:"safe_score"`
	IsNSFW            bool             `json:"is_nsfw"`
	Confidence        float64          `json:"confidence"`
	ProcessingTimeMs  int64            `json:"processing_time_ms"`
	Detections        []DetectionResult `json:"detections,omitempty"`
}

// BatchScanResponse represents the response for batch image scanning
type BatchScanResponse struct {
	Results []BatchScanResult `json:"results"`
}

type BatchScanResult struct {
	ID         string  `json:"id"`
	NSFWScore  float64 `json:"nsfw_score"`
	IsNSFW     bool    `json:"is_nsfw"`
	ProcessingTimeMs int64 `json:"processing_time_ms"`
}

// StatsResponse represents statistics response
type StatsResponse struct {
	TotalScans        int64   `json:"total_scans"`
	NSFWDetected      int64   `json:"nsfw_detected"`
	AvgResponseTimeMs float64 `json:"avg_response_time_ms"`
}

// ModelListResponse represents the list of loaded models
type ModelListResponse struct {
	Models []string `json:"models"`
}

// ReadyResponse represents the readiness check response
type ReadyResponse struct {
	Status string `json:"status"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}