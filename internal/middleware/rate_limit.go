package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RateLimitMiddleware handles rate limiting
type RateLimitMiddleware struct {
	logger      *zap.Logger
	visitors    map[string]*Visitor
	mutex       sync.RWMutex
	limit       int
	window      time.Duration
	blockWindow time.Duration
}

// Visitor represents a client with its request history
type Visitor struct {
	Requests    []time.Time
	BlockedUntil time.Time
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(logger *zap.Logger, limit int, window, blockWindow time.Duration) *RateLimitMiddleware {
	r := &RateLimitMiddleware{
		logger:      logger,
		visitors:    make(map[string]*Visitor),
		limit:       limit,
		window:      window,
		blockWindow: blockWindow,
	}

	// Start cleanup goroutine to remove old entries
	go r.cleanupOldEntries()

	return r
}

// RateLimit limits requests based on IP address
func (r *RateLimitMiddleware) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		r.mutex.Lock()
		visitor, exists := r.visitors[ip]
		
		// Check if visitor is blocked
		if exists && time.Now().Before(visitor.BlockedUntil) {
			r.mutex.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}

		// Create new visitor if doesn't exist
		if !exists {
			visitor = &Visitor{
				Requests: make([]time.Time, 0),
			}
			r.visitors[ip] = visitor
		}

		now := time.Now()
		cutoff := now.Add(-r.window)
		
		// Remove old requests outside the window
		newRequests := make([]time.Time, 0)
		for _, reqTime := range visitor.Requests {
			if reqTime.After(cutoff) {
				newRequests = append(newRequests, reqTime)
			}
		}
		visitor.Requests = newRequests

		// Check if limit is exceeded
		if len(visitor.Requests) >= r.limit {
			// Block the visitor for the block window
			visitor.BlockedUntil = time.Now().Add(r.blockWindow)
			r.logger.Warn("Rate limit exceeded", zap.String("ip", ip))
			
			r.mutex.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}

		// Add current request
		visitor.Requests = append(visitor.Requests, now)
		r.mutex.Unlock()

		c.Next()
	}
}

// cleanupOldEntries periodically removes old entries to prevent memory leaks
func (r *RateLimitMiddleware) cleanupOldEntries() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		r.mutex.Lock()
		now := time.Now()
		
		// Remove visitors that haven't made requests in the last 24 hours
		for ip, visitor := range r.visitors {
			if len(visitor.Requests) == 0 && !now.Before(visitor.BlockedUntil) {
				delete(r.visitors, ip)
			}
		}
		
		r.mutex.Unlock()
	}
}