package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
type CORSMiddleware struct{}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware() *CORSMiddleware {
	return &CORSMiddleware{}
}

// SetupCORS sets up CORS configuration
func (m *CORSMiddleware) SetupCORS() gin.HandlerFunc {
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // In production, specify exact origins
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{"Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials: true,
		MaxAge: 86400, // 24 hours
	})

	return gin.WrapH(corsMiddleware.Handler())
}