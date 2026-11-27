package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// GetClientIP extracts real client IP handling reverse proxies
func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(parts[0])
		if clientIP != "" {
			return clientIP
		}
	}

	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------------------------------------------------------------------
// 1. Sliding Window Log Rate Limiter
// ---------------------------------------------------------------------

type SlidingWindowLog struct {
	mu      sync.Mutex
	windows map[string][]time.Time
}

func NewSlidingWindowLog(cleanupInterval time.Duration) *SlidingWindowLog {
	swl := &SlidingWindowLog{
		windows: make(map[string][]time.Time),
	}

	if cleanupInterval > 0 {
		go func() {
			ticker := time.NewTicker(cleanupInterval)
			for range ticker.C {
				swl.cleanup()
			}
		}()
	}

	return swl
}

// Allow evaluates if a request with key is allowed within window and threshold limit.
// Returns (allowed, retryAfterDuration).
func (s *SlidingWindowLog) Allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	cutoff := now.Add(-window)

	timestamps := s.windows[key]

	// Prune timestamps outside current sliding window
	validIndex := 0
	for validIndex < len(timestamps) && timestamps[validIndex].Before(cutoff) {
		validIndex++
	}
	timestamps = timestamps[validIndex:]

	if len(timestamps) >= limit {
		// Oldest timestamp in current window determines when a slot frees up
		earliest := timestamps[0]
		retryAfter := earliest.Add(window).Sub(now)
		if retryAfter <= 0 {
			retryAfter = time.Second
		}
		s.windows[key] = timestamps
		return false, retryAfter
	}

	// Record current attempt
	timestamps = append(timestamps, now)
	s.windows[key] = timestamps
	return true, 0
}

func (s *SlidingWindowLog) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for key, timestamps := range s.windows {
		if len(timestamps) == 0 {
			delete(s.windows, key)
			continue
		}
		// If newest timestamp is older than 2 hours, delete key
		if now.Sub(timestamps[len(timestamps)-1]) > 2*time.Hour {
			delete(s.windows, key)
		}
	}
}

// ---------------------------------------------------------------------
// 2. Token Bucket Rate Limiter
// ---------------------------------------------------------------------

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

type TokenBucketLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens per second
	capacity float64 // burst capacity
}

// NewTokenBucketLimiter creates a token bucket limiter
// rate: refill rate in tokens/sec
// capacity: burst size
func NewTokenBucketLimiter(rate float64, capacity float64, cleanupInterval time.Duration) *TokenBucketLimiter {
	tb := &TokenBucketLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}

	if cleanupInterval > 0 {
		go func() {
			ticker := time.NewTicker(cleanupInterval)
			for range ticker.C {
				tb.cleanup()
			}
		}()
	}

	return tb
}

func (tb *TokenBucketLimiter) Allow(key string) (bool, time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now().UTC()
	b, exists := tb.buckets[key]
	if !exists {
		// New bucket starts with full burst capacity minus 1 for current request
		tb.buckets[key] = &bucket{
			tokens:     tb.capacity - 1,
			lastRefill: now,
		}
		return true, 0
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = math.Min(tb.capacity, b.tokens+(elapsed*tb.rate))
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, 0
	}

	// Calculate wait time until 1 token is available
	missing := 1.0 - b.tokens
	retryAfterSec := missing / tb.rate
	return false, time.Duration(math.Ceil(retryAfterSec)) * time.Second
}

func (tb *TokenBucketLimiter) cleanup() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now().UTC()
	for key, b := range tb.buckets {
		// Clean up buckets that have been inactive for more than 10 minutes
		if now.Sub(b.lastRefill) > 10*time.Minute {
			delete(tb.buckets, key)
		}
	}
}

// ---------------------------------------------------------------------
// 3. Rate Limiting Middlewares
// ---------------------------------------------------------------------

func writeRateLimitError(w http.ResponseWriter, retryAfter time.Duration, message string) {
	retrySeconds := int(math.Ceil(retryAfter.Seconds()))
	if retrySeconds <= 0 {
		retrySeconds = 1
	}

	w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySeconds))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     false,
		"error":       message,
		"retry_after": fmt.Sprintf("%d seconds", retrySeconds),
	})
}

// RateLimitGeneral applies Token Bucket (60 req/min, burst 10) across general API routes
func RateLimitGeneral(tb *TokenBucketLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Key by User ID if authenticated, otherwise fallback to Client IP
			key := GetClientIP(r)
			if claims, ok := GetUserClaims(r.Context()); ok && claims != nil {
				key = claims.UserID.String()
			}

			allowed, retryAfter := tb.Allow(key)
			if !allowed {
				writeRateLimitError(w, retryAfter, "general rate limit exceeded (max 60 requests/minute, burst 10): please slow down")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitRegister applies Sliding Window Log: 3 registrations / 1 hour per IP
func RateLimitRegister(swl *SlidingWindowLog) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetClientIP(r)
			key := "register:ip:" + ip

			allowed, retryAfter := swl.Allow(key, 3, 1*time.Hour)
			if !allowed {
				writeRateLimitError(w, retryAfter, "sign up rate limit exceeded (maximum 3 registrations per hour per IP): please try again later")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitLogin applies Sliding Window Log Dual-Key limiting:
// 1. IP limit: 20 attempts / 15 mins
// 2. Account limit: 5 attempts / 15 mins (keyed by email)
func RateLimitLogin(swl *SlidingWindowLog) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetClientIP(r)
			ipKey := "login:ip:" + ip

			// 1. Check IP limit (20 attempts / 15 minutes)
			allowedIP, retryAfterIP := swl.Allow(ipKey, 20, 15*time.Minute)
			if !allowedIP {
				writeRateLimitError(w, retryAfterIP, "too many login attempts from this IP address (max 20 per 15 minutes): please try again later")
				return
			}

			// Read body to extract email, then restore r.Body
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, `{"error":"unable to read request body"}`, http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			var credentials struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(bodyBytes, &credentials)
			email := strings.ToLower(strings.TrimSpace(credentials.Email))

			if email != "" {
				accountKey := "login:account:" + email
				// 2. Check Account limit (5 attempts / 15 minutes)
				allowedAccount, retryAfterAccount := swl.Allow(accountKey, 5, 15*time.Minute)
				if !allowedAccount {
					writeRateLimitError(w, retryAfterAccount, "too many login attempts for this account (max 5 per 15 minutes): account temporarily locked")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
