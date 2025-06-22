package middleware

import (
	"context"
	"sync"
	"time"
)

// TokenBucketRateLimiter implements a token bucket rate limiter for sampling
type TokenBucketRateLimiter struct {
	buckets    map[string]*tokenBucket
	mu         sync.RWMutex
	capacity   int           // Maximum tokens in bucket
	refillRate int           // Tokens added per second
	refillTime time.Duration // How often to refill
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketRateLimiter creates a new token bucket rate limiter
// capacity: maximum number of tokens in the bucket
// refillRate: number of tokens to add per second
func NewTokenBucketRateLimiter(capacity, refillRate int) *TokenBucketRateLimiter {
	limiter := &TokenBucketRateLimiter{
		buckets:    make(map[string]*tokenBucket),
		capacity:   capacity,
		refillRate: refillRate,
		refillTime: time.Second,
	}
	
	// Start refill goroutine
	go limiter.refillBuckets()
	
	return limiter
}

// AllowSampling checks if a sampling request is allowed for the given client
func (r *TokenBucketRateLimiter) AllowSampling(ctx context.Context, clientID string) (bool, error) {
	r.mu.Lock()
	bucket, exists := r.buckets[clientID]
	if !exists {
		bucket = &tokenBucket{
			tokens:     r.capacity,
			lastRefill: time.Now(),
		}
		r.buckets[clientID] = bucket
	}
	r.mu.Unlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	if bucket.tokens > 0 {
		bucket.tokens--
		return true, nil
	}

	return false, nil
}

// GetRemainingQuota returns the number of remaining tokens for the client
func (r *TokenBucketRateLimiter) GetRemainingQuota(ctx context.Context, clientID string) (int, error) {
	r.mu.RLock()
	bucket, exists := r.buckets[clientID]
	r.mu.RUnlock()

	if !exists {
		return r.capacity, nil
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	return bucket.tokens, nil
}

// refillBuckets periodically refills all token buckets
func (r *TokenBucketRateLimiter) refillBuckets() {
	ticker := time.NewTicker(r.refillTime)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.RLock()
		buckets := make([]*tokenBucket, 0, len(r.buckets))
		for _, bucket := range r.buckets {
			buckets = append(buckets, bucket)
		}
		r.mu.RUnlock()

		for _, bucket := range buckets {
			bucket.mu.Lock()
			if time.Since(bucket.lastRefill) >= r.refillTime {
				tokensToAdd := r.refillRate
				if bucket.tokens+tokensToAdd > r.capacity {
					bucket.tokens = r.capacity
				} else {
					bucket.tokens += tokensToAdd
				}
				bucket.lastRefill = time.Now()
			}
			bucket.mu.Unlock()
		}
	}
}

// SimpleRateLimiter implements a simple counter-based rate limiter
type SimpleRateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int           // Maximum requests per window
	window   time.Duration // Time window
}

// NewSimpleRateLimiter creates a new simple rate limiter
// limit: maximum number of requests per window
// window: time window duration
func NewSimpleRateLimiter(limit int, window time.Duration) *SimpleRateLimiter {
	limiter := &SimpleRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	
	// Start cleanup goroutine
	go limiter.cleanup()
	
	return limiter
}

// AllowSampling checks if a sampling request is allowed for the given client
func (r *SimpleRateLimiter) AllowSampling(ctx context.Context, clientID string) (bool, error) {
	now := time.Now()
	cutoff := now.Add(-r.window)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create request history for client
	requests, exists := r.requests[clientID]
	if !exists {
		requests = make([]time.Time, 0)
	}

	// Remove old requests outside the window
	validRequests := make([]time.Time, 0)
	for _, reqTime := range requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Check if we can allow this request
	if len(validRequests) >= r.limit {
		r.requests[clientID] = validRequests
		return false, nil
	}

	// Allow the request and record it
	validRequests = append(validRequests, now)
	r.requests[clientID] = validRequests
	return true, nil
}

// GetRemainingQuota returns the number of remaining requests in the current window
func (r *SimpleRateLimiter) GetRemainingQuota(ctx context.Context, clientID string) (int, error) {
	now := time.Now()
	cutoff := now.Add(-r.window)

	r.mu.RLock()
	defer r.mu.RUnlock()

	requests, exists := r.requests[clientID]
	if !exists {
		return r.limit, nil
	}

	// Count valid requests in current window
	validCount := 0
	for _, reqTime := range requests {
		if reqTime.After(cutoff) {
			validCount++
		}
	}

	remaining := r.limit - validCount
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// cleanup periodically removes old request records
func (r *SimpleRateLimiter) cleanup() {
	ticker := time.NewTicker(r.window)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		cutoff := now.Add(-r.window * 2) // Keep some buffer

		r.mu.Lock()
		for clientID, requests := range r.requests {
			validRequests := make([]time.Time, 0)
			for _, reqTime := range requests {
				if reqTime.After(cutoff) {
					validRequests = append(validRequests, reqTime)
				}
			}
			
			if len(validRequests) == 0 {
				delete(r.requests, clientID)
			} else {
				r.requests[clientID] = validRequests
			}
		}
		r.mu.Unlock()
	}
}

// NewRateLimiter creates a new rate limiter with default settings
// This is an alias for NewTokenBucketRateLimiter for convenience
func NewRateLimiter(capacity, refillRate int) *TokenBucketRateLimiter {
	return NewTokenBucketRateLimiter(capacity, refillRate)
}