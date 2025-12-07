// Package handlers provides HTTP handlers for the loan eligibility engine.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"loan-eligibility-engine/internal/utils"
)

// WebhookTriggerHandler handles requests to trigger n8n webhooks.
type WebhookTriggerHandler struct {
	matchingWebhookURL     string
	notificationWebhookURL string
	crawlerWebhookURL      string
}

// NewWebhookTriggerHandler creates a new webhook trigger handler.
func NewWebhookTriggerHandler() *WebhookTriggerHandler {
	return &WebhookTriggerHandler{
		matchingWebhookURL:     os.Getenv("N8N_MATCHING_WEBHOOK_URL"),
		notificationWebhookURL: os.Getenv("N8N_NOTIFICATION_WEBHOOK_URL"),
		crawlerWebhookURL:      os.Getenv("N8N_CRAWLER_WEBHOOK_URL"),
	}
}

// TriggerRequest is the request body for triggering a webhook.
type TriggerRequest struct {
	WorkflowType string                 `json:"workflow_type"`
	BatchID      string                 `json:"batch_id"`
	ExtraParams  map[string]interface{} `json:"extra_params,omitempty"`
}

// TriggerResponse is the response for a webhook trigger request.
type TriggerResponse struct {
	Message         string      `json:"message"`
	BatchID         string      `json:"batch_id"`
	WebhookResponse interface{} `json:"webhook_response,omitempty"`
}

// Handle processes API Gateway requests to trigger webhooks.
func (h *WebhookTriggerHandler) Handle(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger := utils.GetLogger()

	// CORS headers
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "Content-Type,Authorization",
		"Access-Control-Allow-Methods": "POST,OPTIONS",
		"Content-Type":                 "application/json",
	}

	// Handle CORS preflight
	if request.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    headers,
		}, nil
	}

	// Parse request body
	var req TriggerRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(headers, http.StatusBadRequest, "Invalid JSON in request body")
	}

	// Validate request
	if req.BatchID == "" {
		return errorResponse(headers, http.StatusBadRequest, "Missing required field: batch_id")
	}

	if req.WorkflowType == "" {
		req.WorkflowType = "matching"
	}

	// Get webhook URL based on workflow type
	webhookURL := h.getWebhookURL(req.WorkflowType)
	if webhookURL == "" {
		return errorResponse(headers, http.StatusBadRequest, fmt.Sprintf("Unknown workflow type: %s", req.WorkflowType))
	}

	// Prepare payload
	payload := map[string]interface{}{
		"batch_id":      req.BatchID,
		"workflow_type": req.WorkflowType,
		"source":        "lambda_trigger",
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	// Merge extra params
	for k, v := range req.ExtraParams {
		payload[k] = v
	}

	// Trigger webhook
	webhookResp, err := h.triggerWebhook(ctx, webhookURL, payload)
	if err != nil {
		logger.Error("Failed to trigger webhook",
			utils.String("workflowType", req.WorkflowType),
			utils.Error(err))
		return errorResponse(headers, http.StatusInternalServerError, fmt.Sprintf("Failed to trigger workflow: %v", err))
	}

	logger.Info("Successfully triggered webhook",
		utils.String("workflowType", req.WorkflowType),
		utils.String("batchID", req.BatchID))

	response := TriggerResponse{
		Message:         fmt.Sprintf("Successfully triggered %s workflow", req.WorkflowType),
		BatchID:         req.BatchID,
		WebhookResponse: webhookResp,
	}

	body, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

// getWebhookURL returns the appropriate webhook URL for the workflow type.
func (h *WebhookTriggerHandler) getWebhookURL(workflowType string) string {
	switch workflowType {
	case "matching":
		return h.matchingWebhookURL
	case "notification":
		return h.notificationWebhookURL
	case "crawler":
		return h.crawlerWebhookURL
	default:
		return ""
	}
}

// triggerWebhook sends a POST request to the n8n webhook.
func (h *WebhookTriggerHandler) triggerWebhook(ctx context.Context, webhookURL string, payload map[string]interface{}) (interface{}, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	// Try to parse response as JSON
	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// If not JSON, return nil
		return nil, nil
	}

	return result, nil
}
