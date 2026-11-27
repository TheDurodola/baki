package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                   string
	Env                    string
	MongoURI               string
	MongoDBName            string
	JWTSecret              string
	AccessTokenDuration    time.Duration
	RefreshTokenDuration   time.Duration
	PaystackSecretKey      string
	PaystackPublicKey      string
	PaystackWebhookKey     string
	PaystackCallbackURL    string
	CloudinaryCloudName    string
	CloudinaryAPIKey       string
	CloudinaryAPISecret    string
	CloudinaryUploadFolder string
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	env := getEnv("ENV", "development")
	mongoURI := getEnv("MONGODB_URI", "mongodb://localhost:27017")
	mongoDBName := getEnv("MONGODB_DATABASE", "baki")
	jwtSecret := getEnv("JWT_SECRET", "super-secret-jwt-key-for-gym-subscription-baki")

	accessMinutesStr := getEnv("ACCESS_TOKEN_MINUTES", "15")
	accessMinutes, err := strconv.Atoi(accessMinutesStr)
	if err != nil {
		accessMinutes = 15
	}

	refreshDaysStr := getEnv("REFRESH_TOKEN_DAYS", "7")
	refreshDays, err := strconv.Atoi(refreshDaysStr)
	if err != nil {
		refreshDays = 7
	}

	return &Config{
		Port:                   port,
		Env:                    env,
		MongoURI:               mongoURI,
		MongoDBName:            mongoDBName,
		JWTSecret:              jwtSecret,
		AccessTokenDuration:    time.Duration(accessMinutes) * time.Minute,
		RefreshTokenDuration:   time.Duration(refreshDays) * 24 * time.Hour,
		PaystackSecretKey:      getEnv("PAYSTACK_SECRET_KEY", ""),
		PaystackPublicKey:      getEnv("PAYSTACK_PUBLIC_KEY", ""),
		PaystackWebhookKey:     getEnv("PAYSTACK_WEBHOOK_SECRET", ""),
		PaystackCallbackURL:    getEnv("PAYSTACK_CALLBACK_URL", "http://localhost:8080/api/v1/payments/verify"),
		CloudinaryCloudName:    getEnv("CLOUDINARY_CLOUD_NAME", ""),
		CloudinaryAPIKey:       getEnv("CLOUDINARY_API_KEY", ""),
		CloudinaryAPISecret:    getEnv("CLOUDINARY_API_SECRET", ""),
		CloudinaryUploadFolder: getEnv("CLOUDINARY_UPLOAD_FOLDER", "baki/profiles"),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}
