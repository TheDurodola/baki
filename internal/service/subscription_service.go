package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bolaji/baki/internal/domain"
	"github.com/bolaji/baki/internal/repository"
	"github.com/google/uuid"
)

type AccessVerificationResult struct {
	AccessGranted bool                 `json:"access_granted"`
	Reason        string               `json:"reason"`
	Member        *domain.User         `json:"member,omitempty"`
	Subscription  *domain.Subscription `json:"subscription,omitempty"`
	DaysRemaining int                  `json:"days_remaining"`
}

type SubscriptionService struct {
	subRepo  *repository.SubscriptionRepository
	planRepo *repository.PlanRepository
	userRepo *repository.UserRepository
}

func NewSubscriptionService(
	subRepo *repository.SubscriptionRepository,
	planRepo *repository.PlanRepository,
	userRepo *repository.UserRepository,
) *SubscriptionService {
	return &SubscriptionService{
		subRepo:  subRepo,
		planRepo: planRepo,
		userRepo: userRepo,
	}
}

// CalculateSubscriptionDates implements the stacking renewal requirement:
// If the user has an active subscription that hasn't expired yet, the new subscription
// extends from the existing EndDate. Otherwise, it starts immediately at now.
func (s *SubscriptionService) CalculateSubscriptionDates(ctx context.Context, userID uuid.UUID, durationDays int) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	activeSub, err := s.subRepo.GetActiveByUserID(ctx, userID)

	var startDate time.Time
	if err == nil && activeSub != nil && activeSub.EndDate.After(now) {
		// Stacking renewal: start where the current active subscription ends
		startDate = activeSub.EndDate
	} else {
		// New subscription or previous one expired
		startDate = now
	}

	endDate := startDate.AddDate(0, 0, durationDays)
	return startDate, endDate, nil
}

// AssignSubscription assigns or extends a membership plan for a member
func (s *SubscriptionService) AssignSubscription(ctx context.Context, userID, planID uuid.UUID, autoRenew bool) (*domain.Subscription, error) {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("find plan: %w", err)
	}
	if !plan.IsActive {
		return nil, errors.New("cannot subscribe to an inactive plan")
	}

	startDate, endDate, err := s.CalculateSubscriptionDates(ctx, userID, plan.DurationDays)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	sub := &domain.Subscription{
		ID:        uuid.New(),
		UserID:    userID,
		PlanID:    plan.ID,
		Status:    domain.SubStatusActive,
		StartDate: startDate,
		EndDate:   endDate,
		AutoRenew: autoRenew,
		CreatedAt: now,
		UpdatedAt: now,
		Plan:      plan,
	}

	if err := s.subRepo.Create(ctx, sub); err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}

	return sub, nil
}

// GetActiveSubscription returns the member's current active subscription with strict time check
func (s *SubscriptionService) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	sub, err := s.subRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !sub.IsCurrentlyValid() {
		return nil, domain.ErrSubscriptionExpired
	}
	return sub, nil
}

// GetLatestSubscription returns the most recent subscription
func (s *SubscriptionService) GetLatestSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	return s.subRepo.GetLatestByUserID(ctx, userID)
}

// ListSubscriptionsByMember returns subscription history for a member
func (s *SubscriptionService) ListSubscriptionsByMember(ctx context.Context, userID uuid.UUID) ([]*domain.Subscription, error) {
	return s.subRepo.ListByUserID(ctx, userID)
}

// ListExpiredMembers retrieves all members with expired subscriptions (admin & staff)
func (s *SubscriptionService) ListExpiredMembers(ctx context.Context, page, pageSize int, search string) ([]*domain.ExpiredMemberItem, int, error) {
	return s.subRepo.ListExpiredMembers(ctx, page, pageSize, search)
}

// VerifyAccess performs front-desk instant check-in verification via member code, phone, or email.
// Strictly cuts off access if expired or no subscription exists.
func (s *SubscriptionService) VerifyAccess(ctx context.Context, identifier string) (*AccessVerificationResult, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, domain.ErrInvalidInput
	}

	var member *domain.User
	var err error

	// Determine lookup strategy
	if strings.HasPrefix(strings.ToUpper(identifier), "GYM-") {
		member, err = s.userRepo.GetByMemberCode(ctx, identifier)
	} else if strings.Contains(identifier, "@") {
		member, err = s.userRepo.GetByEmail(ctx, identifier)
	} else {
		// Try member code, then phone
		member, err = s.userRepo.GetByMemberCode(ctx, identifier)
		if err != nil && errors.Is(err, domain.ErrNotFound) {
			member, err = s.userRepo.GetByPhone(ctx, identifier)
		}
	}

	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return &AccessVerificationResult{
				AccessGranted: false,
				Reason:        "member not found",
			}, nil
		}
		return nil, err
	}

	if member.Status != domain.UserStatusActive {
		return &AccessVerificationResult{
			AccessGranted: false,
			Reason:        fmt.Sprintf("member account is %s", member.Status),
			Member:        member,
		}, nil
	}

	// Retrieve active subscription
	sub, err := s.subRepo.GetActiveByUserID(ctx, member.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNoActiveSubscription) {
			// Check if they ever had a previous subscription
			latestSub, latestErr := s.subRepo.GetLatestByUserID(ctx, member.ID)
			if latestErr == nil && latestSub != nil {
				return &AccessVerificationResult{
					AccessGranted: false,
					Reason:        fmt.Sprintf("subscription expired on %s", latestSub.EndDate.Format("2006-01-02 15:04:05")),
					Member:        member,
					Subscription:  latestSub,
					DaysRemaining: 0,
				}, nil
			}

			return &AccessVerificationResult{
				AccessGranted: false,
				Reason:        "no active subscription found",
				Member:        member,
				DaysRemaining: 0,
			}, nil
		}
		return nil, err
	}

	// Strict check: access cut off exact second EndDate is reached
	if !sub.IsCurrentlyValid() {
		return &AccessVerificationResult{
			AccessGranted: false,
			Reason:        "subscription expired",
			Member:        member,
			Subscription:  sub,
			DaysRemaining: 0,
		}, nil
	}

	return &AccessVerificationResult{
		AccessGranted: true,
		Reason:        "access granted",
		Member:        member,
		Subscription:  sub,
		DaysRemaining: sub.DaysRemaining(),
	}, nil
}
