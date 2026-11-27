package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/bolaji/baki/internal/config"
	"github.com/bolaji/baki/internal/domain"
	"github.com/bolaji/baki/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type JWTClaims struct {
	UserID     uuid.UUID   `json:"user_id"`
	MemberCode string      `json:"member_code"`
	Email      string      `json:"email"`
	Role       domain.Role `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	userRepo  *repository.UserRepository
	tokenRepo *repository.RefreshTokenRepository
	cfg       *config.Config
}

func NewAuthService(
	userRepo *repository.UserRepository,
	tokenRepo *repository.RefreshTokenRepository,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		cfg:       cfg,
	}
}

// GenerateMemberCode generates a short, unique code for member check-in e.g. "EMT-849201"
func GenerateMemberCode() string {
	const charset = "0123456789"
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			b[i] = charset[i%len(charset)]
		} else {
			b[i] = charset[n.Int64()]
		}
	}
	return fmt.Sprintf("EMT-%s", string(b))
}

func generateRandomTokenString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

type RegisterInput struct {
	Email            string                  `json:"email"`
	Phone            string                  `json:"phone"`
	Password         string                  `json:"password"`
	FirstName        string                  `json:"first_name"`
	LastName         string                  `json:"last_name"`
	EmergencyContact domain.EmergencyContact `json:"emergency_contact"`
	Role             domain.Role             `json:"role"` // Defaults to RoleMember if empty
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*domain.User, *domain.TokenPair, error) {
	if strings.TrimSpace(input.Email) == "" || strings.TrimSpace(input.Phone) == "" || len(input.Password) < 6 {
		return nil, nil, domain.ErrInvalidInput
	}

	role := input.Role
	if role == "" {
		role = domain.RoleMember
	}

	// Members are strictly required to submit emergency contact details at sign-up
	if role == domain.RoleMember {
		if strings.TrimSpace(input.EmergencyContact.Name) == "" || strings.TrimSpace(input.EmergencyContact.Phone) == "" {
			return nil, nil, domain.ErrEmergencyContactRequired
		}
		rel := domain.Relationship(strings.ToLower(strings.TrimSpace(string(input.EmergencyContact.Relationship))))
		if !rel.IsValid() {
			return nil, nil, domain.ErrInvalidRelationship
		}
		input.EmergencyContact.Relationship = rel
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	memberCode := GenerateMemberCode()
	now := time.Now().UTC()

	user := &domain.User{
		ID:               uuid.New(),
		MemberCode:       memberCode,
		Email:            strings.ToLower(strings.TrimSpace(input.Email)),
		Phone:            strings.TrimSpace(input.Phone),
		PasswordHash:     string(hash),
		FirstName:        strings.TrimSpace(input.FirstName),
		LastName:         strings.TrimSpace(input.LastName),
		EmergencyContact: input.EmergencyContact,
		Role:             role,
		Status:           domain.UserStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Retry member code collision if exceptionally rare collision occurs
	for attempts := 0; attempts < 3; attempts++ {
		err = s.userRepo.Create(ctx, user)
		if err == nil {
			break
		}
		if errors.Is(err, domain.ErrDuplicateMemberCode) {
			user.MemberCode = GenerateMemberCode()
			continue
		}
		return nil, nil, err
	}

	if err != nil {
		return nil, nil, err
	}

	tokens, err := s.GenerateTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.TokenPair, *domain.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil, domain.ErrUnauthorized
		}
		return nil, nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, domain.ErrUnauthorized
	}

	tokens, err := s.GenerateTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return tokens, user, nil
}

// GenerateAccessToken creates a short-lived JWT for API access
func (s *AuthService) GenerateAccessToken(user *domain.User) (string, error) {
	now := time.Now().UTC()
	claims := JWTClaims{
		UserID:     user.ID,
		MemberCode: user.MemberCode,
		Email:      user.Email,
		Role:       user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return tokenString, nil
}

// GenerateTokenPair generates both an access token and a refresh token, persisting the refresh token
func (s *AuthService) GenerateTokenPair(ctx context.Context, user *domain.User) (*domain.TokenPair, error) {
	accessToken, err := s.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	rawRefreshToken, err := generateRandomTokenString()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	rt := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     rawRefreshToken,
		ExpiresAt: now.Add(s.cfg.RefreshTokenDuration),
		CreatedAt: now,
	}

	if s.tokenRepo != nil {
		if err := s.tokenRepo.Create(ctx, rt); err != nil {
			return nil, fmt.Errorf("store refresh token: %w", err)
		}
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		ExpiresIn:    int64(s.cfg.AccessTokenDuration.Seconds()),
	}, nil
}

// RefreshToken rotates the refresh token and returns a fresh TokenPair
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr string) (*domain.TokenPair, error) {
	refreshTokenStr = strings.TrimSpace(refreshTokenStr)
	if refreshTokenStr == "" {
		return nil, domain.ErrInvalidRefreshToken
	}

	if s.tokenRepo == nil {
		return nil, errors.New("token repository not configured")
	}

	rt, err := s.tokenRepo.GetByToken(ctx, refreshTokenStr)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, err
	}

	if !rt.IsActive() {
		return nil, domain.ErrInvalidRefreshToken
	}

	// Token rotation: revoke old refresh token
	_ = s.tokenRepo.Revoke(ctx, refreshTokenStr)

	user, err := s.userRepo.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user for token: %w", err)
	}

	if user.Status != domain.UserStatusActive {
		return nil, domain.ErrForbidden
	}

	return s.GenerateTokenPair(ctx, user)
}

// Logout revokes the specified refresh token
func (s *AuthService) Logout(ctx context.Context, refreshTokenStr string) error {
	refreshTokenStr = strings.TrimSpace(refreshTokenStr)
	if refreshTokenStr == "" {
		return domain.ErrInvalidInput
	}

	if s.tokenRepo == nil {
		return nil
	}

	_ = s.tokenRepo.Revoke(ctx, refreshTokenStr)
	return nil
}

func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	return claims, nil
}
