package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bolaji/baki/internal/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PlanRepository struct {
	col *mongo.Collection
}

func NewPlanRepository(db *mongo.Database) *PlanRepository {
	return &PlanRepository{
		col: db.Collection("plans"),
	}
}

func (r *PlanRepository) Create(ctx context.Context, p *domain.MembershipPlan) error {
	_, err := r.col.InsertOne(ctx, p)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	return nil
}

func (r *PlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.MembershipPlan, error) {
	var p domain.MembershipPlan
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&p)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get plan by id: %w", err)
	}
	return &p, nil
}

func (r *PlanRepository) List(ctx context.Context, onlyActive bool) ([]*domain.MembershipPlan, error) {
	filter := bson.M{}
	if onlyActive {
		filter["is_active"] = true
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "duration_days", Value: 1}})
	cursor, err := r.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer cursor.Close(ctx)

	var plans []*domain.MembershipPlan
	if err := cursor.All(ctx, &plans); err != nil {
		return nil, fmt.Errorf("decode plans: %w", err)
	}

	return plans, nil
}

func (r *PlanRepository) Update(ctx context.Context, p *domain.MembershipPlan) error {
	update := bson.M{
		"$set": bson.M{
			"name":          p.Name,
			"description":   p.Description,
			"duration_days": p.DurationDays,
			"price_cents":   p.PriceCents,
			"currency":      p.Currency,
			"is_active":     p.IsActive,
			"updated_at":    p.UpdatedAt,
		},
	}

	res, err := r.col.UpdateOne(ctx, bson.M{"_id": p.ID}, update)
	if err != nil {
		return fmt.Errorf("update plan: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
