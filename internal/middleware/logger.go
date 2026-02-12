package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoggerMiddleware handles request logging
type LoggerMiddleware struct {
	logger *zap.Logger
}

// NewLoggerMiddleware creates a new logger middleware
func NewLoggerMiddleware(logger *zap.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: logger,
	}
}

// LoggerToFile logs requests to a file
func (m *LoggerMiddleware) LoggerToFile() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			m.logger.Info("Request",
				zap.String("client_ip", param.ClientIP),
				zap.Time("time", param.TimeStamp),
				zap.String("method", param.Method),
				zap.String("path", param.Path),
				zap.Int("status_code", param.StatusCode),
				zap.Int64("latency", param.Latency.Nanoseconds()/1000),
				zap.String("user_agent", param.Request.UserAgent()),
			)
			return ""
		},
	})
}