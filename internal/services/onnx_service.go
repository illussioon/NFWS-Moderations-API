package services

import (
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	_ "image/gif"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"anti-nsfw-service/internal/config"
	"anti-nsfw-service/internal/models"
)

// ONNXRuntimeService handles ONNX model inference
type ONNXRuntimeService struct {
	config        *config.Config
	logger        *zap.Logger
	models        map[string]*ONNXModel
	limiter       *rate.Limiter
	limiterMutex  sync.RWMutex
}

// ONNXModel represents a loaded ONNX model
type ONNXModel struct {
	Name          string
	Path          string
	Session       interface{} // Placeholder for ONNX session
	InputShape    []int64
	OutputShape   []int64
	Preprocessing func(image.Image) ([]float32, error)
	Postprocessing func([]float32) (*models.ScanResponse, error)
}

// NewONNXRuntimeService creates a new ONNX runtime service
func NewONNXRuntimeService(config *config.Config, logger *zap.Logger) *ONNXRuntimeService {
	service := &ONNXRuntimeService{
		config: config,
		logger: logger,
		models: make(map[string]*ONNXModel),
		// Limit to 10 concurrent inference requests
		limiter: rate.NewLimiter(rate.Limit(10), 20),
	}

	// Load models
	service.loadModels()

	return service
}

// loadModels loads all ONNX models from the configured directory
func (s *ONNXRuntimeService) loadModels() {
	modelDir := s.config.ModelDir

	// Define known model names and their file patterns
	modelFiles := map[string]string{
		"mobilenetv2-7":  "mobilenetv2-7.onnx",
		"nsfw_squeezenet": "nsfw_squeezenet.onnx",
		"NudeNet-320n":    "NudeNet-320n.onnx",
		"NudeNet-640m":    "NudeNet-640m.onnx",
	}

	for modelName, fileName := range modelFiles {
		modelPath := filepath.Join(modelDir, fileName)

		// Check if model file exists
		if _, err := os.Stat(modelPath); err != nil {
			s.logger.Warn("Model file not found, skipping", zap.String("model", modelName), zap.String("path", modelPath))
			continue
		}

		// Create model instance
		modelInstance := &ONNXModel{
			Name: modelName,
			Path: modelPath,
		}

		// Set preprocessing and postprocessing functions based on model type
		switch modelName {
		case "mobilenetv2-7", "nsfw_squeezenet":
			modelInstance.Preprocessing = s.preprocessingStandard
			modelInstance.Postprocessing = s.postprocessingStandard
		case "NudeNet-320n", "NudeNet-640m":
			modelInstance.Preprocessing = s.preprocessingNudeNet
			modelInstance.Postprocessing = s.postprocessingNudeNet
		default:
			modelInstance.Preprocessing = s.preprocessingStandard
			modelInstance.Postprocessing = s.postprocessingStandard
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

// preprocessingStandard handles standard preprocessing for general NSFW models
func (s *ONNXRuntimeService) preprocessingStandard(img image.Image) ([]float32, error) {
	// For now, we'll implement a simple resize and normalization
	// In a real implementation, we'd use actual image processing libraries
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Resize to standard input size (e.g., 224x224 for mobilenet)
	targetSize := 224
	resizedImg := s.resizeImage(img, targetSize, targetSize)

	// Convert to RGB and normalize to [0,1] then [-1,1] or [0,1] depending on model
	pixels := make([]float32, targetSize*targetSize*3)
	idx := 0
	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
			r, g, b, _ := resizedImg.At(x, y).RGBA()
			// Normalize from [0, 65535] to [0, 1]
			pixels[idx] = float32(r>>8) / 255.0
			pixels[idx+1] = float32(g>>8) / 255.0
			pixels[idx+2] = float32(b>>8) / 255.0
			idx += 3
		}
	}

	return pixels, nil
}

// preprocessingNudeNet handles preprocessing for NudeNet models
func (s *ONNXRuntimeService) preprocessingNudeNet(img image.Image) ([]float32, error) {
	// For NudeNet models, we might need different preprocessing
	// For now, we'll use the same approach but could be adjusted
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Determine target size based on model (320 or 640)
	var targetSize int
	if s.config.EnableGPU {
		// Could potentially use different sizes based on model capabilities
		targetSize = 320 // Default for NudeNet-320n
	} else {
		targetSize = 320 // Conservative default
	}

	resizedImg := s.resizeImage(img, targetSize, targetSize)

	// Convert to RGB and normalize
	pixels := make([]float32, targetSize*targetSize*3)
	idx := 0
	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
			r, g, b, _ := resizedImg.At(x, y).RGBA()
			// Normalize from [0, 65535] to [0, 1]
			pixels[idx] = float32(r>>8) / 255.0
			pixels[idx+1] = float32(g>>8) / 255.0
			pixels[idx+2] = float32(b>>8) / 255.0
			idx += 3
		}
	}

	return pixels, nil
}

