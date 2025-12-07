// Package s3service provides S3 operations for the loan eligibility engine
package s3service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"

	appConfig "loan-eligibility-engine/internal/config"
	"loan-eligibility-engine/internal/utils"
)

// Service handles S3 operations
type Service struct {
	client     *s3.Client
	presigner  *s3.PresignClient
	bucketName string
}

// PresignedURLResult contains the presigned URL details
type PresignedURLResult struct {
	URL       string    `json:"url"`
	Key       string    `json:"key"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewService creates a new S3 service
func NewService(ctx context.Context) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	appCfg, err := appConfig.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load app config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	presigner := s3.NewPresignClient(client)

	return &Service{
		client:     client,
		presigner:  presigner,
		bucketName: appCfg.S3Bucket,
	}, nil
}

// GeneratePresignedUploadURL creates a presigned URL for uploading files
func (s *Service) GeneratePresignedUploadURL(ctx context.Context, key string, contentType string, expiryMinutes int) (*PresignedURLResult, error) {
	if expiryMinutes <= 0 {
		expiryMinutes = 15 // Default 15 minutes
	}

	expiry := time.Duration(expiryMinutes) * time.Minute

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}

	presignedReq, err := s.presigner.PresignPutObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		utils.Logger.Error("Failed to generate presigned URL",
			zap.String("bucket", s.bucketName),
			zap.String("key", key),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	utils.Logger.Info("Generated presigned upload URL",
		zap.String("bucket", s.bucketName),
		zap.String("key", key),
		zap.Int("expiry_minutes", expiryMinutes),
	)

	return &PresignedURLResult{
		URL:       presignedReq.URL,
		Key:       key,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

// GeneratePresignedDownloadURL creates a presigned URL for downloading files
func (s *Service) GeneratePresignedDownloadURL(ctx context.Context, key string, expiryMinutes int) (*PresignedURLResult, error) {
	if expiryMinutes <= 0 {
		expiryMinutes = 15
	}

	expiry := time.Duration(expiryMinutes) * time.Minute

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	presignedReq, err := s.presigner.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned download URL: %w", err)
	}

	return &PresignedURLResult{
		URL:       presignedReq.URL,
		Key:       key,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

// DownloadFile downloads a file from S3
func (s *Service) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		utils.Logger.Error("Failed to download file from S3",
			zap.String("bucket", s.bucketName),
			zap.String("key", key),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	utils.Logger.Info("Downloaded file from S3",
		zap.String("bucket", s.bucketName),
		zap.String("key", key),
		zap.Int("size", len(data)),
	)

	return data, nil
}

// UploadFile uploads a file to S3
func (s *Service) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		utils.Logger.Error("Failed to upload file to S3",
			zap.String("bucket", s.bucketName),
			zap.String("key", key),
			zap.Error(err),
		)
		return fmt.Errorf("failed to upload file: %w", err)
	}

	utils.Logger.Info("Uploaded file to S3",
		zap.String("bucket", s.bucketName),
		zap.String("key", key),
		zap.Int("size", len(data)),
	)

	return nil
}

// DeleteFile deletes a file from S3
func (s *Service) DeleteFile(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	utils.Logger.Info("Deleted file from S3",
		zap.String("bucket", s.bucketName),
		zap.String("key", key),
	)

	return nil
}

// ListFiles lists files in the bucket with optional prefix
func (s *Service) ListFiles(ctx context.Context, prefix string, maxKeys int32) ([]types.Object, error) {
	if maxKeys <= 0 {
		maxKeys = 100
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucketName),
		MaxKeys: aws.Int32(maxKeys),
	}

	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	result, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return result.Contents, nil
}

// FileExists checks if a file exists in S3
func (s *Service) FileExists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	_, err := s.client.HeadObject(ctx, input)
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}

	return true, nil
}

// CopyFile copies a file within S3
func (s *Service) CopyFile(ctx context.Context, sourceKey, destKey string) error {
	input := &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucketName),
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucketName, sourceKey)),
		Key:        aws.String(destKey),
	}

	_, err := s.client.CopyObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	utils.Logger.Info("Copied file in S3",
		zap.String("source", sourceKey),
		zap.String("destination", destKey),
	)

	return nil
}

// MoveFile moves a file within S3 (copy + delete)
func (s *Service) MoveFile(ctx context.Context, sourceKey, destKey string) error {
	if err := s.CopyFile(ctx, sourceKey, destKey); err != nil {
		return err
	}

	return s.DeleteFile(ctx, sourceKey)
}
