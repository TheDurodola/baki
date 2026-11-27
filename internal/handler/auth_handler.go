package handler

import (
	"net/http"

	"github.com/TheDurodola/baki/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input service.RegisterInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	user, tokens, err := h.authService.Register(r.Context(), input)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user":   user,
		"tokens": tokens,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	tokens, user, err := h.authService.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":   user,
		"tokens": tokens,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input refreshRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	tokens, err := h.authService.RefreshToken(r.Context(), input.RefreshToken)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tokens": tokens,
	})
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var input logoutRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	_ = h.authService.Logout(r.Context(), input.RefreshToken)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}
