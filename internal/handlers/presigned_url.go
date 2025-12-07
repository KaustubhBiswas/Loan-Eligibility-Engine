// Package handlers provides HTTP handlers for the loan eligibility engine.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"loan-eligibility-engine/internal/utils"
)

// PresignedURLHandler handles requests for generating presigned S3 URLs.
type PresignedURLHandler struct {
	s3Client   *s3.Client
	bucketName string
}

// NewPresignedURLHandler creates a new presigned URL handler.
func NewPresignedURLHandler() (*PresignedURLHandler, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	return &PresignedURLHandler{
		s3Client:   s3.NewFromConfig(cfg),
		bucketName: os.Getenv("S3_BUCKET"),
	}, nil
}

// PresignedURLResponse is the response structure for presigned URL requests.
type PresignedURLResponse struct {
	UploadURL string `json:"uploadUrl"`
	S3Key     string `json:"s3Key"`
	ExpiresIn int    `json:"expiresIn"`
}

// Handle processes the API Gateway request for generating presigned URLs.
func (h *PresignedURLHandler) Handle(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger := utils.GetLogger()

	// CORS headers
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "Content-Type,Authorization",
		"Access-Control-Allow-Methods": "GET,OPTIONS",
		"Content-Type":                 "application/json",
	}

	// Handle CORS preflight
	if request.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    headers,
		}, nil
	}

	// Get filename from query params
	filename := request.QueryStringParameters["filename"]
	if filename == "" {
		filename = "upload_" + uuid.New().String()[:8] + ".csv"
	}

	// Validate filename
	if len(filename) < 4 || filename[len(filename)-4:] != ".csv" {
		return errorResponse(headers, http.StatusBadRequest, "Only CSV files are allowed")
	}

	// Generate unique S3 key
	timestamp := time.Now().UTC().Format("2006/01/02")
	s3Key := "uploads/" + timestamp + "/" + uuid.New().String() + "_" + sanitizeFilename(filename)

	// Create presign client
	presignClient := s3.NewPresignClient(h.s3Client)

	// Generate presigned PUT URL
	presignedReq, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(h.bucketName),
		Key:         aws.String(s3Key),
		ContentType: aws.String("text/csv"),
	}, s3.WithPresignExpires(time.Hour))

	if err != nil {
		logger.Error("Failed to generate presigned URL", utils.Error(err))
		return errorResponse(headers, http.StatusInternalServerError, "Failed to generate upload URL")
	}

	response := PresignedURLResponse{
		UploadURL: presignedReq.URL,
		S3Key:     s3Key,
		ExpiresIn: 3600,
	}

	body, _ := json.Marshal(response)

	logger.Info("Generated presigned URL",
		utils.String("s3Key", s3Key),
		utils.String("bucket", h.bucketName))

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

// sanitizeFilename removes unsafe characters from filename.
func sanitizeFilename(filename string) string {
	// Simple sanitization - in production, use a more robust approach
	safe := ""
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			safe += string(r)
		}
	}
	if len(safe) > 100 {
		safe = safe[:100]
	}
	return safe
}

// errorResponse creates an error response.
func errorResponse(headers map[string]string, statusCode int, message string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"error":   http.StatusText(statusCode),
		"message": message,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}
