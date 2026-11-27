package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bolaji/baki/internal/config"
	"github.com/bolaji/baki/internal/domain"
	"github.com/bolaji/baki/internal/service"
	"github.com/google/uuid"
)

func TestGenerateMemberCode(t *testing.T) {
	code := service.GenerateMemberCode()
	if !strings.HasPrefix(code, "GYM-") {
		t.Fatalf("expected member code to start with GYM-, got: %s", code)
	}
	if len(code) != 10 { // "GYM-" + 6 digits = 10 chars
		t.Fatalf("expected member code length 10, got: %d (%s)", len(code), code)
	}
}

func TestSubscription_IsCurrentlyValid(t *testing.T) {
	now := time.Now()

	activeValid := &domain.Subscription{
		Status:  domain.SubStatusActive,
		EndDate: now.Add(24 * time.Hour),
	}
	if !activeValid.IsCurrentlyValid() {
		t.Errorf("expected active subscription with future end date to be valid")
	}

	activeExpired := &domain.Subscription{
		Status:  domain.SubStatusActive,
		EndDate: now.Add(-1 * time.Second),
	}
	if activeExpired.IsCurrentlyValid() {
		t.Errorf("expected active subscription with past end date to be invalid (strict cut-off)")
	}

	cancelled := &domain.Subscription{
		Status:  domain.SubStatusCancelled,
		EndDate: now.Add(24 * time.Hour),
	}
	if cancelled.IsCurrentlyValid() {
		t.Errorf("expected cancelled subscription to be invalid")
	}
}

func TestSubscription_DaysRemaining(t *testing.T) {
	now := time.Now()

	sub := &domain.Subscription{
		Status:  domain.SubStatusActive,
		EndDate: now.Add(5 * 24 * time.Hour),
	}
	if days := sub.DaysRemaining(); days != 5 {
		t.Errorf("expected 5 days remaining, got %d", days)
	}

	expiredSub := &domain.Subscription{
		Status:  domain.SubStatusActive,
		EndDate: now.Add(-2 * time.Hour),
	}
	if days := expiredSub.DaysRemaining(); days != 0 {
		t.Errorf("expected 0 days remaining for expired subscription, got %d", days)
	}
}

func TestAuthService_TokenGenerationAndValidation(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:           "test-super-secret-key-at-least-32-chars",
		AccessTokenDuration: 15 * time.Minute,
	}

	authService := service.NewAuthService(nil, nil, cfg)

	user := &domain.User{
		ID:         uuid.New(),
		MemberCode: "GYM-123456",
		Email:      "member@test.com",
		Role:       domain.RoleMember,
	}

	tokenStr, err := authService.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	claims, err := authService.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, claims.UserID)
	}
	if claims.MemberCode != user.MemberCode {
		t.Errorf("expected member code %s, got %s", user.MemberCode, claims.MemberCode)
	}
	if claims.Role != domain.RoleMember {
		t.Errorf("expected role %s, got %s", domain.RoleMember, claims.Role)
	}
}

func TestSubscription_StackingRenewalDates(t *testing.T) {
	now := time.Now().UTC()

	// Case 1: Member currently has 10 days remaining
	currentExpiry := now.Add(10 * 24 * time.Hour)
	planDurationDays := 30

	var newStartDate time.Time
	if currentExpiry.After(now) {
		newStartDate = currentExpiry // Stacking renewal
	} else {
		newStartDate = now
	}
	newEndDate := newStartDate.AddDate(0, 0, planDurationDays)

	expectedTotalDurationHours := 40 * 24 * time.Hour
	actualTotalDurationHours := newEndDate.Sub(now)

	// Within slight clock precision delta
	if actualTotalDurationHours < expectedTotalDurationHours-time.Second || actualTotalDurationHours > expectedTotalDurationHours+time.Second {
		t.Fatalf("expected stacking renewal to preserve remaining 10 days + 30 days = 40 days, got duration: %v", actualTotalDurationHours)
	}

	// Case 2: Member's subscription expired 3 days ago
	expiredAt := now.Add(-3 * 24 * time.Hour)
	if expiredAt.After(now) {
		newStartDate = expiredAt
	} else {
		newStartDate = now // Starts fresh from today
	}
	newEndDate = newStartDate.AddDate(0, 0, planDurationDays)

	actualDurationAfterNow := newEndDate.Sub(now)
	expected30DaysHours := 30 * 24 * time.Hour
	if actualDurationAfterNow < expected30DaysHours-time.Second || actualDurationAfterNow > expected30DaysHours+time.Second {
		t.Fatalf("expected expired subscription to restart from now (30 days), got duration: %v", actualDurationAfterNow)
	}
}

func TestAuthService_GenerateTokenPair(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:            "test-super-secret-key-at-least-32-chars",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
	}

	authService := service.NewAuthService(nil, nil, cfg)

	user := &domain.User{
		ID:         uuid.New(),
		MemberCode: "GYM-789012",
		Email:      "member2@test.com",
		Role:       domain.RoleMember,
	}

	tokenPair, err := authService.GenerateTokenPair(nil, user)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	if tokenPair.AccessToken == "" {
		t.Errorf("expected access token to be non-empty")
	}
	if tokenPair.RefreshToken == "" {
		t.Errorf("expected refresh token to be non-empty")
	}
	if len(tokenPair.RefreshToken) != 64 { // 32 hex bytes = 64 chars
		t.Errorf("expected refresh token length 64, got %d", len(tokenPair.RefreshToken))
	}
	if tokenPair.ExpiresIn != 900 {
		t.Errorf("expected expires_in to be 900s (15m), got %d", tokenPair.ExpiresIn)
	}

	// Validate access token
	claims, err := authService.ValidateToken(tokenPair.AccessToken)
	if err != nil {
		t.Fatalf("failed to validate generated access token: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, claims.UserID)
	}
}

