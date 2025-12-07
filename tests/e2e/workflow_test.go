// Package e2e_test contains end-to-end tests
package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Skip e2e tests if not explicitly enabled
func skipIfNotE2E(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("E2E tests not enabled. Set E2E_TESTS=true to run")
	}
}

func TestE2E_PresignedURLFlow(t *testing.T) {
	skipIfNotE2E(t)

	apiURL := os.Getenv("API_GATEWAY_URL")
	if apiURL == "" {
		t.Skip("API_GATEWAY_URL not set")
	}

	// Request presigned URL
	requestBody := map[string]string{
		"filename":     "test-upload.csv",
		"content_type": "text/csv",
	}
	body, _ := json.Marshal(requestBody)

	resp, err := http.Post(apiURL+"/api/presigned-url", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["url"] == nil {
		t.Error("Response should contain presigned URL")
	}
	if result["key"] == nil {
		t.Error("Response should contain S3 key")
	}
}

func TestE2E_HealthEndpoint(t *testing.T) {
	skipIfNotE2E(t)

	apiURL := os.Getenv("API_GATEWAY_URL")
	if apiURL == "" {
		t.Skip("API_GATEWAY_URL not set")
	}

	resp, err := http.Get(apiURL + "/health")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", result["status"])
	}
}

func TestE2E_CSVUploadToS3(t *testing.T) {
	skipIfNotE2E(t)

	apiURL := os.Getenv("API_GATEWAY_URL")
	if apiURL == "" {
		t.Skip("API_GATEWAY_URL not set")
	}

	// Step 1: Get presigned URL
	requestBody := map[string]string{
		"filename":     "e2e-test-" + time.Now().Format("20060102150405") + ".csv",
		"content_type": "text/csv",
	}
	body, _ := json.Marshal(requestBody)

	resp, err := http.Post(apiURL+"/api/presigned-url", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Get presigned URL failed: %v", err)
	}
	defer resp.Body.Close()

	var presignedResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&presignedResult)

	presignedURL, ok := presignedResult["url"].(string)
	if !ok || presignedURL == "" {
		t.Fatal("Failed to get presigned URL")
	}

	// Step 2: Upload CSV to S3
	csvContent := `name,email,age,annual_income,credit_score,employment_status,loan_amount_required,location
E2E Test User,e2e@test.com,30,500000,750,salaried,200000,Mumbai`

	req, _ := http.NewRequest("PUT", presignedURL, strings.NewReader(csvContent))
	req.Header.Set("Content-Type", "text/csv")

	client := &http.Client{Timeout: 30 * time.Second}
	uploadResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("S3 upload failed: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(uploadResp.Body)
		t.Errorf("S3 upload failed with status %d: %s", uploadResp.StatusCode, string(bodyBytes))
	}
}

func TestE2E_N8NWebhook(t *testing.T) {
	skipIfNotE2E(t)

	n8nURL := os.Getenv("N8N_WEBHOOK_URL")
	if n8nURL == "" {
		t.Skip("N8N_WEBHOOK_URL not set")
	}

	// Test user matching webhook
	payload := map[string]interface{}{
		"user_ids": []string{"test-user-1", "test-user-2"},
		"batch_id": "test-batch-" + time.Now().Format("20060102150405"),
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(n8nURL+"/user-matching", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("n8n webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	// n8n should respond with 200
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("n8n webhook returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

// MockServer for local testing without actual API
type MockServer struct {
	*httptest.Server
}

func NewMockServer() *MockServer {
	mux := http.NewServeMux()

	// Mock presigned URL endpoint
	mux.HandleFunc("/api/presigned-url", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := map[string]interface{}{
			"url":        "https://bucket.s3.amazonaws.com/uploads/test.csv?presigned",
			"key":        "uploads/test.csv",
			"expires_at": time.Now().Add(15 * time.Minute).Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Mock health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"services": map[string]string{
				"database": "connected",
				"s3":       "connected",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Mock matching endpoint
	mux.HandleFunc("/api/matching/process", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":             "completed",
			"total_users":        10,
			"matches_found":      25,
			"processing_time_ms": 1500,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	server := httptest.NewServer(mux)
	return &MockServer{Server: server}
}

func TestMockServer_PresignedURL(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	requestBody := map[string]string{
		"filename":     "test.csv",
		"content_type": "text/csv",
	}
	body, _ := json.Marshal(requestBody)

	resp, err := http.Post(server.URL+"/api/presigned-url", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["url"] == nil {
		t.Error("Response should contain URL")
	}
	if result["key"] == nil {
		t.Error("Response should contain key")
	}
}

func TestMockServer_Health(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", result["status"])
	}
}

func TestMockServer_Matching(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	payload := map[string]interface{}{
		"user_ids": []string{"user-1", "user-2"},
		"batch_id": "batch-123",
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(server.URL+"/api/matching/process", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "completed" {
		t.Errorf("Expected status 'completed', got %v", result["status"])
	}
	if result["matches_found"] == nil {
		t.Error("Response should contain matches_found")
	}
}

func TestFullWorkflow_Mock(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	// Step 1: Health check
	healthResp, _ := http.Get(server.URL + "/health")
	if healthResp.StatusCode != http.StatusOK {
		t.Fatal("Health check failed")
	}
	healthResp.Body.Close()

	// Step 2: Get presigned URL
	presignedBody, _ := json.Marshal(map[string]string{
		"filename":     "workflow-test.csv",
		"content_type": "text/csv",
	})
	presignedResp, _ := http.Post(server.URL+"/api/presigned-url", "application/json", bytes.NewReader(presignedBody))
	if presignedResp.StatusCode != http.StatusOK {
		t.Fatal("Get presigned URL failed")
	}
	presignedResp.Body.Close()

	// Step 3: Trigger matching
	matchingBody, _ := json.Marshal(map[string]interface{}{
		"user_ids": []string{"user-1"},
		"batch_id": "workflow-test-batch",
	})
	matchingResp, _ := http.Post(server.URL+"/api/matching/process", "application/json", bytes.NewReader(matchingBody))
	if matchingResp.StatusCode != http.StatusOK {
		t.Fatal("Matching process failed")
	}

	var matchingResult map[string]interface{}
	json.NewDecoder(matchingResp.Body).Decode(&matchingResult)
	matchingResp.Body.Close()

	if matchingResult["status"] != "completed" {
		t.Errorf("Matching did not complete: %v", matchingResult["status"])
	}

	t.Log("Full workflow completed successfully")
}
