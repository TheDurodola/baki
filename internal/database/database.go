package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Database struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Connect establishes a connection to MongoDB and sets up indexes
func Connect(ctx context.Context, uri, dbName string) (*Database, error) {
	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	db := client.Database(dbName)
	d := &Database{
		Client: client,
		DB:     db,
	}

	if err := d.EnsureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("ensure indexes: %w", err)
	}

	if err := d.SeedDefaultPlans(ctx); err != nil {
		return nil, fmt.Errorf("seed default plans: %w", err)
	}

	return d, nil
}

// SeedDefaultPlans seeds the standard gym membership plans if they do not already exist
func (d *Database) SeedDefaultPlans(ctx context.Context) error {
	plansCol := d.DB.Collection("plans")
	count, err := plansCol.CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("count plans: %w", err)
	}

	// Only seed if no plans exist yet
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()
	defaultPlans := []interface{}{
		bson.M{
			"_id":           uuid.New(),
			"name":          "Monthly",
			"description":   "Standard 1-Month Gym Access (₦20,000)",
			"duration_days": 30,
			"price_cents":   2000000, // ₦20,000 * 100
			"currency":      "NGN",
			"is_active":     true,
			"created_at":    now,
			"updated_at":    now,
		},
		bson.M{
			"_id":           uuid.New(),
			"name":          "3 Months",
			"description":   "Quarterly 3-Month Gym Access (₦50,000)",
			"duration_days": 90,
			"price_cents":   5000000, // ₦50,000 * 100
			"currency":      "NGN",
			"is_active":     true,
			"created_at":    now,
			"updated_at":    now,
		},
		bson.M{
			"_id":           uuid.New(),
			"name":          "6 Months",
			"description":   "Biannual 6-Month Gym Access (₦100,000)",
			"duration_days": 180,
			"price_cents":   10000000, // ₦100,000 * 100
			"currency":      "NGN",
			"is_active":     true,
			"created_at":    now,
			"updated_at":    now,
		},
		bson.M{
			"_id":           uuid.New(),
			"name":          "1 Year",
			"description":   "Annual 1-Year Gym Access (₦180,000)",
			"duration_days": 365,
			"price_cents":   18000000, // ₦180,000 * 100
			"currency":      "NGN",
			"is_active":     true,
			"created_at":    now,
			"updated_at":    now,
		},
	}

	if _, err := plansCol.InsertMany(ctx, defaultPlans); err != nil {
		return fmt.Errorf("insert default plans: %w", err)
	}

	return nil
}

// EnsureIndexes creates required indexes for fast lookups and unique constraints
func (d *Database) EnsureIndexes(ctx context.Context) error {
	// Users collection indexes
	usersCol := d.DB.Collection("users")
	userIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "phone", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "member_code", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	if _, err := usersCol.Indexes().CreateMany(ctx, userIndexes); err != nil {
		return fmt.Errorf("create user indexes: %w", err)
	}

	// Subscriptions collection indexes
	subsCol := d.DB.Collection("subscriptions")
	subIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "end_date", Value: -1}},
		},
	}
	if _, err := subsCol.Indexes().CreateMany(ctx, subIndexes); err != nil {
		return fmt.Errorf("create subscription indexes: %w", err)
	}

	// Payments collection indexes
	paymentsCol := d.DB.Collection("payments")
	paymentIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "reference", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
	}
	if _, err := paymentsCol.Indexes().CreateMany(ctx, paymentIndexes); err != nil {
		return fmt.Errorf("create payment indexes: %w", err)
	}

	// Refresh tokens collection indexes
	refreshTokensCol := d.DB.Collection("refresh_tokens")
	refreshTokenIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "token", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "expires_at", Value: 1}},
		},
	}
	if _, err := refreshTokensCol.Indexes().CreateMany(ctx, refreshTokenIndexes); err != nil {
		return fmt.Errorf("create refresh token indexes: %w", err)
	}

	return nil
}

// Close closes the MongoDB client connection
func (d *Database) Close(ctx context.Context) error {
	return d.Client.Disconnect(ctx)
}
