package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/TheDurodola/baki/internal/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SubscriptionRepository struct {
	col     *mongo.Collection
	planCol *mongo.Collection
}

func NewSubscriptionRepository(db *mongo.Database) *SubscriptionRepository {
	return &SubscriptionRepository{
		col:     db.Collection("subscriptions"),
		planCol: db.Collection("plans"),
	}
}

func (r *SubscriptionRepository) populatePlan(ctx context.Context, s *domain.Subscription) {
	if s == nil || s.PlanID == uuid.Nil {
		return
	}
	var plan domain.MembershipPlan
	if err := r.planCol.FindOne(ctx, bson.M{"_id": s.PlanID}).Decode(&plan); err == nil {
		s.Plan = &plan
	}
}

func (r *SubscriptionRepository) Create(ctx context.Context, s *domain.Subscription) error {
	_, err := r.col.InsertOne(ctx, s)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	return nil
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	var s domain.Subscription
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&s)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get subscription by id: %w", err)
	}
	r.populatePlan(ctx, &s)
	return &s, nil
}

// GetActiveByUserID retrieves currently valid active subscription for a member (strict: status='active' AND end_date > NOW())
func (r *SubscriptionRepository) GetActiveByUserID(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	filter := bson.M{
		"user_id":  userID,
		"status":   domain.SubStatusActive,
		"end_date": bson.M{"$gt": time.Now().UTC()},
	}

	findOpts := options.FindOne().SetSort(bson.D{{Key: "end_date", Value: -1}})

	var s domain.Subscription
	err := r.col.FindOne(ctx, filter, findOpts).Decode(&s)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNoActiveSubscription
		}
		return nil, fmt.Errorf("get active subscription: %w", err)
	}
	r.populatePlan(ctx, &s)
	return &s, nil
}

// GetLatestByUserID retrieves the most recent subscription regardless of status
func (r *SubscriptionRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	filter := bson.M{"user_id": userID}
	findOpts := options.FindOne().SetSort(bson.D{{Key: "end_date", Value: -1}})

	var s domain.Subscription
	err := r.col.FindOne(ctx, filter, findOpts).Decode(&s)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get latest subscription: %w", err)
	}
	r.populatePlan(ctx, &s)
	return &s, nil
}

func (r *SubscriptionRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Subscription, error) {
	filter := bson.M{"user_id": userID}
	findOpts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer cursor.Close(ctx)

	var subs []*domain.Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, fmt.Errorf("decode subscriptions: %w", err)
	}

	for _, s := range subs {
		r.populatePlan(ctx, s)
	}

	return subs, nil
}

// ExpirePassedSubscriptions marks all subscriptions whose end_date is in the past as expired
func (r *SubscriptionRepository) ExpirePassedSubscriptions(ctx context.Context) (int64, error) {
	filter := bson.M{
		"status":   domain.SubStatusActive,
		"end_date": bson.M{"$lte": time.Now().UTC()},
	}
	update := bson.M{
		"$set": bson.M{
			"status":     domain.SubStatusExpired,
			"updated_at": time.Now().UTC(),
		},
	}

	res, err := r.col.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("expire passed subscriptions: %w", err)
	}
	return res.ModifiedCount, nil
}

type expiredAggregationDoc struct {
	Metadata []struct {
		Total int `bson:"total"`
	} `bson:"metadata"`
	Data []struct {
		UserID               uuid.UUID              `bson:"_id"`
		LatestSubscriptionID uuid.UUID              `bson:"latest_subscription_id"`
		Plan                 *domain.MembershipPlan `bson:"plan"`
		User                 *domain.User           `bson:"user"`
		EndDate              time.Time              `bson:"end_date"`
	} `bson:"data"`
}

// ListExpiredMembers retrieves all members whose latest subscription has expired (end_date <= now)
func (r *SubscriptionRepository) ListExpiredMembers(ctx context.Context, page, pageSize int, search string) ([]*domain.ExpiredMemberItem, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	now := time.Now().UTC()

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$sort", Value: bson.D{{Key: "end_date", Value: -1}}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":                    "$user_id",
			"latest_subscription_id": bson.M{"$first": "$_id"},
			"plan_id":                bson.M{"$first": "$plan_id"},
			"status":                 bson.M{"$first": "$status"},
			"start_date":             bson.M{"$first": "$start_date"},
			"end_date":               bson.M{"$first": "$end_date"},
		}}},
		bson.D{{Key: "$match", Value: bson.M{"end_date": bson.M{"$lte": now}}}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from":         "users",
			"localField":   "_id",
			"foreignField": "_id",
			"as":           "user",
		}}},
		bson.D{{Key: "$unwind", Value: "$user"}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from":         "plans",
			"localField":   "plan_id",
			"foreignField": "_id",
			"as":           "plan",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{"path": "$plan", "preserveNullAndEmptyArrays": true}}},
	}

	search = strings.TrimSpace(search)
	if search != "" {
		regexPattern := bson.M{"$regex": regexp.QuoteMeta(search), "$options": "i"}
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{
			"$or": []bson.M{
				{"user.first_name": regexPattern},
				{"user.last_name": regexPattern},
				{"user.phone": regexPattern},
				{"user.member_code": regexPattern},
				{"user.email": regexPattern},
			},
		}}})
	}

	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{{Key: "end_date", Value: -1}}}},
		bson.D{{Key: "$facet", Value: bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data": []bson.M{
				{"$skip": int64(offset)},
				{"$limit": int64(pageSize)},
			},
		}}},
	)

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, fmt.Errorf("aggregate expired members: %w", err)
	}
	defer cursor.Close(ctx)

	var results []expiredAggregationDoc
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, fmt.Errorf("decode expired members: %w", err)
	}

	if len(results) == 0 {
		return []*domain.ExpiredMemberItem{}, 0, nil
	}

	total := 0
	if len(results[0].Metadata) > 0 {
		total = results[0].Metadata[0].Total
	}

	var items []*domain.ExpiredMemberItem
	for _, row := range results[0].Data {
		diff := now.Sub(row.EndDate)
		daysExpired := int(diff.Hours() / 24)
		if daysExpired == 0 && diff > 0 {
			daysExpired = 1
		}

		items = append(items, &domain.ExpiredMemberItem{
			User:                 row.User,
			LatestSubscriptionID: row.LatestSubscriptionID,
			Plan:                 row.Plan,
			ExpiredAt:            row.EndDate,
			DaysExpired:          daysExpired,
		})
	}

	return items, total, nil
}
