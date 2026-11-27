package domain

import (
	"time"

	"github.com/google/uuid"
)

type PaymentChannel string

const (
	ChannelCash     PaymentChannel = "cash"
	ChannelPOS      PaymentChannel = "pos"
	ChannelTransfer PaymentChannel = "bank_transfer"
	ChannelPaystack PaymentChannel = "paystack"
)

type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusSuccessful PaymentStatus = "successful"
	PaymentStatusFailed     PaymentStatus = "failed"
)

// FirstTimerFeeCents is the mandatory 5,000 NGN onboarding registration fee for new members (in minor units: kobo)
const FirstTimerFeeCents int64 = 500000 // 5,000 NGN * 100

type PaymentQuote struct {
	PlanID               uuid.UUID `json:"plan_id"`
	PlanName             string    `json:"plan_name"`
	PlanPriceCents       int64     `json:"plan_price_cents"`
	RegistrationFeeCents int64     `json:"registration_fee_cents"`
	TotalAmountCents     int64     `json:"total_amount_cents"`
	Currency             string    `json:"currency"`
	IsFirstTimer         bool      `json:"is_first_timer"`
}

type Payment struct {
	ID             uuid.UUID      `json:"id" bson:"_id"`
	UserID         uuid.UUID      `json:"user_id" bson:"user_id"`
	SubscriptionID *uuid.UUID     `json:"subscription_id,omitempty" bson:"subscription_id,omitempty"`
	AmountCents    int64          `json:"amount_cents" bson:"amount_cents"`
	Currency       string         `json:"currency" bson:"currency"`
	Channel        PaymentChannel `json:"channel" bson:"channel"`
	Status         PaymentStatus  `json:"status" bson:"status"`
	Reference      string         `json:"reference" bson:"reference"`
	StaffID        *uuid.UUID     `json:"staff_id,omitempty" bson:"staff_id,omitempty"`
	Notes          string         `json:"notes,omitempty" bson:"notes,omitempty"`
	PaidAt         *time.Time     `json:"paid_at,omitempty" bson:"paid_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at" bson:"created_at"`
}
