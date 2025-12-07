// Package handlers provides HTTP handlers for the loan eligibility engine.
package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appConfig "loan-eligibility-engine/internal/config"
	"loan-eligibility-engine/internal/services/database"
	"loan-eligibility-engine/internal/utils"
)

// CSVProcessorHandler handles S3 events for CSV processing.
type CSVProcessorHandler struct {
	s3Client   *s3.Client
	db         *database.DB
	userRepo   *database.UserRepository
	webhookURL string
}

// NewCSVProcessorHandler creates a new CSV processor handler.
func NewCSVProcessorHandler() (*CSVProcessorHandler, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	cfg, err := appConfig.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load app config: %w", err)
	}

	db, err := database.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &CSVProcessorHandler{
		s3Client:   s3.NewFromConfig(awsCfg),
		db:         db,
		userRepo:   database.NewUserRepository(db),
		webhookURL: cfg.N8NWebhookURL,
	}, nil
}

// CSVProcessResult is the result of processing a CSV file.
type CSVProcessResult struct {
	Message  string   `json:"message"`
	BatchID  string   `json:"batch_id"`
	Inserted int      `json:"inserted"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// Handle processes S3 events for uploaded CSV files.
func (h *CSVProcessorHandler) Handle(ctx context.Context, s3Event events.S3Event) (CSVProcessResult, error) {
	logger := utils.GetLogger()

	if len(s3Event.Records) == 0 {
		return CSVProcessResult{Message: "No records to process"}, nil
	}

	record := s3Event.Records[0]
	bucket := record.S3.Bucket.Name
	key, err := url.QueryUnescape(record.S3.Object.Key)
	if err != nil {
		return CSVProcessResult{}, fmt.Errorf("failed to decode S3 key: %w", err)
	}

	logger.Info("Processing CSV file",
		utils.String("bucket", bucket),
		utils.String("key", key))

	// Download CSV from S3
	csvContent, err := h.downloadCSV(ctx, bucket, key)
	if err != nil {
		logger.Error("Failed to download CSV", utils.Error(err))
		return CSVProcessResult{}, fmt.Errorf("failed to download CSV: %w", err)
	}

	// Generate batch ID
	batchID := generateBatchID(key)

	// Parse CSV
	parser := utils.NewCSVParser()
	users, parseErrors := parser.ParseUsers(csvContent, batchID)

	if len(users) == 0 {
		errMsgs := make([]string, len(parseErrors))
		for i, e := range parseErrors {
			errMsgs[i] = e.Error()
		}
		return CSVProcessResult{
			Message: "No valid users found in CSV",
			BatchID: batchID,
			Errors:  errMsgs,
		}, nil
	}

	logger.Info("Parsed CSV",
		utils.String("batchID", batchID),
		utils.Int("validUsers", len(users)),
		utils.Int("parseErrors", len(parseErrors)))

	// Insert users into database
	result, err := h.userRepo.BulkInsert(ctx, users)
	if err != nil {
		logger.Error("Failed to insert users", utils.Error(err))
		return CSVProcessResult{}, fmt.Errorf("failed to insert users: %w", err)
	}

	logger.Info("Inserted users",
		utils.String("batchID", batchID),
		utils.Int("inserted", result.InsertedCount),
		utils.Int("failed", result.FailedCount))

	// Trigger n8n webhook if users were inserted
	if result.InsertedCount > 0 && h.webhookURL != "" {
		if err := h.triggerWebhook(ctx, batchID, result.InsertedCount); err != nil {
			logger.Warn("Failed to trigger n8n webhook", utils.Error(err))
		}
	}

	// Archive processed file
	if err := h.archiveFile(ctx, bucket, key); err != nil {
		logger.Warn("Failed to archive file", utils.Error(err))
	}

	// Combine parse errors with insert errors
	allErrors := make([]string, 0)
	for _, e := range parseErrors {
		allErrors = append(allErrors, e.Error())
	}
	allErrors = append(allErrors, result.Errors...)

	// Limit errors in response
	if len(allErrors) > 10 {
		allErrors = allErrors[:10]
	}

	return CSVProcessResult{
		Message:  "CSV processed successfully",
		BatchID:  batchID,
		Inserted: result.InsertedCount,
		Failed:   result.FailedCount + len(parseErrors),
		Errors:   allErrors,
	}, nil
}

// downloadCSV downloads CSV content from S3.
func (h *CSVProcessorHandler) downloadCSV(ctx context.Context, bucket, key string) (string, error) {
	output, err := h.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return "", err
	}
	defer output.Body.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, output.Body); err != nil {
		return "", err
	}

	content := buf.String()
	if content == "" {
		return "", fmt.Errorf("CSV file is empty")
	}

	return content, nil
}

// generateBatchID generates a unique batch ID for this upload.
func generateBatchID(key string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	hash := sha256.Sum256([]byte(key + timestamp))
	return hex.EncodeToString(hash[:])[:16]
}

// triggerWebhook triggers the n8n matching workflow.
func (h *CSVProcessorHandler) triggerWebhook(ctx context.Context, batchID string, userCount int) error {
	payload := map[string]interface{}{
		"batch_id":     batchID,
		"user_count":   userCount,
		"trigger_type": "csv_upload",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// archiveFile moves the processed file to an archive folder.
func (h *CSVProcessorHandler) archiveFile(ctx context.Context, bucket, key string) error {
	archiveKey := "processed/" + key
	copySource := bucket + "/" + key

	// Copy to archive
	_, err := h.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     &bucket,
		CopySource: &copySource,
		Key:        &archiveKey,
	})
	if err != nil {
		return fmt.Errorf("failed to copy to archive: %w", err)
	}

	// Delete original
	_, err = h.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("failed to delete original: %w", err)
	}

	return nil
}

// Close cleans up resources.
func (h *CSVProcessorHandler) Close() {
	if h.db != nil {
		h.db.Close()
	}
}

// HandleWithConfig processes S3 events with a custom config (for testing).
func HandleWithConfig(ctx context.Context, s3Event events.S3Event, dbURL, webhookURL string) (CSVProcessResult, error) {
	db, err := database.NewFromURL(dbURL)
	if err != nil {
		return CSVProcessResult{}, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return CSVProcessResult{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	handler := &CSVProcessorHandler{
		s3Client:   s3.NewFromConfig(awsCfg),
		db:         db,
		userRepo:   database.NewUserRepository(db),
		webhookURL: webhookURL,
	}

	return handler.Handle(ctx, s3Event)
}

// GetBucketFromEnv returns the S3 bucket name from environment.
func GetBucketFromEnv() string {
	return os.Getenv("S3_BUCKET")
}
