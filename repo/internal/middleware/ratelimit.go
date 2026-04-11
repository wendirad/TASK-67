package middleware

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	count    int
	resetAt  time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[key]
	if !exists || now.After(v.resetAt) {
		rl.visitors[key] = &visitor{count: 1, resetAt: now.Add(rl.window)}
		return true
	}

	v.count++
	return v.count <= rl.limit
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, v := range rl.visitors {
			if now.After(v.resetAt) {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit applies per-IP rate limiting. The limit defaults to 500 requests
// per minute and can be overridden via the RATE_LIMIT_PER_MINUTE env var.
func RateLimit() gin.HandlerFunc {
	limit := 500
	if v := os.Getenv("RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	limiter := newRateLimiter(limit, time.Minute)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "Rate limit exceeded. Try again later.",
			})
			return
		}
		c.Next()
	}
}
