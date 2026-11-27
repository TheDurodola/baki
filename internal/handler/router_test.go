package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bolaji/baki/internal/config"
	"github.com/bolaji/baki/internal/domain"
	"github.com/bolaji/baki/internal/handler"
	"github.com/bolaji/baki/internal/service"
	"github.com/google/uuid"
)

func TestRouter_HealthCheck(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:           "secret-secret-key-32-bytes-long!",
		AccessTokenDuration: 15 * time.Minute,
	}
	authSvc := service.NewAuthService(nil, nil, cfg)

	router := handler.NewRouter(handler.RouterParams{
		AuthService:    authSvc,
		PaymentHandler: handler.NewPaymentHandler(nil, nil),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	expectedSubstring := `"status":"healthy"`
	if !contains(rr.Body.String(), expectedSubstring) {
		t.Errorf("expected body to contain %q, got %q", expectedSubstring, rr.Body.String())
	}
}

func TestRouter_ProtectedEndpoint_RequiresAuth(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:           "secret-secret-key-32-bytes-long!",
		AccessTokenDuration: 15 * time.Minute,
	}
	authSvc := service.NewAuthService(nil, nil, cfg)

	router := handler.NewRouter(handler.RouterParams{
		AuthService:    authSvc,
		PaymentHandler: handler.NewPaymentHandler(nil, nil),
	})

	// GET /api/v1/users/me without token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got %d", rr.Code)
	}
}

func TestRouter_StaffEndpoint_ForbiddenForMember(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:           "secret-secret-key-32-bytes-long!",
		AccessTokenDuration: 15 * time.Minute,
	}
	authSvc := service.NewAuthService(nil, nil, cfg)

	// Member user token
	memberUser := &domain.User{
		ID:         uuid.New(),
		MemberCode: "GYM-999999",
		Email:      "regular_member@gym.com",
		Role:       domain.RoleMember,
	}
	token, err := authSvc.GenerateAccessToken(memberUser)
	if err != nil {
		t.Fatalf("failed to generate member token: %v", err)
	}

	router := handler.NewRouter(handler.RouterParams{
		AuthService:    authSvc,
		PaymentHandler: handler.NewPaymentHandler(nil, nil),
	})

	// Access staff-only endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/v1/members", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden for member accessing staff endpoint, got %d", rr.Code)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRouter_ExpiredMembersEndpoint_ForbiddenForMember(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:           "secret-secret-key-32-bytes-long!",
		AccessTokenDuration: 15 * time.Minute,
	}
	authSvc := service.NewAuthService(nil, nil, cfg)

	memberUser := &domain.User{
		ID:         uuid.New(),
		MemberCode: "GYM-111111",
		Email:      "member1@gym.com",
		Role:       domain.RoleMember,
	}
	token, err := authSvc.GenerateAccessToken(memberUser)
	if err != nil {
		t.Fatalf("failed to generate member token: %v", err)
	}

	router := handler.NewRouter(handler.RouterParams{
		AuthService:    authSvc,
		PaymentHandler: handler.NewPaymentHandler(nil, nil),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/members/expired", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden for member accessing /api/v1/members/expired, got %d", rr.Code)
	}
}
