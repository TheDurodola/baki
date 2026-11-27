package handler

import (
	"net/http"

	"github.com/bolaji/baki/internal/service"
	"github.com/google/uuid"
)

type PlanHandler struct {
	planService *service.PlanService
}

func NewPlanHandler(planService *service.PlanService) *PlanHandler {
	return &PlanHandler{planService: planService}
}

func (h *PlanHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	// Public endpoint returns active plans
	plans, err := h.planService.ListPlans(r.Context(), true)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, plans)
}

func (h *PlanHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var input service.CreatePlanInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	plan, err := h.planService.CreatePlan(r.Context(), input)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusCreated, plan)
}

func (h *PlanHandler) GetPlanByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	plan, err := h.planService.GetByID(r.Context(), id)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

func (h *PlanHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	var input service.UpdatePlanInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	plan, err := h.planService.UpdatePlan(r.Context(), id, input)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, plan)
}
