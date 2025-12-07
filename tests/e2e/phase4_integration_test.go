// Package e2e_test contains end-to-end integration tests for Phase 4
package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// Test configuration
var (
	apiBaseURL = getEnvOrDefault("API_BASE_URL", "http://localhost:8080")
	n8nBaseURL = getEnvOrDefault("N8N_BASE_URL", "http://localhost:5678")
)

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// skipIfNotIntegration skips the test if not running integration tests
func skipIfNotIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Integration tests not enabled. Set INTEGRATION_TESTS=true to run")
	}
}

// TestPhase4_HealthCheck tests the API health endpoint
func TestPhase4_HealthCheck(t *testing.T) {
	skipIfNotIntegration(t)

	resp, err := http.Get(apiBaseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data object in response")
	}

	if data["status"] != "healthy" {
		t.Errorf("Expected status=healthy, got %v", data["status"])
	}

	t.Logf("Health check passed: %v", data)
}

// TestPhase4_ProductsAPI tests the products endpoint
func TestPhase4_ProductsAPI(t *testing.T) {
	skipIfNotIntegration(t)

	resp, err := http.Get(apiBaseURL + "/api/products")
	if err != nil {
		t.Fatalf("Products API failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}

	products, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}

	t.Logf("Found %d products", len(products))

	if len(products) == 0 {
		t.Error("Expected at least 1 product")
	}

	// Validate first product structure
	if len(products) > 0 {
		product := products[0].(map[string]interface{})
		requiredFields := []string{"product_name", "provider_name", "interest_rate_min", "loan_amount_min", "min_credit_score"}
		for _, field := range requiredFields {
			if _, ok := product[field]; !ok {
				t.Errorf("Missing field %s in product", field)
			}
		}
	}
}

// TestPhase4_MatchesAPI tests the matches endpoint
func TestPhase4_MatchesAPI(t *testing.T) {
	skipIfNotIntegration(t)

	resp, err := http.Get(apiBaseURL + "/api/matches")
	if err != nil {
		t.Fatalf("Matches API failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}

	matches, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}

	t.Logf("Found %d matches", len(matches))
}

// TestPhase4_N8nMatchingWorkflow tests the n8n user matching workflow
func TestPhase4_N8nMatchingWorkflow(t *testing.T) {
	skipIfNotIntegration(t)

	payload := map[string]interface{}{
		"process_all": true,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(
		n8nBaseURL+"/webhook/match-users",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("Matching workflow failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(respBody))
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("Failed to decode response: %v. Body: %s", err, string(respBody))
	}

	t.Logf("Matching workflow response: %v", result)

	// Check for matches
	if matches, ok := result["matches"].([]interface{}); ok {
		t.Logf("Found %d matches", len(matches))
	}
}

// TestPhase4_N8nNotificationWorkflow tests the n8n notification workflow
func TestPhase4_N8nNotificationWorkflow(t *testing.T) {
	skipIfNotIntegration(t)

	// Skip if no test email configured
	testEmail := os.Getenv("TEST_EMAIL")
	if testEmail == "" {
		t.Skip("TEST_EMAIL not configured")
	}

	payload := map[string]interface{}{
		"user_email": testEmail,
		"user_name":  "Integration Test",
		"match_id":   fmt.Sprintf("test-%d", time.Now().Unix()),
		"matched_products": []map[string]interface{}{
			{
				"product_name":  "Test Loan Product",
				"provider":      "Test Bank",
				"interest_rate": 10.5,
				"min_amount":    50000,
				"max_amount":    500000,
				"match_score":   95,
			},
		},
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(
		n8nBaseURL+"/webhook/notify-user",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("Notification workflow failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(respBody))
		return
	}

	t.Logf("Notification sent successfully: %s", string(respBody))
}

// TestPhase4_FullWorkflow tests the complete end-to-end flow
func TestPhase4_FullWorkflow(t *testing.T) {
	skipIfNotIntegration(t)

	t.Log("=== Phase 4 Full Workflow Test ===")

	// Step 1: Health Check
	t.Log("Step 1: Checking API health...")
	resp, err := http.Get(apiBaseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API not healthy")
	}
	t.Log("✓ API is healthy")

	// Step 2: Verify Products
	t.Log("Step 2: Verifying products...")
	resp, err = http.Get(apiBaseURL + "/api/products")
	if err != nil {
		t.Fatalf("Products API failed: %v", err)
	}
	var productsResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&productsResult)
	resp.Body.Close()

	products := productsResult["data"].([]interface{})
	if len(products) == 0 {
		t.Fatal("No products found")
	}
	t.Logf("✓ Found %d products", len(products))

	// Step 3: Trigger Matching
	t.Log("Step 3: Triggering user matching...")
	payload := map[string]interface{}{"process_all": true}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err = client.Post(
		n8nBaseURL+"/webhook/match-users",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Logf("⚠ Matching workflow not available: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Log("✓ Matching workflow triggered successfully")
		}
	}

	// Step 4: Verify Matches
	t.Log("Step 4: Verifying matches...")
	resp, err = http.Get(apiBaseURL + "/api/matches")
	if err != nil {
		t.Fatalf("Matches API failed: %v", err)
	}
	var matchesResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&matchesResult)
	resp.Body.Close()

	matches := matchesResult["data"].([]interface{})
	t.Logf("✓ Found %d matches", len(matches))

	t.Log("=== Phase 4 Full Workflow Test Complete ===")
}

// BenchmarkProductsAPI benchmarks the products endpoint
func BenchmarkProductsAPI(b *testing.B) {
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		b.Skip("Integration tests not enabled")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(apiBaseURL + "/api/products")
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkMatchesAPI benchmarks the matches endpoint
func BenchmarkMatchesAPI(b *testing.B) {
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		b.Skip("Integration tests not enabled")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(apiBaseURL + "/api/matches")
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
