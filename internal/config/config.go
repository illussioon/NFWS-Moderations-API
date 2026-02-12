package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port          string
	APIKey        string
	NSFWThreshold float64
	MaxFileSizeMB int64
	EnableGPU     bool
	LogLevel      string
	ModelDir      string
}

func LoadConfig(logger *zap.Logger) (*Config, error) {
	// Загрузка .env файла если он существует
	if err := godotenv.Load(); err != nil {
		logger.Info("No .env file found, using environment variables")
	}

	port := getEnvOrDefault("PORT", "8080")
	apiKey := getEnvOrDefault("API_KEY", "")
	nsfwThresholdStr := getEnvOrDefault("NSFW_THRESHOLD", "0.7")
	maxFileSizeStr := getEnvOrDefault("MAX_FILE_SIZE_MB", "10")
	enableGPUStr := getEnvOrDefault("ENABLE_GPU", "false")
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	modelDir := getEnvOrDefault("MODEL_DIR", "/models")

	nsfwThreshold, err := strconv.ParseFloat(nsfwThresholdStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid NSFW threshold: %v", err)
	}

	maxFileSize, err := strconv.ParseInt(maxFileSizeStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid max file size: %v", err)
	}

	enableGPU, err := strconv.ParseBool(enableGPUStr)
	if err != nil {
		return nil, fmt.Errorf("invalid enable GPU value: %v", err)
	}

	config := &Config{
		Port:          port,
		APIKey:        apiKey,
		NSFWThreshold: nsfwThreshold,
		MaxFileSizeMB: maxFileSize,
		EnableGPU:     enableGPU,
		LogLevel:      logLevel,
		ModelDir:      modelDir,
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}