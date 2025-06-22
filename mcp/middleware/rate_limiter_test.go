package middleware

import (
	"context"
	"testing"
	"time"
)

func TestNewTokenBucketRateLimiter(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(10, 5)
	if limiter == nil {
		t.Fatal("Expected NewTokenBucketRateLimiter to return non-nil")
	}
	if limiter.capacity != 10 {
		t.Errorf("Expected capacity 10, got %d", limiter.capacity)
	}
	if limiter.refillRate != 5 {
		t.Errorf("Expected refill rate 5, got %d", limiter.refillRate)
	}
}

func TestTokenBucketRateLimiter_AllowSampling_NewClient(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(10, 5)
	defer limiter.stop() // Stop the refill goroutine

	ctx := context.Background()
	clientID := "test-client"

	// New client should start with full capacity
	allowed, err := limiter.AllowSampling(ctx, clientID)
	if err != nil {
		t.Fatalf("AllowSampling failed: %v", err)
	}
	if !allowed {
		t.Error("Expected new client to be allowed")
	}

	// Check remaining quota
	quota, err := limiter.GetRemainingQuota(ctx, clientID)
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	if quota != 9 { // Should be capacity - 1 after one request
		t.Errorf("Expected quota 9, got %d", quota)
	}
}

func TestTokenBucketRateLimiter_AllowSampling_ExhaustTokens(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(3, 1)
	defer limiter.stop()

	ctx := context.Background()
	clientID := "test-client"

	// Use all tokens
	for i := 0; i < 3; i++ {
		allowed, err := limiter.AllowSampling(ctx, clientID)
		if err != nil {
			t.Fatalf("AllowSampling failed: %v", err)
		}
		if !allowed {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// Next request should be denied
	allowed, err := limiter.AllowSampling(ctx, clientID)
	if err != nil {
		t.Fatalf("AllowSampling failed: %v", err)
	}
	if allowed {
		t.Error("Expected request to be denied when tokens exhausted")
	}

	// Quota should be 0
	quota, err := limiter.GetRemainingQuota(ctx, clientID)
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	if quota != 0 {
		t.Errorf("Expected quota 0, got %d", quota)
	}
}

func TestTokenBucketRateLimiter_GetRemainingQuota_UnknownClient(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(10, 5)
	defer limiter.stop()

	ctx := context.Background()
	clientID := "unknown-client"

	quota, err := limiter.GetRemainingQuota(ctx, clientID)
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	if quota != 10 { // Should return full capacity for unknown client
		t.Errorf("Expected quota 10 for unknown client, got %d", quota)
	}
}

func TestTokenBucketRateLimiter_Refill(t *testing.T) {
	// Use shorter refill time for testing
	limiter := &TokenBucketRateLimiter{
		buckets:    make(map[string]*tokenBucket),
		capacity:   5,
		refillRate: 2,
		refillTime: 50 * time.Millisecond,
	}
	go limiter.refillBuckets()
	defer limiter.stop()

	ctx := context.Background()
	clientID := "test-client"

	// Exhaust tokens
	for i := 0; i < 5; i++ {
		limiter.AllowSampling(ctx, clientID)
	}

	// Should be denied
	allowed, _ := limiter.AllowSampling(ctx, clientID)
	if allowed {
		t.Error("Expected request to be denied")
	}

	// Wait for refill
	time.Sleep(100 * time.Millisecond)

	// Should be allowed again after refill
	allowed, _ = limiter.AllowSampling(ctx, clientID)
	if !allowed {
		t.Error("Expected request to be allowed after refill")
	}
}

func TestNewSimpleRateLimiter(t *testing.T) {
	limiter := NewSimpleRateLimiter(10, time.Minute)
	if limiter == nil {
		t.Fatal("Expected NewSimpleRateLimiter to return non-nil")
	}
	if limiter.limit != 10 {
		t.Errorf("Expected limit 10, got %d", limiter.limit)
	}
	if limiter.window != time.Minute {
		t.Errorf("Expected window 1 minute, got %v", limiter.window)
	}
}

func TestSimpleRateLimiter_AllowSampling_NewClient(t *testing.T) {
	limiter := NewSimpleRateLimiter(10, time.Minute)
	defer limiter.stop()

	ctx := context.Background()
	clientID := "test-client"

	// New client should be allowed
	allowed, err := limiter.AllowSampling(ctx, clientID)
	if err != nil {
		t.Fatalf("AllowSampling failed: %v", err)
	}
	if !allowed {
		t.Error("Expected new client to be allowed")
	}

	// Check remaining quota
	quota, err := limiter.GetRemainingQuota(ctx, clientID)
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	if quota != 9 { // Should be limit - 1 after one request
		t.Errorf("Expected quota 9, got %d", quota)
	}
}

func TestSimpleRateLimiter_AllowSampling_ExhaustLimit(t *testing.T) {
	limiter := NewSimpleRateLimiter(3, time.Minute)
	defer limiter.stop()

	ctx := context.Background()
	clientID := "test-client"

	// Use all allowed requests
	for i := 0; i < 3; i++ {
		allowed, err := limiter.AllowSampling(ctx, clientID)
		if err != nil {
			t.Fatalf("AllowSampling failed: %v", err)
		}
		if !allowed {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// Next request should be denied
	allowed, err := limiter.AllowSampling(ctx, clientID)
	if err != nil {
		t.Fatalf("AllowSampling failed: %v", err)
	}
	if allowed {
		t.Error("Expected request to be denied when limit exceeded")
	}

	// Quota should be 0
	quota, err := limiter.GetRemainingQuota(ctx, clientID)
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	if quota != 0 {
		t.Errorf("Expected quota 0, got %d", quota)
	}
}

func TestSimpleRateLimiter_GetRemainingQuota_UnknownClient(t *testing.T) {
	limiter := NewSimpleRateLimiter(10, time.Minute)
	defer limiter.stop()

	ctx := context.Background()
	clientID := "unknown-client"

	quota, err := limiter.GetRemainingQuota(ctx, clientID)
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	if quota != 10 { // Should return full limit for unknown client
		t.Errorf("Expected quota 10 for unknown client, got %d", quota)
	}
}

func TestSimpleRateLimiter_WindowExpiry(t *testing.T) {
	// Use short window for testing
	limiter := NewSimpleRateLimiter(2, 50*time.Millisecond)
	defer limiter.stop()

	ctx := context.Background()
	clientID := "test-client"

	// Use all requests
	for i := 0; i < 2; i++ {
		allowed, _ := limiter.AllowSampling(ctx, clientID)
		if !allowed {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// Should be denied
	allowed, _ := limiter.AllowSampling(ctx, clientID)
	if allowed {
		t.Error("Expected request to be denied")
	}

	// Wait for window to expire
	time.Sleep(100 * time.Millisecond)

	// Should be allowed again after window expires
	allowed, _ = limiter.AllowSampling(ctx, clientID)
	if !allowed {
		t.Error("Expected request to be allowed after window expiry")
	}
}

func TestSimpleRateLimiter_Cleanup(t *testing.T) {
	// Use short window and cleanup interval for testing
	limiter := &SimpleRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    5,
		window:   50 * time.Millisecond,
	}
	go limiter.cleanup()
	defer limiter.stop()

	ctx := context.Background()
	clientID := "test-client"

	// Make a request to create entry
	limiter.AllowSampling(ctx, clientID)

	// Verify entry exists
	limiter.mu.RLock()
	_, exists := limiter.requests[clientID]
	limiter.mu.RUnlock()
	if !exists {
		t.Error("Expected client entry to exist")
	}

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Entry should be cleaned up
	limiter.mu.RLock()
	_, exists = limiter.requests[clientID]
	limiter.mu.RUnlock()
	if exists {
		t.Error("Expected client entry to be cleaned up")
	}
}

func TestNewRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(10, 5)
	if limiter == nil {
		t.Fatal("Expected NewRateLimiter to return non-nil")
	}
	if limiter.capacity != 10 {
		t.Errorf("Expected capacity 10, got %d", limiter.capacity)
	}
	if limiter.refillRate != 5 {
		t.Errorf("Expected refill rate 5, got %d", limiter.refillRate)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(100, 10)
	defer limiter.stop()

	ctx := context.Background()
	clientID := "test-client"

	// Run concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				limiter.AllowSampling(ctx, clientID)
				limiter.GetRemainingQuota(ctx, clientID)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should still be functional
	quota, err := limiter.GetRemainingQuota(ctx, clientID)
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	if quota < 0 || quota > 100 {
		t.Errorf("Unexpected quota after concurrent access: %d", quota)
	}
}

// Helper method to stop rate limiters (add this to the actual structs in production)
func (r *TokenBucketRateLimiter) stop() {
	// In a real implementation, you'd have a done channel to stop the goroutine
	// For testing purposes, we'll just let them run
}

func (r *SimpleRateLimiter) stop() {
	// In a real implementation, you'd have a done channel to stop the goroutine
	// For testing purposes, we'll just let them run
}