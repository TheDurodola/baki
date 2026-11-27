package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TheDurodola/baki/internal/config"
	"github.com/TheDurodola/baki/internal/database"
	"github.com/TheDurodola/baki/internal/gateway"
	"github.com/TheDurodola/baki/internal/handler"
	"github.com/TheDurodola/baki/internal/middleware"
	"github.com/TheDurodola/baki/internal/repository"
	"github.com/TheDurodola/baki/internal/service"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg := config.Load()

	slog.Info("starting baki gym backend", "env", cfg.Env, "port", cfg.Port)

	// Connect to MongoDB & ensure collection indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		slog.Error("failed to connect to mongodb", "error", err)
		os.Exit(1)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		_ = db.Close(disconnectCtx)
	}()
	slog.Info("connected to mongodb successfully and ensured indexes", "database", cfg.MongoDBName)

	// Repositories
	userRepo := repository.NewUserRepository(db.DB)
	tokenRepo := repository.NewRefreshTokenRepository(db.DB)
	planRepo := repository.NewPlanRepository(db.DB)
	subRepo := repository.NewSubscriptionRepository(db.DB)
	paymentRepo := repository.NewPaymentRepository(db.DB)

	// External Gateways
	paystackGateway := gateway.NewPaystackGateway(cfg)
	cloudinaryGateway := gateway.NewCloudinaryGateway(cfg)

	// Services
	authService := service.NewAuthService(userRepo, tokenRepo, cfg)
	memberService := service.NewMemberService(userRepo, cloudinaryGateway)
	subService := service.NewSubscriptionService(subRepo, planRepo, userRepo)
	paymentService := service.NewPaymentService(paymentRepo, subService, planRepo, userRepo, subRepo, paystackGateway)
	planService := service.NewPlanService(planRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authService)
	memberHandler := handler.NewMemberHandler(memberService, subService)
	planHandler := handler.NewPlanHandler(planService)
	subHandler := handler.NewSubscriptionHandler(subService)
	paymentHandler := handler.NewPaymentHandler(paymentService, paystackGateway)

	// Rate Limiters:
	// - Sliding Window Log: Login (5/15m Account, 20/15m IP) and Register (3/1hr IP)
	// - Token Bucket: General API (60 req/min, burst 10)
	swl := middleware.NewSlidingWindowLog(10 * time.Minute)
	tb := middleware.NewTokenBucketLimiter(1.0, 10.0, 10*time.Minute)

	// Router
	router := handler.NewRouter(handler.RouterParams{
		AuthService:        authService,
		MemberService:      memberService,
		PlanService:        planService,
		SubService:         subService,
		PaymentService:     paymentService,
		AuthHandler:        authHandler,
		MemberHandler:      memberHandler,
		PlanHandler:        planHandler,
		SubHandler:         subHandler,
		PaymentHandler:     paymentHandler,
		SlidingWindowLog:   swl,
		TokenBucketLimiter: tb,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server shutdown listener
	shutdownErr := make(chan error, 1)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		slog.Info("shutting down server gracefully...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		shutdownErr <- server.Shutdown(shutdownCtx)
	}()

	slog.Info(fmt.Sprintf("server listening on :%s", cfg.Port))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	if err := <-shutdownErr; err != nil {
		slog.Error("error during server shutdown", "error", err)
	}

	slog.Info("server stopped gracefully")
}
