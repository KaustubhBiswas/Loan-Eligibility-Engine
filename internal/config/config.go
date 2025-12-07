// Package config provides configuration management for the application.
package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration values for the application.
type Config struct {
	// AWS
	AWSRegion string
	S3Bucket  string

	// Database
	DBHost     string
	DBPort     int
	DBName     string
	DBUser     string
	DBPassword string

	// n8n
	N8NWebhookURL             string
	N8NMatchingWebhookURL     string
	N8NNotificationWebhookURL string

	// SES
	SESSenderEmail string

	// AI
	GeminiAPIKey string
	OpenAIAPIKey string

	// Application
	Stage    string
	LogLevel string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists (for local development)
	_ = godotenv.Load()

	cfg := &Config{
		// AWS
		AWSRegion: getEnv("AWS_REGION", "us-east-1"),
		S3Bucket:  getEnv("S3_BUCKET", "loan-eligibility-csv-dev"),

		// Database
		DBHost:     getEnv("DB_HOST", getEnv("LOAN_DB_HOST", "localhost")),
		DBPort:     getEnvInt("DB_PORT", getEnvInt("LOAN_DB_PORT", 5432)),
		DBName:     getEnv("DB_NAME", getEnv("LOAN_DB_NAME", "loan_eligibility")),
		DBUser:     getEnv("DB_USER", getEnv("LOAN_DB_USER", "postgres")),
		DBPassword: getEnv("DB_PASSWORD", getEnv("LOAN_DB_PASSWORD", "")),

		// n8n
		N8NWebhookURL:             getEnv("N8N_WEBHOOK_URL", ""),
		N8NMatchingWebhookURL:     getEnv("N8N_MATCHING_WEBHOOK_URL", ""),
		N8NNotificationWebhookURL: getEnv("N8N_NOTIFICATION_WEBHOOK_URL", ""),

		// SES
		SESSenderEmail: getEnv("SES_SENDER_EMAIL", ""),

		// AI
		GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),

		// Application
		Stage:    getEnv("STAGE", "dev"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	return cfg, nil
}

// DatabaseURL returns the PostgreSQL connection string.
func (c *Config) DatabaseURL() string {
	sslMode := "require" // Use SSL for RDS
	if c.DBHost == "localhost" || c.DBHost == "127.0.0.1" {
		sslMode = "disable" // Disable SSL for local development
	}
	return "postgres://" + c.DBUser + ":" + c.DBPassword + "@" + c.DBHost + ":" + strconv.Itoa(c.DBPort) + "/" + c.DBName + "?sslmode=" + sslMode
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an environment variable as int or returns a default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
