package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bolaji/baki/internal/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type RefreshTokenRepository struct {
	col *mongo.Collection
}

func NewRefreshTokenRepository(db *mongo.Database) *RefreshTokenRepository {
	return &RefreshTokenRepository{
		col: db.Collection("refresh_tokens"),
	}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, rt *domain.RefreshToken) error {
	_, err := r.col.InsertOne(ctx, rt)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetByToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := r.col.FindOne(ctx, bson.M{"token": token}).Decode(&rt)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &rt, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"revoked_at": now,
		},
	}
	res, err := r.col.UpdateOne(ctx, bson.M{"token": token, "revoked_at": nil}, update)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"revoked_at": now,
		},
	}
	_, err := r.col.UpdateMany(ctx, bson.M{"user_id": userID, "revoked_at": nil}, update)
	if err != nil {
		return fmt.Errorf("revoke all refresh tokens for user: %w", err)
	}
	return nil
}
