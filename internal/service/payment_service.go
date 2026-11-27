package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/TheDurodola/baki/internal/domain"
	"github.com/TheDurodola/baki/internal/gateway"
	"github.com/TheDurodola/baki/internal/repository"
	"github.com/google/uuid"
)

type PaymentService struct {
	paymentRepo *repository.PaymentRepository
	subService  *SubscriptionService
	planRepo    *repository.PlanRepository
	userRepo    *repository.UserRepository
	subRepo     *repository.SubscriptionRepository
	paystack    *gateway.PaystackGateway
}

func NewPaymentService(
	paymentRepo *repository.PaymentRepository,
	subService *SubscriptionService,
	planRepo *repository.PlanRepository,
	userRepo *repository.UserRepository,
	subRepo *repository.SubscriptionRepository,
	paystack *gateway.PaystackGateway,
) *PaymentService {
	return &PaymentService{
		paymentRepo: paymentRepo,
		subService:  subService,
		planRepo:    planRepo,
		userRepo:    userRepo,
		subRepo:     subRepo,
		paystack:    paystack,
	}
}

func generatePaymentReference() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("BAKI-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// GetPaymentQuote calculates the total cost for a member, enforcing the strict ₦5,000 first-timer fee
func (s *PaymentService) GetPaymentQuote(ctx context.Context, userID, planID uuid.UUID) (*domain.PaymentQuote, error) {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	// Check if member has any past or current subscriptions
	pastSubs, err := s.subRepo.ListByUserID(ctx, userID)
	isFirstTimer := (err == nil && len(pastSubs) == 0)

	var regFee int64 = 0
	if isFirstTimer {
		regFee = domain.FirstTimerFeeCents // Strict 5,000 Naira
	}

	total := plan.PriceCents + regFee

	return &domain.PaymentQuote{
		PlanID:               plan.ID,
		PlanName:             plan.Name,
		PlanPriceCents:       plan.PriceCents,
		RegistrationFeeCents: regFee,
		TotalAmountCents:     total,
		Currency:             plan.Currency,
		IsFirstTimer:         isFirstTimer,
	}, nil
}

type RecordManualPaymentInput struct {
	UserID      uuid.UUID             `json:"user_id"`
	PlanID      uuid.UUID             `json:"plan_id"`
	StaffID     uuid.UUID             `json:"staff_id"`
	Channel     domain.PaymentChannel `json:"channel"` // "cash", "pos", "bank_transfer"
	AmountCents int64                 `json:"amount_cents"`
	Notes       string                `json:"notes"`
}

// RecordManualPayment logs a front-desk offline payment and automatically issues/extends the subscription
func (s *PaymentService) RecordManualPayment(ctx context.Context, input RecordManualPaymentInput) (*domain.Payment, *domain.Subscription, error) {
	if input.Channel != domain.ChannelCash && input.Channel != domain.ChannelPOS && input.Channel != domain.ChannelTransfer {
		return nil, nil, errors.New("invalid manual payment channel; must be cash, pos, or bank_transfer")
	}

	quote, err := s.GetPaymentQuote(ctx, input.UserID, input.PlanID)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("member not found: %w", err)
	}

	// If AmountCents was omitted or 0, default to quote total
	paidAmount := input.AmountCents
	if paidAmount <= 0 {
		paidAmount = quote.TotalAmountCents
	}

	// Stacking subscription activation
	sub, err := s.subService.AssignSubscription(ctx, user.ID, quote.PlanID, false)
	if err != nil {
		return nil, nil, fmt.Errorf("assign subscription: %w", err)
	}

	notes := input.Notes
	if quote.IsFirstTimer {
		if notes != "" {
			notes += " | Includes ₦5,000 first-timer registration fee"
		} else {
			notes = "Includes ₦5,000 first-timer registration fee"
		}
	}

	now := time.Now().UTC()
	payment := &domain.Payment{
		ID:             uuid.New(),
		UserID:         user.ID,
		SubscriptionID: &sub.ID,
		AmountCents:    paidAmount,
		Currency:       quote.Currency,
		Channel:        input.Channel,
		Status:         domain.PaymentStatusSuccessful,
		Reference:      generatePaymentReference(),
		StaffID:        &input.StaffID,
		Notes:          notes,
		PaidAt:         &now,
		CreatedAt:      now,
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, nil, fmt.Errorf("record payment: %w", err)
	}

	return payment, sub, nil
}

