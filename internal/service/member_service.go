package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bolaji/baki/internal/domain"
	"github.com/bolaji/baki/internal/gateway"
	"github.com/bolaji/baki/internal/repository"
	"github.com/google/uuid"
)

type MemberService struct {
	userRepo   *repository.UserRepository
	cloudinary *gateway.CloudinaryGateway
}

func NewMemberService(userRepo *repository.UserRepository, cloudinary *gateway.CloudinaryGateway) *MemberService {
	return &MemberService{
		userRepo:   userRepo,
		cloudinary: cloudinary,
	}
}

func (s *MemberService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

func (s *MemberService) GetByMemberCode(ctx context.Context, code string) (*domain.User, error) {
	return s.userRepo.GetByMemberCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
}

func (s *MemberService) ListMembers(ctx context.Context, page, pageSize int, search string) ([]*domain.User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	return s.userRepo.ListMembers(ctx, pageSize, offset, search)
}

type UpdateMemberInput struct {
	Email            string                   `json:"email"`
	Phone            string                   `json:"phone"`
	FirstName        string                   `json:"first_name"`
	LastName         string                   `json:"last_name"`
	AvatarURL        string                   `json:"avatar_url"`
	EmergencyContact *domain.EmergencyContact `json:"emergency_contact,omitempty"`
	Status           *domain.UserStatus       `json:"status,omitempty"`
}

func (s *MemberService) UpdateMember(ctx context.Context, id uuid.UUID, input UpdateMemberInput) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Email != "" {
		user.Email = strings.ToLower(strings.TrimSpace(input.Email))
	}
	if input.Phone != "" {
		user.Phone = strings.TrimSpace(input.Phone)
	}
	if input.FirstName != "" {
		user.FirstName = strings.TrimSpace(input.FirstName)
	}
	if input.LastName != "" {
		user.LastName = strings.TrimSpace(input.LastName)
	}
	if input.AvatarURL != "" {
		user.AvatarURL = strings.TrimSpace(input.AvatarURL)
	}
	if input.EmergencyContact != nil {
		rel := domain.Relationship(strings.ToLower(strings.TrimSpace(string(input.EmergencyContact.Relationship))))
		if !rel.IsValid() {
			return nil, domain.ErrInvalidRelationship
		}
		input.EmergencyContact.Relationship = rel
		user.EmergencyContact = *input.EmergencyContact
	}
	if input.Status != nil {
		user.Status = *input.Status
	}
	user.UpdatedAt = time.Now().UTC()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update member: %w", err)
	}

	return user, nil
}

// UploadAvatar uploads profile picture to Cloudinary and saves URL on member profile
func (s *MemberService) UploadAvatar(ctx context.Context, userID uuid.UUID, file io.Reader, filename string) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if s.cloudinary == nil {
		return nil, fmt.Errorf("cloudinary gateway is not configured")
	}

	avatarURL, err := s.cloudinary.UploadImage(ctx, file, filename)
	if err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}

	if err := s.userRepo.UpdateAvatar(ctx, user.ID, avatarURL); err != nil {
		return nil, fmt.Errorf("update member avatar url: %w", err)
	}

	user.AvatarURL = avatarURL
	user.UpdatedAt = time.Now().UTC()
	return user, nil
}
