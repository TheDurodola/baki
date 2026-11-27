package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/bolaji/baki/internal/domain"
)

type responseEnvelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(responseEnvelope{
		Success: status >= 200 && status < 300,
		Data:    data,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(responseEnvelope{
		Success: false,
		Error:   message,
	})
}

func readJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func mapDomainError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "resource not found"
	case errors.Is(err, domain.ErrDuplicateEmail):
		return http.StatusConflict, "email is already registered"
	case errors.Is(err, domain.ErrDuplicatePhone):
		return http.StatusConflict, "phone number is already registered"
	case errors.Is(err, domain.ErrDuplicateMemberCode):
		return http.StatusConflict, "member code already exists"
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, "invalid credentials"
	case errors.Is(err, domain.ErrInvalidRefreshToken):
		return http.StatusUnauthorized, "invalid, expired or revoked refresh token"
	case errors.Is(err, domain.ErrEmergencyContactRequired):
		return http.StatusBadRequest, "emergency contact name and phone are required for member registration"
	case errors.Is(err, domain.ErrInvalidRelationship):
		return http.StatusBadRequest, "invalid emergency contact relationship; must be spouse, parent, sibling, child, friend, relative, colleague, or other"
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrNoActiveSubscription):
		return http.StatusNotFound, "no active subscription found"
	case errors.Is(err, domain.ErrSubscriptionExpired):
		return http.StatusGone, "subscription is expired"
	case errors.Is(err, domain.ErrPaymentAlreadyProcessed):
		return http.StatusConflict, "payment already processed"
	default:
		return http.StatusBadRequest, err.Error()
	}
}
