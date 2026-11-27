package domain

import (
	"math"
	"time"

	"github.com/google/uuid"
)

type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusExpired   SubscriptionStatus = "expired"
	SubStatusCancelled SubscriptionStatus = "cancelled"
	SubStatusSuspended SubscriptionStatus = "suspended"
)

type Subscription struct {
	ID        uuid.UUID          `json:"id" bson:"_id"`
	UserID    uuid.UUID          `json:"user_id" bson:"user_id"`
	PlanID    uuid.UUID          `json:"plan_id" bson:"plan_id"`
	Status    SubscriptionStatus `json:"status" bson:"status"`
	StartDate time.Time          `json:"start_date" bson:"start_date"`
	EndDate   time.Time          `json:"end_date" bson:"end_date"`
	AutoRenew bool               `json:"auto_renew" bson:"auto_renew"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`

	Plan *MembershipPlan `json:"plan,omitempty" bson:"plan,omitempty"`
	User *User           `json:"user,omitempty" bson:"user,omitempty"`
}

// IsCurrentlyValid strictly checks if the subscription status is active and current time is before EndDate
func (s *Subscription) IsCurrentlyValid() bool {
	return s.Status == SubStatusActive && time.Now().Before(s.EndDate)
}

// DaysRemaining calculates how many days remain until expiration (rounded up to nearest whole day)
func (s *Subscription) DaysRemaining() int {
	now := time.Now()
	if !now.Before(s.EndDate) {
		return 0
	}
	diff := s.EndDate.Sub(now)
	return int(math.Ceil(diff.Hours() / 24.0))
}

type ExpiredMemberItem struct {
	User                 *User           `json:"user"`
	LatestSubscriptionID uuid.UUID       `json:"latest_subscription_id"`
	Plan                 *MembershipPlan `json:"plan,omitempty"`
	ExpiredAt            time.Time       `json:"expired_at"`
	DaysExpired          int             `json:"days_expired"`
}
