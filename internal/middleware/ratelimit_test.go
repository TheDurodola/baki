package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bolaji/baki/internal/middleware"
)

func TestSlidingWindowLog_BasicLimit(t *testing.T) {
	swl := middleware.NewSlidingWindowLog(0)
	key := "test_ip_127.0.0.1"
	limit := 3
	window := 1 * time.Minute

	// First 3 requests should succeed
	for i := 0; i < limit; i++ {
		allowed, _ := swl.Allow(key, limit, window)
		if !allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	// 4th request must be rejected
	allowed, retryAfter := swl.Allow(key, limit, window)
	if allowed {
		t.Fatalf("4th request should have been rejected")
	}
	if retryAfter <= 0 {
		t.Errorf("expected positive retryAfter duration, got %v", retryAfter)
	}
}

func TestTokenBucketLimiter_BurstAndExhaustion(t *testing.T) {
	rate := 1.0     // 1 token per second
	capacity := 3.0 // burst of 3
	tb := middleware.NewTokenBucketLimiter(rate, capacity, 0)
	key := "test_client_key"

	// 3 burst requests should be allowed
	for i := 0; i < 3; i++ {
		allowed, _ := tb.Allow(key)
		if !allowed {
			t.Fatalf("burst request %d should have been allowed", i+1)
		}
	}

	// 4th immediate request should be rejected (bucket empty)
	allowed, retryAfter := tb.Allow(key)
	if allowed {
		t.Fatalf("4th request should have been rejected as bucket is empty")
	}
	if retryAfter <= 0 {
		t.Errorf("expected positive retryAfter duration, got %v", retryAfter)
	}
}

func TestRateLimitLogin_AccountLimitEnforcement(t *testing.T) {
	swl := middleware.NewSlidingWindowLog(0)
	loginMiddleware := middleware.RateLimitLogin(swl)

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	wrapped := loginMiddleware(dummyHandler)

	bodyJSON := []byte(`{"email":"target_account@gym.com","password":"badpassword"}`)

	// Account limit is 5 attempts / 15 mins
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("attempt %d should be 200 OK, got: %d", i+1, rr.Code)
		}
	}

	// 6th attempt for same email must return 429 Too Many Requests
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt should be 429 Too Many Requests, got: %d", rr.Code)
	}

	if rr.Header().Get("Retry-After") == "" {
		t.Errorf("expected Retry-After header to be set on 429 response")
	}
}

func TestRateLimitRegister_IPLimitEnforcement(t *testing.T) {
	swl := middleware.NewSlidingWindowLog(0)
	registerMiddleware := middleware.RateLimitRegister(swl)

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	wrapped := registerMiddleware(dummyHandler)

	// IP limit is 3 registrations / 1 hour
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
		req.RemoteAddr = "203.0.113.195:4321"
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("registration %d should succeed, got: %d", i+1, rr.Code)
		}
	}

	// 4th registration from same IP must be 429 Too Many Requests
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
	req.RemoteAddr = "203.0.113.195:4321"
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("4th registration should be 429 Too Many Requests, got: %d", rr.Code)
	}
}
