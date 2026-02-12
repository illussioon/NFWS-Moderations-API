package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	_ "image/gif"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"anti-nsfw-service/internal/config"
)

// NSFWService provides functionality for NSFW detection
type NSFWService struct {
	config     *config.Config
	logger     *zap.Logger
	models     map[string]*ModelInstance
	onnxService *ONNXRuntimeService
	stats      *Stats
	statsMutex sync.RWMutex
}

// ModelInstance holds information about a loaded model
type ModelInstance struct {
	Name string
	Path string
	// Placeholder for actual ONNX model instance
	// We'll implement this later with onnxruntime-go
}

// Stats keeps track of service statistics
type Stats struct {
	TotalScans        int64
	NSFWDetected      int64
	TotalProcessingTime time.Duration
}

// NewNSFWService creates a new NSFW service
func NewNSFWService(config *config.Config, logger *zap.Logger) *NSFWService {
	service := &NSFWService{
		config: config,
		logger: logger,
		models: make(map[string]*ModelInstance),
		onnxService: NewONNXRuntimeService(config, logger),
		stats:  &Stats{},
	}
	
	// Load all models from the models directory
	service.loadModels()
	
	return service
}

// loadModels loads all ONNX models from the configured directory
func (s *NSFWService) loadModels() {
	modelDir := s.config.ModelDir
	
	// Define known model names and their file patterns
	modelFiles := map[string]string{
		"mobilenetv2-7": "mobilenetv2-7.onnx",
		"nsfw_squeezenet": "nsfw_squeezenet.onnx",
		"NudeNet-320n": "NudeNet-320n.onnx",
		"NudeNet-640m": "NudeNet-640m.onnx",
	}
	
	for modelName, fileName := range modelFiles {
		modelPath := filepath.Join(modelDir, fileName)
		
		// Check if model file exists
		if _, err := os.Stat(modelPath); err != nil {
			s.logger.Warn("Model file not found, skipping", zap.String("model", modelName), zap.String("path", modelPath))
			continue
		}
		
		// Create model instance
		modelInstance := &ModelInstance{
			Name: modelName,
			Path: modelPath,
		}
		
		s.models[modelName] = modelInstance
		s.logger.Info("Loaded model", zap.String("model", modelName))
	}
	
	if len(s.models) == 0 {
		s.logger.Error("No models loaded, service will not work properly")
	} else {
		s.logger.Info("Successfully loaded models", zap.Int("count", len(s.models)))
	}
}

// GetLoadedModels returns a list of loaded model names
func (s *NSFWService) GetLoadedModels() []string {
	return s.onnxService.GetLoadedModels()
}

// IsReady checks if the service is ready (all essential models loaded)
func (s *NSFWService) IsReady() bool {
	return len(s.onnxService.GetLoadedModels()) > 0
}

// ScanImage scans a single image for NSFW content
func (s *NSFWService) ScanImage(ctx context.Context, req *models.ScanRequest) (*models.ScanResponse, error) {
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime).Milliseconds()
		s.updateStats(processingTime, false) // We'll update NSFW count separately after classification
	}()

	// Validate model exists
	modelExists := false
	for _, modelName := range s.onnxService.GetLoadedModels() {
		if modelName == req.Model {
			modelExists = true
			break
		}
	}
	if !modelExists {
		return nil, fmt.Errorf("model '%s' not found", req.Model)
	}

	// Get image data
	imgData, err := s.getImageData(req.ImageBase64, req.ImageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get image data: %w", err)
	}

	// Validate image format
	if _, _, err := image.DecodeConfig(strings.NewReader(string(imgData[:512]))); err != nil {
		return nil, fmt.Errorf("invalid image format: %w", err)
	}

	// Run inference using ONNX service
	response, err := s.onnxService.RunInference(ctx, req.Model, imgData)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Update stats with NSFW detection info
	if response.IsNSFW {
		s.updateStats(response.ProcessingTimeMs, true)
	} else {
		s.updateStats(response.ProcessingTimeMs, false)
	}

	return response, nil
}