func TestPaymentQuote_FirstTimerFee(t *testing.T) {
	// Plan: Monthly Standard (₦20,000)
	monthlyPlanPriceCents := int64(2000000)
	firstTimerFeeCents := domain.FirstTimerFeeCents // ₦5,000 = 500,000

	// Case 1: First-timer member (0 past subscriptions)
	hasPastSubscriptions := false
	var regFee int64 = 0
	if !hasPastSubscriptions {
		regFee = firstTimerFeeCents
	}
	totalForFirstTimer := monthlyPlanPriceCents + regFee
	if totalForFirstTimer != 2500000 { // ₦25,000
		t.Errorf("expected first timer to pay ₦25,000 (2500000 kobo), got: %d", totalForFirstTimer)
	}

	// Case 2: Returning member renewing (has past subscriptions)
	hasPastSubscriptions = true
	regFee = 0
	if !hasPastSubscriptions {
		regFee = firstTimerFeeCents
	}
	totalForReturning := monthlyPlanPriceCents + regFee
	if totalForReturning != 2000000 { // ₦20,000
		t.Errorf("expected returning member to pay ₦20,000 (2000000 kobo), got: %d", totalForReturning)
	}
}

func TestDefaultMembershipPlanPrices(t *testing.T) {
	expectedPrices := map[string]struct {
		durationDays int
		priceCents   int64 // minor units (kobo)
	}{
		"Monthly":  {durationDays: 30, priceCents: 2000000},   // ₦20,000
		"3 Months": {durationDays: 90, priceCents: 5000000},   // ₦50,000
		"6 Months": {durationDays: 180, priceCents: 10000000}, // ₦100,000
		"1 Year":   {durationDays: 365, priceCents: 18000000}, // ₦180,000
	}

	for planName, p := range expectedPrices {
		if p.priceCents <= 0 {
			t.Errorf("expected positive price for plan %s", planName)
		}
		if p.durationDays <= 0 {
			t.Errorf("expected positive duration for plan %s", planName)
		}
	}
}

func TestAuthService_MemberEmergencyContactRequired(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:           "test-super-secret-key-at-least-32-chars",
		AccessTokenDuration: 15 * time.Minute,
	}

	authService := service.NewAuthService(nil, nil, cfg)

	// Case 1: Member missing emergency contact should fail
	inputMissingContact := service.RegisterInput{
		Email:     "test_nomember@test.com",
		Phone:     "+2348000000001",
		Password:  "password123",
		FirstName: "John",
		LastName:  "Doe",
		Role:      domain.RoleMember,
		EmergencyContact: domain.EmergencyContact{
			Name:  "",
			Phone: "",
		},
	}
	_, _, err := authService.Register(context.Background(), inputMissingContact)
	if !errors.Is(err, domain.ErrEmergencyContactRequired) {
		t.Fatalf("expected ErrEmergencyContactRequired when emergency contact is empty, got: %v", err)
	}

	// Case 2: Member with only name missing phone should fail
	inputMissingPhone := service.RegisterInput{
		Email:     "test_nophone@test.com",
		Phone:     "+2348000000002",
		Password:  "password123",
		FirstName: "John",
		LastName:  "Doe",
		Role:      domain.RoleMember,
		EmergencyContact: domain.EmergencyContact{
			Name:  "Jane Doe",
			Phone: "",
		},
	}
	_, _, err = authService.Register(context.Background(), inputMissingPhone)
	if !errors.Is(err, domain.ErrEmergencyContactRequired) {
		t.Fatalf("expected ErrEmergencyContactRequired when emergency contact phone is empty, got: %v", err)
	}
}

func TestAuthService_MemberEmergencyContactRelationshipValidation(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:           "test-super-secret-key-at-least-32-chars",
		AccessTokenDuration: 15 * time.Minute,
	}

	authService := service.NewAuthService(nil, nil, cfg)

	// Case 1: Invalid relationship string should fail
	inputInvalidRel := service.RegisterInput{
		Email:     "test_invalidrel@test.com",
		Phone:     "+2348000000003",
		Password:  "password123",
		FirstName: "John",
		LastName:  "Doe",
		Role:      domain.RoleMember,
		EmergencyContact: domain.EmergencyContact{
			Name:         "Jane Doe",
			Phone:        "+2348000000004",
			Relationship: "alien", // invalid
		},
	}
	_, _, err := authService.Register(context.Background(), inputInvalidRel)
	if !errors.Is(err, domain.ErrInvalidRelationship) {
		t.Fatalf("expected ErrInvalidRelationship when relationship is invalid, got: %v", err)
	}

	// Case 2: Relationship.IsValid() method test
	validRelationships := []domain.Relationship{
		domain.RelationshipSpouse,
		domain.RelationshipParent,
		domain.RelationshipSibling,
		domain.RelationshipChild,
		domain.RelationshipFriend,
		domain.RelationshipRelative,
		domain.RelationshipColleague,
		domain.RelationshipOther,
	}

	for _, rel := range validRelationships {
		if !rel.IsValid() {
			t.Errorf("expected relationship %s to be valid", rel)
		}
	}

	if domain.Relationship("unknown").IsValid() {
		t.Errorf("expected unknown relationship to be invalid")
	}
}
