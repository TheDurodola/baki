package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheDurodola/baki/internal/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PaymentRepository struct {
	col *mongo.Collection
}

func NewPaymentRepository(db *mongo.Database) *PaymentRepository {
	return &PaymentRepository{
		col: db.Collection("payments"),
	}
}

func (r *PaymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	_, err := r.col.InsertOne(ctx, p)
	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

func (r *PaymentRepository) GetByReference(ctx context.Context, ref string) (*domain.Payment, error) {
	var p domain.Payment
	err := r.col.FindOne(ctx, bson.M{"reference": ref}).Decode(&p)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get payment by reference: %w", err)
	}
	return &p, nil
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, ref string, status domain.PaymentStatus, subID *uuid.UUID, paidAt *time.Time) error {
	setFields := bson.M{
		"status":  status,
		"paid_at": paidAt,
	}
	if subID != nil {
		setFields["subscription_id"] = subID
	}

	update := bson.M{"$set": setFields}
	res, err := r.col.UpdateOne(ctx, bson.M{"reference": ref}, update)
	if err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PaymentRepository) List(ctx context.Context, userID *uuid.UUID, limit, offset int) ([]*domain.Payment, error) {
	filter := bson.M{}
	if userID != nil {
		filter["user_id"] = *userID
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer cursor.Close(ctx)

	var payments []*domain.Payment
	if err := cursor.All(ctx, &payments); err != nil {
		return nil, fmt.Errorf("decode payments: %w", err)
	}

	return payments, nil
}
