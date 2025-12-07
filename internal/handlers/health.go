// Package handlers provides HTTP handlers for the loan eligibility engine.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"

	appConfig "loan-eligibility-engine/internal/config"
	"loan-eligibility-engine/internal/services/database"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	db *database.DB
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler() (*HealthHandler, error) {
	cfg, err := appConfig.Load()
	if err != nil {
		return &HealthHandler{}, nil // Return handler without DB
	}

	db, err := database.New(cfg)
	if err != nil {
		return &HealthHandler{}, nil // Return handler without DB
	}

	return &HealthHandler{db: db}, nil
}

// HealthResponse is the response structure for health checks.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Stage     string `json:"stage"`
	Database  string `json:"database,omitempty"`
}

// Handle processes health check requests.
func (h *HealthHandler) Handle(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := map[string]string{
		"Access-Control-Allow-Origin": "*",
		"Content-Type":                "application/json",
	}

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "loan-eligibility-engine",
		Version:   getEnvOrDefault("SERVICE_VERSION", "1.0.0"),
		Stage:     getEnvOrDefault("STAGE", "unknown"),
	}

	// Check database connectivity
	if h.db != nil {
		if err := h.db.HealthCheck(ctx); err != nil {
			response.Database = "disconnected"
			response.Status = "degraded"
		} else {
			response.Database = "connected"
		}
	} else {
		response.Database = "not configured"
	}

	statusCode := http.StatusOK
	if response.Status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	body, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

// Close cleans up resources.
func (h *HealthHandler) Close() {
	if h.db != nil {
		h.db.Close()
	}
}

// getEnvOrDefault returns environment variable or default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
