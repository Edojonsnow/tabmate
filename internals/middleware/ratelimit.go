package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimitConfig struct {
	Name   string
	Limit  int
	Window time.Duration
	Burst  int
	Key    func(*gin.Context) string
}

type rateLimitBucket struct {
	tokens   float64
	lastSeen time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateLimitBucket
	config  RateLimitConfig
}

func RateLimitByIP(name string, limit int, window time.Duration, burst int) gin.HandlerFunc {
	return NewRateLimiter(RateLimitConfig{
		Name:   name,
		Limit:  limit,
		Window: window,
		Burst:  burst,
		Key: func(c *gin.Context) string {
			return c.ClientIP()
		},
	})
}

func RateLimitByUser(name string, limit int, window time.Duration, burst int) gin.HandlerFunc {
	return NewRateLimiter(RateLimitConfig{
		Name:   name,
		Limit:  limit,
		Window: window,
		Burst:  burst,
		Key: func(c *gin.Context) string {
			if userID, ok := c.Get("user_id"); ok {
				return fmt.Sprintf("user:%v", userID)
			}
			return "ip:" + c.ClientIP()
		},
	})
}

func NewRateLimiter(config RateLimitConfig) gin.HandlerFunc {
	if config.Name == "" {
		config.Name = "default"
	}
	if config.Limit <= 0 {
		config.Limit = 60
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	if config.Burst <= 0 {
		config.Burst = config.Limit
	}
	if config.Key == nil {
		config.Key = func(c *gin.Context) string {
			return c.ClientIP()
		}
	}

	limiter := &rateLimiter{
		buckets: make(map[string]*rateLimitBucket),
		config:  config,
	}
	go limiter.cleanup()

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		key := config.Name + ":" + config.Key(c)
		allowed, remaining, retryAfter := limiter.allow(key, time.Now())
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again shortly.",
			})
			return
		}

		c.Next()
	}
}

func (l *rateLimiter) allow(key string, now time.Time) (bool, int, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &rateLimitBucket{
			tokens:   float64(l.config.Burst),
			lastSeen: now,
		}
		l.buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.lastSeen)
	refill := elapsed.Seconds() * float64(l.config.Limit) / l.config.Window.Seconds()
	bucket.tokens = math.Min(float64(l.config.Burst), bucket.tokens+refill)
	bucket.lastSeen = now

	if bucket.tokens < 1 {
		secondsUntilNextToken := (1 - bucket.tokens) * l.config.Window.Seconds() / float64(l.config.Limit)
		return false, 0, time.Duration(secondsUntilNextToken * float64(time.Second))
	}

	bucket.tokens--
	return true, int(math.Floor(bucket.tokens)), 0
}

func (l *rateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		for key, bucket := range l.buckets {
			if time.Since(bucket.lastSeen) > 3*l.config.Window {
				delete(l.buckets, key)
			}
		}
		l.mu.Unlock()
	}
}
