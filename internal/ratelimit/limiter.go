// Package ratelimit implements Discord API rate limiting based on response headers.
package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Bucket represents a rate limit bucket for a specific Discord API endpoint
type Bucket struct {
	Remaining int           // Requests remaining in current window
	Limit     int           // Total requests allowed per window
	ResetAt   time.Time     // When the rate limit resets
	limiter   *rate.Limiter // Token bucket rate limiter
	mu        sync.Mutex
}

// RateLimiter manages rate limits for Discord API endpoints
type RateLimiter struct {
	buckets map[string]*Bucket // endpoint -> bucket
	mu      sync.RWMutex
	logger  *zap.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*Bucket),
		logger:  logger,
	}
}

// getBucket retrieves or creates a bucket for an endpoint
func (rl *RateLimiter) getBucket(endpoint string) *Bucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bucket, exists := rl.buckets[endpoint]; exists {
		return bucket
	}

	// Default rate limit: 5 requests per second (Discord global rate limit)
	// Per-route limits will be updated from response headers
	bucket := &Bucket{
		Remaining: 5,
		Limit:     5,
		ResetAt:   time.Now().Add(1 * time.Second),
		limiter:   rate.NewLimiter(rate.Every(200*time.Millisecond), 5),
	}

	rl.buckets[endpoint] = bucket
	return bucket
}

// Wait waits if necessary to respect rate limits before making a request
func (rl *RateLimiter) Wait(endpoint string) error {
	bucket := rl.getBucket(endpoint)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Check if rate limit is exhausted
	if bucket.Remaining <= 0 && time.Now().Before(bucket.ResetAt) {
		waitDuration := time.Until(bucket.ResetAt)
		rl.logger.Warn("Rate limit exhausted, waiting",
			zap.String("endpoint", endpoint),
			zap.Duration("wait_duration", waitDuration),
		)
		time.Sleep(waitDuration)
	}

	// Wait for token from rate limiter
	err := bucket.limiter.Wait(context.Background())
	if err != nil {
		return fmt.Errorf("rate limiter wait failed: %w", err)
	}

	return nil
}

// UpdateFromHeaders updates rate limit bucket from Discord API response headers
func (rl *RateLimiter) UpdateFromHeaders(endpoint string, headers map[string][]string) {
	bucket := rl.getBucket(endpoint)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Parse X-RateLimit-Remaining
	if remaining := headers["X-RateLimit-Remaining"]; len(remaining) > 0 {
		if val, err := strconv.Atoi(remaining[0]); err == nil {
			bucket.Remaining = val
		}
	}

	// Parse X-RateLimit-Limit
	if limit := headers["X-RateLimit-Limit"]; len(limit) > 0 {
		if val, err := strconv.Atoi(limit[0]); err == nil {
			bucket.Limit = val
		}
	}

	// Parse X-RateLimit-Reset (RFC3339 or Unix timestamp)
	if reset := headers["X-RateLimit-Reset"]; len(reset) > 0 {
		// Try parsing as RFC3339 first
		if t, err := time.Parse(time.RFC3339, reset[0]); err == nil {
			bucket.ResetAt = t
		} else if val, err := strconv.ParseInt(reset[0], 10, 64); err == nil {
			// Fall back to Unix timestamp
			bucket.ResetAt = time.Unix(val, 0)
		}
	}

	// Update rate limiter if we have new limit information
	if bucket.Limit > 0 {
		// Calculate tokens per second based on limit and reset window
		resetDuration := time.Until(bucket.ResetAt)
		if resetDuration > 0 {
			tokensPerSecond := float64(bucket.Limit) / resetDuration.Seconds()
			bucket.limiter = rate.NewLimiter(rate.Limit(tokensPerSecond), bucket.Limit)
		}
	}

	rl.logger.Debug("Updated rate limit from headers",
		zap.String("endpoint", endpoint),
		zap.Int("remaining", bucket.Remaining),
		zap.Int("limit", bucket.Limit),
		zap.Time("reset_at", bucket.ResetAt),
	)
}

// HandleRateLimitResponse handles a 429 (rate limited) response
func (rl *RateLimiter) HandleRateLimitResponse(endpoint string, headers map[string][]string) error {
	bucket := rl.getBucket(endpoint)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Parse Retry-After header (in seconds)
	var retryAfter time.Duration
	if retry := headers["Retry-After"]; len(retry) > 0 {
		if seconds, err := strconv.Atoi(retry[0]); err == nil {
			retryAfter = time.Duration(seconds) * time.Second
		}
	}

	// If no Retry-After, use reset time from headers
	if retryAfter == 0 {
		if reset := headers["X-Ratelimit-Reset"]; len(reset) > 0 {
			if val, err := strconv.ParseInt(reset[0], 10, 64); err == nil {
				resetAt := time.Unix(val, 0)
				retryAfter = time.Until(resetAt)
			}
		}
	}

	// Default to 1 second if no timing information
	if retryAfter <= 0 {
		retryAfter = 1 * time.Second
	}

	bucket.Remaining = 0
	bucket.ResetAt = time.Now().Add(retryAfter)

	rl.logger.Warn("Rate limited by Discord API",
		zap.String("endpoint", endpoint),
		zap.Duration("retry_after", retryAfter),
	)

	return fmt.Errorf("rate limited, retry after %v", retryAfter)
}

// GetStatus returns the current rate limit status for an endpoint
func (rl *RateLimiter) GetStatus(endpoint string) (remaining int, limit int, resetAt time.Time) {
	bucket := rl.getBucket(endpoint)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	return bucket.Remaining, bucket.Limit, bucket.ResetAt
}

// Reset clears all rate limit buckets (useful for testing)
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.buckets = make(map[string]*Bucket)
	rl.logger.Info("Rate limiter reset")
}
