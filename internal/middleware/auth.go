package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"anti-nsfw-service/internal/config"
)

// AuthMiddleware handles API key authentication
type AuthMiddleware struct {
	config *config.Config
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(config *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		config: config,
	}
}

// AuthRequired validates the API key in the request header
func (m *AuthMiddleware) AuthRequired(c *gin.Context) {
	// Skip auth for health endpoints
	if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/metrics" {
		c.Next()
		return
	}

	apiKey := c.GetHeader("X-API-KEY")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
		c.Abort()
		return
	}

	if apiKey != m.config.APIKey {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		c.Abort()
		return
	}

	c.Next()
}