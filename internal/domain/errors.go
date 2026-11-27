package domain

import "errors"

var (
	ErrNotFound                = errors.New("record not found")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrForbidden               = errors.New("forbidden: insufficient permissions")
	ErrDuplicateEmail          = errors.New("email already registered")
	ErrDuplicatePhone          = errors.New("phone number already registered")
	ErrDuplicateMemberCode     = errors.New("member code already exists")
	ErrInvalidInput              = errors.New("invalid input")
	ErrEmergencyContactRequired  = errors.New("emergency contact name and phone number are required")
	ErrInvalidRelationship       = errors.New("invalid emergency contact relationship; must be spouse, parent, sibling, child, friend, relative, colleague, or other")
	ErrInvalidRefreshToken       = errors.New("invalid, expired or revoked refresh token")
	ErrSubscriptionExpired     = errors.New("subscription is expired")
	ErrNoActiveSubscription    = errors.New("no active subscription found")
	ErrPaymentAlreadyProcessed = errors.New("payment has already been processed")
)
