package domain

import (
	"time"

	"github.com/google/uuid"
)

type MembershipPlan struct {
	ID           uuid.UUID `json:"id" bson:"_id"`
	Name         string    `json:"name" bson:"name"`
	Description  string    `json:"description" bson:"description"`
	DurationDays int       `json:"duration_days" bson:"duration_days"`
	PriceCents   int64     `json:"price_cents" bson:"price_cents"`
	Currency     string    `json:"currency" bson:"currency"`
	IsActive     bool      `json:"is_active" bson:"is_active"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" bson:"updated_at"`
}
