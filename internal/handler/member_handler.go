package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/TheDurodola/baki/internal/middleware"
	"github.com/TheDurodola/baki/internal/service"
	"github.com/google/uuid"
)

type MemberHandler struct {
	memberService *service.MemberService
	subService    *service.SubscriptionService
}

func NewMemberHandler(memberService *service.MemberService, subService *service.SubscriptionService) *MemberHandler {
	return &MemberHandler{
		memberService: memberService,
		subService:    subService,
	}
}

// GetMe returns the authenticated user's profile
func (h *MemberHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.memberService.GetByID(r.Context(), claims.UserID)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ListMembers searches and paginates members (staff & admin only)
func (h *MemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	search := r.URL.Query().Get("search")

	members, total, err := h.memberService.ListMembers(r.Context(), page, pageSize, search)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"members": members,
		"total":   total,
		"page":    page,
	})
}

// ListExpiredMembers returns all members with expired subscriptions (staff & admin only)
func (h *MemberHandler) ListExpiredMembers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	search := r.URL.Query().Get("search")

	items, total, err := h.subService.ListExpiredMembers(r.Context(), page, pageSize, search)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"expired_members": items,
		"total":           total,
		"page":            page,
	})
}

// GetMemberByID returns member details and their active subscription status
func (h *MemberHandler) GetMemberByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member ID")
		return
	}

	member, err := h.memberService.GetByID(r.Context(), id)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	// Fetch active subscription if any
	sub, _ := h.subService.GetActiveSubscription(r.Context(), member.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"member":              member,
		"active_subscription": sub,
	})
}

// UpdateMember updates member profile
func (h *MemberHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member ID")
		return
	}

	var input service.UpdateMemberInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	member, err := h.memberService.UpdateMember(r.Context(), id, input)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, member)
}

// VerifyMember handles front-desk member verification via query (?query= or ?code=)
func (h *MemberHandler) VerifyMember(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		query = r.URL.Query().Get("code")
	}

	if strings.TrimSpace(query) == "" {
		writeError(w, http.StatusBadRequest, "query or code parameter is required")
		return
	}

	result, err := h.subService.VerifyAccess(r.Context(), query)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// VerifyByQRCode handles QR code check-in via route pattern /api/v1/members/qr/{code}
func (h *MemberHandler) VerifyByQRCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if strings.TrimSpace(code) == "" {
		writeError(w, http.StatusBadRequest, "member code is required")
		return
	}

	result, err := h.subService.VerifyAccess(r.Context(), code)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UploadMyAvatar uploads profile picture for the logged-in member
func (h *MemberHandler) UploadMyAvatar(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		writeError(w, http.StatusBadRequest, "failed to parse multipart form or file too large: "+err.Error())
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field 'avatar' or 'file' is required")
		return
	}
	defer file.Close()

	user, err := h.memberService.UploadAvatar(r.Context(), claims.UserID, file, header.Filename)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "profile picture updated successfully",
		"avatar_url": user.AvatarURL,
		"user":       user,
	})
}

// UploadMemberAvatar allows staff/admin to upload a member's photo at the desk
func (h *MemberHandler) UploadMemberAvatar(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid member ID")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form or file too large: "+err.Error())
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field 'avatar' or 'file' is required")
		return
	}
	defer file.Close()

	user, err := h.memberService.UploadAvatar(r.Context(), id, file, header.Filename)
	if err != nil {
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "member profile picture updated successfully",
		"avatar_url": user.AvatarURL,
		"user":       user,
	})
}
