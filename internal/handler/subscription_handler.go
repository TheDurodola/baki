package handler

import (
	"net/http"

	"github.com/bolaji/baki/internal/middleware"
	"github.com/bolaji/baki/internal/service"
	"github.com/google/uuid"
)

type SubscriptionHandler struct {
	subService *service.SubscriptionService
}

func NewSubscriptionHandler(subService *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subService: subService}
}

// GetMySubscription returns the active subscription of the logged-in member
func (h *SubscriptionHandler) GetMySubscription(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sub, err := h.subService.GetActiveSubscription(r.Context(), claims.UserID)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"subscription":   sub,
		"days_remaining": sub.DaysRemaining(),
		"is_valid":       sub.IsCurrentlyValid(),
	})
}

// GetMyHistory returns all subscriptions for the logged-in member
func (h *SubscriptionHandler) GetMyHistory(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	subs, err := h.subService.ListSubscriptionsByMember(r.Context(), claims.UserID)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, subs)
}

type assignSubscriptionRequest struct {
	UserID    uuid.UUID `json:"user_id"`
	PlanID    uuid.UUID `json:"plan_id"`
	AutoRenew bool      `json:"auto_renew"`
}

// AssignSubscription manually assigns or extends a subscription (admin, staff)
func (h *SubscriptionHandler) AssignSubscription(w http.ResponseWriter, r *http.Request) {
	var input assignSubscriptionRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	sub, err := h.subService.AssignSubscription(r.Context(), input.UserID, input.PlanID, input.AutoRenew)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusCreated, sub)
}
