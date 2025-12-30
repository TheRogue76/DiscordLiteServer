package ratelimit

import (
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewRateLimiter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	if limiter == nil {
		t.Fatal("Expected non-nil rate limiter")
	}

	if limiter.buckets == nil {
		t.Error("Expected buckets map to be initialized")
	}
}

func TestWait_NewEndpoint(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/users/@me/guilds"

	// First call should not block
	start := time.Now()
	err := limiter.Wait(endpoint)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	// Should complete quickly for new endpoint
	if duration > 100*time.Millisecond {
		t.Errorf("Wait() took too long for new endpoint: %v", duration)
	}
}

func TestUpdateFromHeaders_ValidHeaders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/guilds/123/channels"

	headers := http.Header{
		"X-RateLimit-Limit":     []string{"50"},
		"X-RateLimit-Remaining": []string{"45"},
		"X-RateLimit-Reset":     []string{time.Now().Add(5 * time.Second).Format(time.RFC3339)},
	}

	limiter.UpdateFromHeaders(endpoint, headers)

	// Verify bucket was updated
	bucket := limiter.getBucket(endpoint)
	if bucket == nil {
		t.Fatal("Expected bucket to be created")
	}

	if bucket.Limit != 50 {
		t.Errorf("Expected Limit 50, got %d", bucket.Limit)
	}

	if bucket.Remaining != 45 {
		t.Errorf("Expected Remaining 45, got %d", bucket.Remaining)
	}
}

func TestUpdateFromHeaders_MissingHeaders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/users/@me"

	// Call with empty headers should not crash
	headers := http.Header{}
	limiter.UpdateFromHeaders(endpoint, headers)

	// Should still create bucket with defaults
	bucket := limiter.getBucket(endpoint)
	if bucket == nil {
		t.Fatal("Expected bucket to be created even with missing headers")
	}
}

func TestUpdateFromHeaders_InvalidResetTime(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/channels/456/messages"

	headers := http.Header{
		"X-RateLimit-Limit":     []string{"100"},
		"X-RateLimit-Remaining": []string{"95"},
		"X-RateLimit-Reset":     []string{"invalid_time"},
	}

	// Should not crash with invalid reset time
	limiter.UpdateFromHeaders(endpoint, headers)

	bucket := limiter.getBucket(endpoint)
	if bucket == nil {
		t.Fatal("Expected bucket to be created")
	}

	if bucket.Limit != 100 {
		t.Errorf("Expected Limit 100, got %d", bucket.Limit)
	}
}

func TestWait_RateLimitExhausted(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limit test in short mode")
	}
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/test/ratelimit"

	// Simulate exhausted rate limit
	// Note: We use 2 seconds to reduce the proportional impact of timing precision
	originalResetTime := time.Now().Add(2 * time.Second)
	resetTimeStr := originalResetTime.Format(time.RFC3339)

	headers := http.Header{
		"X-RateLimit-Limit":     []string{"5"},
		"X-RateLimit-Remaining": []string{"0"},
		"X-RateLimit-Reset":     []string{resetTimeStr},
	}

	limiter.UpdateFromHeaders(endpoint, headers)

	// Calculate the expected wait duration based on the PARSED reset time
	// (RFC3339 has second-level precision, so we lose nanoseconds)
	parsedResetTime, _ := time.Parse(time.RFC3339, resetTimeStr)

	// Wait should block until reset time
	start := time.Now()
	err := limiter.Wait(endpoint)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	// Calculate expected wait based on parsed time (accounting for RFC3339 precision loss)
	expectedWait := parsedResetTime.Sub(start)

	// Allow 100ms tolerance for test execution overhead and timing precision
	tolerance := 100 * time.Millisecond
	minWait := expectedWait - tolerance

	// Should have waited approximately until reset time
	if duration < minWait {
		t.Errorf("Wait() did not block long enough: waited %v, expected at least %v (tolerance: %v)",
			duration, minWait, tolerance)
	}
}

func TestConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/concurrent/test"

	// Multiple goroutines accessing the same endpoint
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			err := limiter.Wait(endpoint)
			if err != nil {
				t.Errorf("Wait() failed: %v", err)
			}

			limiter.UpdateFromHeaders(endpoint, http.Header{
				"X-RateLimit-Limit":     []string{"100"},
				"X-RateLimit-Remaining": []string{"90"},
			})

			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMultipleEndpoints(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoints := []string{
		"/api/v10/users/@me/guilds",
		"/api/v10/guilds/123/channels",
		"/api/v10/channels/456/messages",
	}

	// Each endpoint should have independent rate limits
	for i, endpoint := range endpoints {
		headers := http.Header{
			"X-RateLimit-Limit":     []string{string(rune(50 + i*10))}, // Different limits
			"X-RateLimit-Remaining": []string{string(rune(45 + i*10))},
		}

		limiter.UpdateFromHeaders(endpoint, headers)

		err := limiter.Wait(endpoint)
		if err != nil {
			t.Errorf("Wait() failed for endpoint %s: %v", endpoint, err)
		}
	}

	// Verify buckets are independent
	limiter.mu.RLock()
	if len(limiter.buckets) != len(endpoints) {
		t.Errorf("Expected %d buckets, got %d", len(endpoints), len(limiter.buckets))
	}
	limiter.mu.RUnlock()
}

func TestBucket_RateLimiterReset(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/reset/test"

	// Set initial rate limit
	resetTime := time.Now().Add(1 * time.Second)
	headers1 := http.Header{
		"X-RateLimit-Limit":     []string{"10"},
		"X-RateLimit-Remaining": []string{"0"},
		"X-RateLimit-Reset":     []string{resetTime.Format(time.RFC3339)},
	}
	limiter.UpdateFromHeaders(endpoint, headers1)

	// Wait for reset
	time.Sleep(1100 * time.Millisecond)

	// Update with new reset time
	newResetTime := time.Now().Add(1 * time.Second)
	headers2 := http.Header{
		"X-RateLimit-Limit":     []string{"10"},
		"X-RateLimit-Remaining": []string{"10"},
		"X-RateLimit-Reset":     []string{newResetTime.Format(time.RFC3339)},
	}
	limiter.UpdateFromHeaders(endpoint, headers2)

	// Should not block anymore
	start := time.Now()
	err := limiter.Wait(endpoint)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Wait() should not block after reset: %v", duration)
	}
}

func TestBucket_NonZeroRemaining(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	limiter := NewRateLimiter(logger)

	endpoint := "/api/v10/nonzero/test"

	headers := http.Header{
		"X-RateLimit-Limit":     []string{"50"},
		"X-RateLimit-Remaining": []string{"25"},
		"X-RateLimit-Reset":     []string{time.Now().Add(5 * time.Second).Format(time.RFC3339)},
	}

	limiter.UpdateFromHeaders(endpoint, headers)

	// Should not block when remaining > 0
	start := time.Now()
	err := limiter.Wait(endpoint)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Wait() should not block with remaining capacity: %v", duration)
	}
}