type InitializeOnlinePaymentInput struct {
	UserID      uuid.UUID `json:"user_id"`
	PlanID      uuid.UUID `json:"plan_id"`
	CallbackURL string    `json:"callback_url"`
}

// InitializeOnlinePayment initiates a Paystack checkout transaction including the first-timer fee if applicable
func (s *PaymentService) InitializeOnlinePayment(ctx context.Context, input InitializeOnlinePaymentInput) (*gateway.InitializeTransactionResponse, error) {
	quote, err := s.GetPaymentQuote(ctx, input.UserID, input.PlanID)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("member not found: %w", err)
	}

	ref := generatePaymentReference()
	now := time.Now().UTC()

	notes := fmt.Sprintf("Online subscription for %s", quote.PlanName)
	if quote.IsFirstTimer {
		notes += " (includes ₦5,000 first-timer fee)"
	}

	// Create pending payment record with total calculated amount
	payment := &domain.Payment{
		ID:          uuid.New(),
		UserID:      user.ID,
		AmountCents: quote.TotalAmountCents,
		Currency:    quote.Currency,
		Channel:     domain.ChannelPaystack,
		Status:      domain.PaymentStatusPending,
		Reference:   ref,
		Notes:       notes,
		CreatedAt:   now,
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("create pending payment: %w", err)
	}

	resp, err := s.paystack.Initialize(ctx, gateway.InitializeTransactionRequest{
		Email:       user.Email,
		Amount:      quote.TotalAmountCents,
		Reference:   ref,
		CallbackURL: input.CallbackURL,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize paystack: %w", err)
	}

	return resp, nil
}

// VerifyOnlinePayment verifies transaction status with Paystack and activates subscription upon success
func (s *PaymentService) VerifyOnlinePayment(ctx context.Context, reference string, planID uuid.UUID) (*domain.Payment, *domain.Subscription, error) {
	payment, err := s.paymentRepo.GetByReference(ctx, reference)
	if err != nil {
		return nil, nil, fmt.Errorf("find payment: %w", err)
	}

	if payment.Status == domain.PaymentStatusSuccessful {
		return payment, nil, domain.ErrPaymentAlreadyProcessed
	}

	resp, err := s.paystack.Verify(ctx, reference)
	if err != nil {
		return nil, nil, fmt.Errorf("verify paystack: %w", err)
	}

	if resp.Data.Status != "success" {
		_ = s.paymentRepo.UpdateStatus(ctx, reference, domain.PaymentStatusFailed, nil, nil)
		return nil, nil, errors.New("paystack payment not successful")
	}

	// Issue subscription (stacking)
	sub, err := s.subService.AssignSubscription(ctx, payment.UserID, planID, false)
	if err != nil {
		return nil, nil, fmt.Errorf("assign subscription: %w", err)
	}

	now := time.Now().UTC()
	if err := s.paymentRepo.UpdateStatus(ctx, reference, domain.PaymentStatusSuccessful, &sub.ID, &now); err != nil {
		return nil, nil, fmt.Errorf("update payment status: %w", err)
	}

	payment.Status = domain.PaymentStatusSuccessful
	payment.SubscriptionID = &sub.ID
	payment.PaidAt = &now

	return payment, sub, nil
}

// ListPayments returns payment history
func (s *PaymentService) ListPayments(ctx context.Context, userID *uuid.UUID, page, pageSize int) ([]*domain.Payment, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	return s.paymentRepo.List(ctx, userID, pageSize, offset)
}
