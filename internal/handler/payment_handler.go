package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/TheDurodola/baki/internal/domain"
	"github.com/TheDurodola/baki/internal/gateway"
	"github.com/TheDurodola/baki/internal/middleware"
	"github.com/TheDurodola/baki/internal/service"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	paymentService *service.PaymentService
	paystack       *gateway.PaystackGateway
}

func NewPaymentHandler(paymentService *service.PaymentService, paystack *gateway.PaystackGateway) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		paystack:       paystack,
	}
}

// GetPaymentQuote calculates price quote with strict ₦5,000 first-timer fee
func (h *PaymentHandler) GetPaymentQuote(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	planIDStr := r.URL.Query().Get("plan_id")
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "valid plan_id query parameter is required")
		return
	}

	targetUserID := claims.UserID
	if claims.Role != domain.RoleMember {
		if reqUserID := r.URL.Query().Get("user_id"); reqUserID != "" {
			if parsed, err := uuid.Parse(reqUserID); err == nil {
				targetUserID = parsed
			}
		}
	}

	quote, err := h.paymentService.GetPaymentQuote(r.Context(), targetUserID, planID)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, quote)
}

type manualPaymentRequest struct {
	UserID      uuid.UUID             `json:"user_id"`
	PlanID      uuid.UUID             `json:"plan_id"`
	Channel     domain.PaymentChannel `json:"channel"` // "cash", "pos", "bank_transfer"
	AmountCents int64                 `json:"amount_cents"`
	Notes       string                `json:"notes"`
}

// RecordManualPayment records cash/pos payments at front desk (admin, staff)
func (h *PaymentHandler) RecordManualPayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input manualPaymentRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	payment, sub, err := h.paymentService.RecordManualPayment(r.Context(), service.RecordManualPaymentInput{
		UserID:      input.UserID,
		PlanID:      input.PlanID,
		StaffID:     claims.UserID,
		Channel:     input.Channel,
		AmountCents: input.AmountCents,
		Notes:       input.Notes,
	})
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"payment":      payment,
		"subscription": sub,
	})
}

type initializePaymentRequest struct {
	PlanID      uuid.UUID `json:"plan_id"`
	CallbackURL string    `json:"callback_url"`
}

// InitializeOnlinePayment initializes Paystack checkout
func (h *PaymentHandler) InitializeOnlinePayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input initializePaymentRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	resp, err := h.paymentService.InitializeOnlinePayment(r.Context(), service.InitializeOnlinePaymentInput{
		UserID:      claims.UserID,
		PlanID:      input.PlanID,
		CallbackURL: input.CallbackURL,
	})
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, resp.Data)
}

type verifyPaymentRequest struct {
	Reference string    `json:"reference"`
	PlanID    uuid.UUID `json:"plan_id"`
}

// VerifyOnlinePayment verifies transaction and activates subscription
func (h *PaymentHandler) VerifyOnlinePayment(w http.ResponseWriter, r *http.Request) {
	var input verifyPaymentRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	payment, sub, err := h.paymentService.VerifyOnlinePayment(r.Context(), input.Reference, input.PlanID)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"payment":      payment,
		"subscription": sub,
	})
}

// ListPayments returns payments list (filtered to user for members, all for staff/admin)
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	var filterUserID *uuid.UUID
	if claims.Role == domain.RoleMember {
		filterUserID = &claims.UserID
	} else if reqUserID := r.URL.Query().Get("user_id"); reqUserID != "" {
		if parsed, err := uuid.Parse(reqUserID); err == nil {
			filterUserID = &parsed
		}
	}

	payments, err := h.paymentService.ListPayments(r.Context(), filterUserID, page, pageSize)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, payments)
}

// PaystackWebhook processes Paystack event webhooks
func (h *PaymentHandler) PaystackWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("x-paystack-signature")
	if !h.paystack.VerifyWebhookSignature(payload, signature) {
		http.Error(w, "invalid webhook signature", http.StatusUnauthorized)
		return
	}

	var event struct {
		Event string `json:"event"`
		Data  struct {
			Reference string `json:"reference"`
			Status    string `json:"status"`
			Metadata  struct {
				PlanID string `json:"plan_id"`
			} `json:"metadata"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err == nil && event.Event == "charge.success" {
		if planUUID, err := uuid.Parse(event.Data.Metadata.PlanID); err == nil {
			_, _, _ = h.paymentService.VerifyOnlinePayment(r.Context(), event.Data.Reference, planUUID)
		}
	}

	w.WriteHeader(http.StatusOK)
}
