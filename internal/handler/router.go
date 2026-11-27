package handler

import (
	"net/http"
	"time"

	"github.com/TheDurodola/baki/internal/domain"
	"github.com/TheDurodola/baki/internal/middleware"
	"github.com/TheDurodola/baki/internal/service"
)

type RouterParams struct {
	AuthService        *service.AuthService
	MemberService      *service.MemberService
	PlanService        *service.PlanService
	SubService         *service.SubscriptionService
	PaymentService     *service.PaymentService
	AuthHandler        *AuthHandler
	MemberHandler      *MemberHandler
	PlanHandler        *PlanHandler
	SubHandler         *SubscriptionHandler
	PaymentHandler     *PaymentHandler
	SlidingWindowLog   *middleware.SlidingWindowLog
	TokenBucketLimiter *middleware.TokenBucketLimiter
}

func NewRouter(p RouterParams) http.Handler {
	mux := http.NewServeMux()

	swl := p.SlidingWindowLog
	if swl == nil {
		swl = middleware.NewSlidingWindowLog(10 * time.Minute)
	}

	tb := p.TokenBucketLimiter
	if tb == nil {
		// 60 requests/minute (1.0 token/sec) with burst capacity of 10
		tb = middleware.NewTokenBucketLimiter(1.0, 10.0, 10*time.Minute)
	}

	rateLimitRegister := middleware.RateLimitRegister(swl)
	rateLimitLogin := middleware.RateLimitLogin(swl)
	rateLimitGeneral := middleware.RateLimitGeneral(tb)

	authHandler := p.AuthHandler
	if authHandler == nil {
		authHandler = &AuthHandler{}
	}
	memberHandler := p.MemberHandler
	if memberHandler == nil {
		memberHandler = &MemberHandler{}
	}
	planHandler := p.PlanHandler
	if planHandler == nil {
		planHandler = &PlanHandler{}
	}
	subHandler := p.SubHandler
	if subHandler == nil {
		subHandler = &SubscriptionHandler{}
	}
	paymentHandler := p.PaymentHandler
	if paymentHandler == nil {
		paymentHandler = &PaymentHandler{}
	}

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "healthy",
			"service": "baki-gym-api",
		})
	})

	// Public Auth routes protected by Sliding Window Log
	mux.Handle("POST /api/v1/auth/register", rateLimitRegister(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/v1/auth/login", rateLimitLogin(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// Public Membership Plans lookup
	mux.HandleFunc("GET /api/v1/plans", planHandler.ListPlans)
	mux.HandleFunc("GET /api/v1/plans/{id}", planHandler.GetPlanByID)

	// Public Webhooks
	mux.HandleFunc("POST /api/v1/payments/paystack/webhook", paymentHandler.PaystackWebhook)

	// Middleware wrappers
	auth := middleware.Authenticate(p.AuthService)
	staffOrAdmin := middleware.RequireRole(domain.RoleStaff, domain.RoleAdmin)
	adminOnly := middleware.RequireRole(domain.RoleAdmin)

	// Authenticated User routes
	mux.Handle("GET /api/v1/users/me", auth(http.HandlerFunc(memberHandler.GetMe)))
	mux.Handle("POST /api/v1/users/me/avatar", auth(http.HandlerFunc(memberHandler.UploadMyAvatar)))
	mux.Handle("GET /api/v1/subscriptions/me", auth(http.HandlerFunc(subHandler.GetMySubscription)))
	mux.Handle("GET /api/v1/subscriptions/history", auth(http.HandlerFunc(subHandler.GetMyHistory)))
	mux.Handle("GET /api/v1/payments", auth(http.HandlerFunc(paymentHandler.ListPayments)))
	mux.Handle("GET /api/v1/payments/quote", auth(http.HandlerFunc(paymentHandler.GetPaymentQuote)))
	mux.Handle("POST /api/v1/payments/initialize", auth(http.HandlerFunc(paymentHandler.InitializeOnlinePayment)))
	mux.Handle("POST /api/v1/payments/verify", auth(http.HandlerFunc(paymentHandler.VerifyOnlinePayment)))

	// Front-Desk & Staff Member Management routes
	mux.Handle("GET /api/v1/members", auth(staffOrAdmin(http.HandlerFunc(memberHandler.ListMembers))))
	mux.Handle("GET /api/v1/members/expired", auth(staffOrAdmin(http.HandlerFunc(memberHandler.ListExpiredMembers))))
	mux.Handle("GET /api/v1/members/verify", auth(staffOrAdmin(http.HandlerFunc(memberHandler.VerifyMember))))
	mux.Handle("GET /api/v1/members/qr/{code}", auth(staffOrAdmin(http.HandlerFunc(memberHandler.VerifyByQRCode))))
	mux.Handle("GET /api/v1/members/{id}", auth(staffOrAdmin(http.HandlerFunc(memberHandler.GetMemberByID))))
	mux.Handle("PUT /api/v1/members/{id}", auth(staffOrAdmin(http.HandlerFunc(memberHandler.UpdateMember))))
	mux.Handle("POST /api/v1/members/{id}/avatar", auth(staffOrAdmin(http.HandlerFunc(memberHandler.UploadMemberAvatar))))

	// Staff Manual Subscription & Payment routes
	mux.Handle("POST /api/v1/subscriptions", auth(staffOrAdmin(http.HandlerFunc(subHandler.AssignSubscription))))
	mux.Handle("POST /api/v1/payments/manual", auth(staffOrAdmin(http.HandlerFunc(paymentHandler.RecordManualPayment))))

	// Admin-Only Plan Configuration routes
	mux.Handle("POST /api/v1/plans", auth(adminOnly(http.HandlerFunc(planHandler.CreatePlan))))
	mux.Handle("PUT /api/v1/plans/{id}", auth(adminOnly(http.HandlerFunc(planHandler.UpdatePlan))))

	// Wrap global middlewares: Logger -> Recovery -> CORS -> RateLimitGeneral (Token Bucket) -> Mux
	return middleware.Logger(middleware.Recovery(middleware.CORS(rateLimitGeneral(mux))))
}
