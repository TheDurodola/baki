package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/TheDurodola/baki/internal/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		col: db.Collection("users"),
	}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	_, err := r.col.InsertOne(ctx, u)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			errStr := err.Error()
			if strings.Contains(errStr, "email") {
				return domain.ErrDuplicateEmail
			}
			if strings.Contains(errStr, "phone") {
				return domain.ErrDuplicatePhone
			}
			if strings.Contains(errStr, "member_code") {
				return domain.ErrDuplicateMemberCode
			}
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	escaped := regexp.QuoteMeta(strings.TrimSpace(email))
	filter := bson.M{
		"email": bson.M{"$regex": "^" + escaped + "$", "$options": "i"},
	}

	var u domain.User
	err := r.col.FindOne(ctx, filter).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	var u domain.User
	err := r.col.FindOne(ctx, bson.M{"phone": strings.TrimSpace(phone)}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get user by phone: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByMemberCode(ctx context.Context, code string) (*domain.User, error) {
	escaped := regexp.QuoteMeta(strings.TrimSpace(code))
	filter := bson.M{
		"member_code": bson.M{"$regex": "^" + escaped + "$", "$options": "i"},
	}

	var u domain.User
	err := r.col.FindOne(ctx, filter).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get user by member code: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) ListMembers(ctx context.Context, limit, offset int, search string) ([]*domain.User, int, error) {
	filter := bson.M{"role": domain.RoleMember}

	search = strings.TrimSpace(search)
	if search != "" {
		regexPattern := bson.M{"$regex": regexp.QuoteMeta(search), "$options": "i"}
		filter["$or"] = []bson.M{
			{"first_name": regexPattern},
			{"last_name": regexPattern},
			{"phone": regexPattern},
			{"member_code": regexPattern},
			{"email": regexPattern},
		}
	}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count members: %w", err)
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("find members: %w", err)
	}
	defer cursor.Close(ctx)

	var members []*domain.User
	if err := cursor.All(ctx, &members); err != nil {
		return nil, 0, fmt.Errorf("decode members: %w", err)
	}

	return members, int(total), nil
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	update := bson.M{
		"$set": bson.M{
			"email":             u.Email,
			"phone":             u.Phone,
			"first_name":        u.FirstName,
			"last_name":         u.LastName,
			"avatar_url":        u.AvatarURL,
			"emergency_contact": u.EmergencyContact,
			"status":            u.Status,
			"updated_at":        u.UpdatedAt,
		},
	}

	res, err := r.col.UpdateOne(ctx, bson.M{"_id": u.ID}, update)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdateAvatar(ctx context.Context, id uuid.UUID, avatarURL string) error {
	update := bson.M{
		"$set": bson.M{
			"avatar_url": avatarURL,
		},
	}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