// ScanBatch scans multiple images for NSFW content
func (s *NSFWService) ScanBatch(ctx context.Context, req *models.BatchScanRequest) (*models.BatchScanResponse, error) {
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime).Milliseconds()
		// We'll update total scans after processing all items
	}()

	// Validate model exists
	modelExists := false
	for _, modelName := range s.onnxService.GetLoadedModels() {
		if modelName == req.Model {
			modelExists = true
			break
		}
	}
	if !modelExists {
		return nil, fmt.Errorf("model '%s' not found", req.Model)
	}

	results := make([]models.BatchScanResult, 0, len(req.Images))

	for _, imgReq := range req.Images {
		imgStartTime := time.Now()
		
		// Get image data
		var imgData []byte
		var err error
		if imgReq.ImageBase64 != "" {
			imgData, err = s.base64ToBytes(imgReq.ImageBase64)
		} else if imgReq.ImageURL != "" {
			imgData, err = s.urlToBytes(imgReq.ImageURL)
		} else {
			err = fmt.Errorf("either image_base64 or image_url must be provided")
		}
		
		if err != nil {
			s.logger.Error("Failed to get image data for batch item", zap.String("id", imgReq.ID), zap.Error(err))
			continue
		}

		// Run inference using ONNX service
		response, err := s.onnxService.RunInference(ctx, req.Model, imgData)
		if err != nil {
			s.logger.Error("Failed to run inference for batch item", zap.String("id", imgReq.ID), zap.Error(err))
			continue
		}

		result := models.BatchScanResult{
			ID:         imgReq.ID,
			NSFWScore:  response.NSFWScore,
			IsNSFW:     response.IsNSFW,
			ProcessingTimeMs: response.ProcessingTimeMs,
		}

		results = append(results, result)
		
		// Update stats with NSFW detection info
		if result.IsNSFW {
			s.updateStats(result.ProcessingTimeMs, true)
		} else {
			s.updateStats(result.ProcessingTimeMs, false)
		}
	}

	// Update total scan count
	totalProcessingTime := time.Since(startTime).Milliseconds()
	s.updateTotalScans(len(results), totalProcessingTime)

	response := &models.BatchScanResponse{
		Results: results,
	}

	return response, nil
}

// DetectImage performs object detection on an image (for NudeNet models)
func (s *NSFWService) DetectImage(ctx context.Context, req *models.ScanRequest) (*models.ScanResponse, error) {
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime).Milliseconds()
		s.updateStats(processingTime, false) // We'll update NSFW count separately after classification
	}()

	// Check if model supports detection
	modelExists := false
	for _, modelName := range s.onnxService.GetLoadedModels() {
		if modelName == req.Model && strings.HasPrefix(strings.ToLower(modelName), "nudenet") {
			modelExists = true
			break
		}
	}
	if !modelExists {
		return nil, fmt.Errorf("model '%s' does not support detection", req.Model)
	}

	// Get image data
	imgData, err := s.getImageData(req.ImageBase64, req.ImageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get image data: %w", err)
	}

	// Run inference using ONNX service
	response, err := s.onnxService.RunInference(ctx, req.Model, imgData)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Update stats with NSFW detection info
	if response.IsNSFW {
		s.updateStats(response.ProcessingTimeMs, true)
	} else {
		s.updateStats(response.ProcessingTimeMs, false)
	}

	return response, nil
}

// GetStats returns service statistics
func (s *NSFWService) GetStats() *models.StatsResponse {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()

	var avgResponseTimeMs float64
	if s.stats.TotalScans > 0 {
		avgResponseTimeMs = float64(s.stats.TotalProcessingTime.Milliseconds()) / float64(s.stats.TotalScans)
	}

	return &models.StatsResponse{
		TotalScans:        s.stats.TotalScans,
		NSFWDetected:      s.stats.NSFWDetected,
		AvgResponseTimeMs: avgResponseTimeMs,
	}
}

// getImageData retrieves image data from either base64 string or URL
func (s *NSFWService) getImageData(base64Str, urlStr string) ([]byte, error) {
	if base64Str != "" {
		return s.base64ToBytes(base64Str)
	} else if urlStr != "" {
		return s.urlToBytes(urlStr)
	}
	return nil, fmt.Errorf("either image_base64 or image_url must be provided")
}

// base64ToBytes converts a base64 string to bytes
func (s *NSFWService) base64ToBytes(base64Str string) ([]byte, error) {
	// Remove data URL prefix if present
	if idx := strings.Index(base64Str, ","); idx != -1 {
		base64Str = base64Str[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Check file size limit
	if int64(len(data)) > s.config.MaxFileSizeMB*1024*1024 {
		return nil, fmt.Errorf("file size exceeds limit of %d MB", s.config.MaxFileSizeMB)
	}

	return data, nil
}

// urlToBytes downloads image data from a URL
func (s *NSFWService) urlToBytes(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status code %d", resp.StatusCode)
	}

	// Check content length header
	if resp.ContentLength > s.config.MaxFileSizeMB*1024*1024 {
		return nil, fmt.Errorf("file size exceeds limit of %d MB", s.config.MaxFileSizeMB)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, s.config.MaxFileSizeMB*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(data)) > s.config.MaxFileSizeMB*1024*1024 {
		return nil, fmt.Errorf("file size exceeds limit of %d MB", s.config.MaxFileSizeMB)
	}

	return data, nil
}

// updateStats updates service statistics
func (s *NSFWService) updateStats(processingTimeMs int64, isNSFW bool) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()

	s.stats.TotalScans++
	s.stats.TotalProcessingTime += time.Duration(processingTimeMs) * time.Millisecond
	if isNSFW {
		s.stats.NSFWDetected++
	}
}

// updateTotalScans updates total scans count and processing time
func (s *NSFWService) updateTotalScans(count int, totalProcessingTimeMs int64) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()

	s.stats.TotalScans += int64(count)
	s.stats.TotalProcessingTime += time.Duration(totalProcessingTimeMs) * time.Millisecond
}