package service

import (
	"context"
	"strings"
	"time"

	"github.com/bolaji/baki/internal/domain"
	"github.com/bolaji/baki/internal/repository"
	"github.com/google/uuid"
)

type PlanService struct {
	planRepo *repository.PlanRepository
}

func NewPlanService(planRepo *repository.PlanRepository) *PlanService {
	return &PlanService{planRepo: planRepo}
}

type CreatePlanInput struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	DurationDays int    `json:"duration_days"`
	PriceCents   int64  `json:"price_cents"`
	Currency     string `json:"currency"`
}

func (s *PlanService) CreatePlan(ctx context.Context, input CreatePlanInput) (*domain.MembershipPlan, error) {
	if strings.TrimSpace(input.Name) == "" || input.DurationDays <= 0 || input.PriceCents < 0 {
		return nil, domain.ErrInvalidInput
	}

	currency := input.Currency
	if currency == "" {
		currency = "NGN"
	}

	now := time.Now().UTC()
	plan := &domain.MembershipPlan{
		ID:           uuid.New(),
		Name:         strings.TrimSpace(input.Name),
		Description:  strings.TrimSpace(input.Description),
		DurationDays: input.DurationDays,
		PriceCents:   input.PriceCents,
		Currency:     currency,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, err
	}

	return plan, nil
}

func (s *PlanService) GetByID(ctx context.Context, id uuid.UUID) (*domain.MembershipPlan, error) {
	return s.planRepo.GetByID(ctx, id)
}

func (s *PlanService) ListPlans(ctx context.Context, onlyActive bool) ([]*domain.MembershipPlan, error) {
	return s.planRepo.List(ctx, onlyActive)
}

type UpdatePlanInput struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	DurationDays int    `json:"duration_days"`
	PriceCents   int64  `json:"price_cents"`
	Currency     string `json:"currency"`
	IsActive     *bool  `json:"is_active,omitempty"`
}

func (s *PlanService) UpdatePlan(ctx context.Context, id uuid.UUID, input UpdatePlanInput) (*domain.MembershipPlan, error) {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		plan.Name = strings.TrimSpace(input.Name)
	}
	if input.Description != "" {
		plan.Description = strings.TrimSpace(input.Description)
	}
	if input.DurationDays > 0 {
		plan.DurationDays = input.DurationDays
	}
	if input.PriceCents >= 0 {
		plan.PriceCents = input.PriceCents
	}
	if input.Currency != "" {
		plan.Currency = input.Currency
	}
	if input.IsActive != nil {
		plan.IsActive = *input.IsActive
	}
	plan.UpdatedAt = time.Now().UTC()

	if err := s.planRepo.Update(ctx, plan); err != nil {
		return nil, err
	}

	return plan, nil
}