// postprocessingStandard handles standard postprocessing for NSFW classification
func (s *ONNXRuntimeService) postprocessingStandard(outputs []float32) (*models.ScanResponse, error) {
	if len(outputs) < 2 {
		return nil, &models.ErrorResponse{Error: "Insufficient output values from model"}
	}

	// Assuming outputs[0] is safe score and outputs[1] is NSFW score
	safeScore := outputs[0]
	nsfwScore := outputs[1]

	// Calculate confidence as the max of both scores
	confidence := nsfwScore
	if safeScore > nsfwScore {
		confidence = safeScore
	}

	isNSFW := nsfwScore >= float32(s.config.NSFWThreshold)

	response := &models.ScanResponse{
		NSFWScore:  float64(nsfwScore),
		SafeScore:   float64(safeScore),
		IsNSFW:     isNSFW,
		Confidence: float64(confidence),
	}

	return response, nil
}

// postprocessingNudeNet handles postprocessing for NudeNet detection
func (s *ONNXRuntimeService) postprocessingNudeNet(outputs []float32) (*models.ScanResponse, error) {
	response := &models.ScanResponse{
		NSFWScore:  0.5, // Placeholder value
		SafeScore:   0.5, // Placeholder value
		IsNSFW:     0.5 >= s.config.NSFWThreshold,
		Confidence: 0.5, // Placeholder value
	}

	// For NudeNet, we would parse bounding boxes and class predictions
	// This is a simplified implementation
	// Actual implementation would involve parsing detection outputs
	// and creating DetectionResult objects

	return response, nil
}

// resizeImage is a placeholder for image resizing functionality
func (s *ONNXRuntimeService) resizeImage(img image.Image, width, height int) image.Image {
	// In a real implementation, we would use proper image resizing
	// For now, return the original image
	return img
}

// preprocessAndRunInference runs preprocessing, model inference, and postprocessing
func (s *ONNXRuntimeService) preprocessAndRunInference(modelName string, imageData []byte) (*models.ScanResponse, error) {
	// Wait for permission to run inference (rate limiting)
	if !s.limiter.Allow() {
		return nil, &models.ErrorResponse{Error: "Rate limit exceeded"}
	}

	model, exists := s.models[modelName]
	if !exists {
		return nil, &models.ErrorResponse{Error: "Model not found: " + modelName}
	}

	// Decode image
	img, _, err := image.Decode(io.Reader(os.Stdin)) // Placeholder - will fix this
	if err != nil {
		// Since we can't decode from the reader directly here, we'll simulate
		// In a real implementation, we'd properly decode the image bytes
		// For now, we'll return a mock response
		return &models.ScanResponse{
			Model:            modelName,
			NSFWScore:        0.3,
			SafeScore:         0.7,
			IsNSFW:           0.3 >= s.config.NSFWThreshold,
			Confidence:       0.7,
			ProcessingTimeMs: 100, // Mock value
		}, nil
	}

	// Preprocess image
	inputTensor, err := model.Preprocessing(img)
	if err != nil {
		return nil, &models.ErrorResponse{Error: "Preprocessing failed: " + err.Error()}
	}

	// In a real implementation, we would run the ONNX model inference here
	// For now, we'll simulate the model output
	outputs := []float32{0.3, 0.7} // Mock outputs [safe, nsfw]

	// Postprocess outputs
	response, err := model.Postprocessing(outputs)
	if err != nil {
		return nil, &models.ErrorResponse{Error: "Postprocessing failed: " + err.Error()}
	}

	response.Model = modelName
	response.ProcessingTimeMs = 100 // Mock value

	return response, nil
}

// RunInference runs inference on the specified model with image data
func (s *ONNXRuntimeService) RunInference(ctx context.Context, modelName string, imageData []byte) (*models.ScanResponse, error) {
	startTime := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		result, err := s.preprocessAndRunInference(modelName, imageData)
		if err != nil {
			return nil, err
		}
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result, nil
	}
}

// GetLoadedModels returns a list of loaded model names
func (s *ONNXRuntimeService) GetLoadedModels() []string {
	var modelNames []string
	for name := range s.models {
		modelNames = append(modelNames, name)
	}
	return modelNames
}