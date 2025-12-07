// Package main provides a local HTTP server for development and testing
// This server integrates with n8n workflows and provides the API endpoints
// needed by the frontend for CSV upload and processing
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"loan-eligibility-engine/internal/config"
	"loan-eligibility-engine/internal/models"
	"loan-eligibility-engine/internal/services/database"
	"loan-eligibility-engine/internal/services/matcher"
	"loan-eligibility-engine/internal/utils"

	"github.com/rs/cors"
)

// Server holds all dependencies
type Server struct {
	db        *database.DB
	userRepo  *database.UserRepository
	prodRepo  *database.ProductRepository
	matchRepo *database.MatchRepository
	matcher   *matcher.MatcherService
	config    *config.Config
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// UploadResponse contains CSV upload processing results
type UploadResponse struct {
	BatchID      string `json:"batch_id"`
	TotalRows    int    `json:"total_rows"`
	ValidUsers   int    `json:"valid_users"`
	Errors       int    `json:"errors"`
	MatchesFound int    `json:"matches_found"`
	ProcessingMs int64  `json:"processing_ms"`
}

// PresignedURLRequest represents the request for presigned URL
type PresignedURLRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

// PresignedURLResponse contains the presigned URL data
type PresignedURLResponse struct {
	URL     string `json:"url"`
	Key     string `json:"key"`
	Expires int    `json:"expires"`
}

func main() {
	// Initialize logger first
	if err := utils.InitLogger("info"); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer utils.Logger.Sync()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Could not load config from environment: %v", err)
		cfg = &config.Config{}
	}

	// Initialize database
	db, err := database.New(cfg)
	if err != nil {
		log.Printf("Warning: Could not connect to database: %v", err)
		log.Println("Server will run in demo mode without database")
	}

	server := &Server{
		db:     db,
		config: cfg,
	}

	if db != nil {
		server.userRepo = database.NewUserRepository(db)
		server.prodRepo = database.NewProductRepository(db)
		server.matchRepo = database.NewMatchRepository(db)

		// Initialize matcher (may fail if no Gemini API key)
		matcherSvc, err := matcher.NewMatcherService(db)
		if err != nil {
			log.Printf("Warning: Could not initialize matcher service: %v", err)
		}
		server.matcher = matcherSvc
	}

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/api/health", server.healthHandler)

	// Presigned URL endpoint (for S3 uploads)
	mux.HandleFunc("/api/presigned-url", server.presignedURLHandler)

	// Direct CSV upload endpoint (for local testing)
	mux.HandleFunc("/api/upload", server.uploadHandler)

	// Process CSV and match users
	mux.HandleFunc("/api/process", server.processHandler)

	// Get products
	mux.HandleFunc("/api/products", server.productsHandler)

	// Get matches
	mux.HandleFunc("/api/matches", server.matchesHandler)

	// Trigger n8n workflows
	mux.HandleFunc("/api/trigger/crawler", server.triggerCrawlerHandler)
	mux.HandleFunc("/api/trigger/matching", server.triggerMatchingHandler)
	mux.HandleFunc("/api/trigger/notification", server.triggerNotificationHandler)

	// Get users with matches (for notification dropdown)
	mux.HandleFunc("/api/users-with-matches", server.usersWithMatchesHandler)

	// Clear data endpoint
	mux.HandleFunc("/api/clear-data", server.clearDataHandler)

	// Serve static files (frontend)
	mux.HandleFunc("/", server.staticHandler)

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	handler := c.Handler(mux)

	port := getEnvOrDefault("PORT", "8080")
	addr := fmt.Sprintf("0.0.0.0:%s", port)

	log.Printf("Loan Eligibility Engine API Server")
	log.Printf("Listening on http://localhost:%s", port)
	log.Printf("Frontend: http://localhost:%s/", port)
	log.Printf("Health: http://localhost:%s/health", port)
	log.Println("")

	// Start server (this blocks until error)
	log.Printf("Starting HTTP server on %s...", addr)
	serverErr := http.ListenAndServe(addr, handler)
	if serverErr != nil {
		log.Fatalf("Server failed: %v", serverErr)
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	dbStatus := "disconnected"
	if s.db != nil {
		if err := s.db.HealthCheck(r.Context()); err == nil {
			dbStatus = "connected"
		}
	}

	response := Response{
		Success: true,
		Message: "Loan Eligibility Engine API is running",
		Data: map[string]interface{}{
			"status":    "healthy",
			"database":  dbStatus,
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) presignedURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PresignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// For local development, return a mock presigned URL that points to our upload endpoint
	key := fmt.Sprintf("uploads/%d_%s", time.Now().Unix(), req.Filename)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: PresignedURLResponse{
			URL:     fmt.Sprintf("http://localhost:%s/api/upload?key=%s", getEnvOrDefault("PORT", "8080"), key),
			Key:     key,
			Expires: 3600,
		},
	})
}

func (s *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		// Handle presigned URL upload (S3-style)
		s.handlePresignedUpload(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("📤 CSV Upload request received")

	// Handle multipart form upload
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		log.Printf("Failed to parse form: %v", err)
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Failed to parse form: " + err.Error(),
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("No file in form: %v", err)
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "No file provided",
		})
		return
	}
	defer file.Close()

	log.Printf("📄 Processing file: %s (%.2f KB)", header.Filename, float64(header.Size)/1024)

	// Validate file type
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Only CSV files are allowed",
		})
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to read file",
		})
		return
	}

	// Process the CSV
	result, err := s.processCSVContent(r.Context(), content, header.Filename)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "CSV processed successfully",
		Data:    result,
	})
}

func (s *Server) handlePresignedUpload(w http.ResponseWriter, r *http.Request) {
	// Read the raw body (CSV content)
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Store temporarily for processing
	key := r.URL.Query().Get("key")
	filename := filepath.Base(key)
	if filename == "" {
		filename = "upload.csv"
	}

	// Save to temp file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, filename)
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) processHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the key from request
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request",
		})
		return
	}

	// Read from temp file
	filename := filepath.Base(req.Key)
	tempFile := filepath.Join(os.TempDir(), filename)

	content, err := os.ReadFile(tempFile)
	if err != nil {
		writeJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "File not found. Please upload again.",
		})
		return
	}

	// Process
	result, err := s.processCSVContent(r.Context(), content, filename)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Cleanup temp file
	os.Remove(tempFile)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "CSV processed successfully",
		Data:    result,
	})
}

func (s *Server) processCSVContent(ctx context.Context, content []byte, filename string) (*UploadResponse, error) {
	startTime := time.Now()
	batchID := fmt.Sprintf("batch_%d", time.Now().Unix())

	log.Printf("Processing CSV: %s (BatchID: %s)", filename, batchID)

	// Parse CSV
	parser := utils.NewCSVParser()
	users, parseErrors := parser.ParseUsers(string(content), batchID)

	log.Printf("Parsed: %d valid users, %d errors", len(users), len(parseErrors))

	// Log first few errors for debugging
	if len(parseErrors) > 0 {
		log.Printf("Parse errors:")
		for i, err := range parseErrors {
			if i >= 5 { // Only log first 5 errors
				log.Printf("   ... and %d more errors", len(parseErrors)-5)
				break
			}
			log.Printf("   - %v", err)
		}
	}

	result := &UploadResponse{
		BatchID:    batchID,
		TotalRows:  len(users) + len(parseErrors),
		ValidUsers: len(users),
		Errors:     len(parseErrors),
	}

	// If no database connection, return demo results
	if s.db == nil || s.userRepo == nil {
		result.MatchesFound = len(users) * 2 // Demo: assume 2 matches per user
		result.ProcessingMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	// Save users to database and collect IDs
	var userIDs []int64
	for _, u := range users {
		id, err := s.userRepo.Create(ctx, u)
		if err != nil {
			log.Printf("Warning: Could not save user %s: %v", u.Email, err)
			continue
		}
		userIDs = append(userIDs, id)
	}

	log.Printf("💾 Saved %d users to database (IDs: %v)", len(userIDs), userIDs)

	// Run matching if we have a matcher service
	if s.matcher != nil && len(userIDs) > 0 {
		matchResult, err := s.matcher.ProcessNewUsers(ctx, userIDs)
		if err != nil {
			log.Printf("Warning: Matching failed: %v", err)
		} else {
			result.MatchesFound = matchResult.FinalMatches
		}
	}

	result.ProcessingMs = time.Since(startTime).Milliseconds()
	return result, nil
}

func (s *Server) productsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.prodRepo == nil {
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    []models.LoanProduct{},
		})
		return
	}

	products, err := s.prodRepo.GetAllActive(r.Context())
	if err != nil {
		log.Printf("Error fetching products: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to fetch products",
		})
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    products,
	})
}

func (s *Server) matchesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.matchRepo == nil {
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    []models.Match{},
		})
		return
	}

	ctx := r.Context()

	// Get enriched matches with user and product information
	query := `
		SELECT 
			m.id,
			m.user_id,
			m.product_id,
			m.match_score,
			m.status,
			u.user_id as user_name,
			u.email as user_email,
			lp.product_name,
			lp.provider_name
		FROM matches m
		JOIN users u ON m.user_id = u.id
		JOIN loan_products lp ON m.product_id = lp.id
		ORDER BY m.created_at DESC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error fetching matches: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to fetch matches",
		})
		return
	}
	defer rows.Close()

	var matches []map[string]interface{}
	for rows.Next() {
		var id, userID, productID int64
		var matchScore float64
		var status, userName, userEmail, productName, providerName string

		if err := rows.Scan(&id, &userID, &productID, &matchScore, &status, &userName, &userEmail, &productName, &providerName); err != nil {
			log.Printf("Failed to scan match: %v", err)
			continue
		}

		matches = append(matches, map[string]interface{}{
			"id":            id,
			"user_id":       userID,
			"product_id":    productID,
			"match_score":   matchScore,
			"status":        status,
			"user_name":     userName,
			"user_email":    userEmail,
			"product_name":  productName,
			"provider_name": providerName,
		})
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    matches,
	})
}

func (s *Server) triggerCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Trigger n8n crawler workflow
	n8nURL := getEnvOrDefault("N8N_WEBHOOK_URL", "http://localhost:5678")
	webhookURL := fmt.Sprintf("%s/webhook/trigger-crawler", n8nURL)

	resp, err := http.Post(webhookURL, "application/json", strings.NewReader("{}"))
	if err != nil {
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Message: "Crawler trigger sent (n8n may be offline)",
			Data: map[string]interface{}{
				"n8n_url": webhookURL,
				"status":  "queued",
			},
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Crawler workflow triggered",
		Data: map[string]interface{}{
			"n8n_status": resp.StatusCode,
			"response":   string(body),
		},
	})
}

func (s *Server) triggerMatchingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserIDs    []int64 `json:"user_ids"`
		BatchID    string  `json:"batch_id"`
		ProcessAll bool    `json:"process_all"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ProcessAll = true // Default to process all
	}

	// Trigger n8n matching workflow
	n8nURL := getEnvOrDefault("N8N_WEBHOOK_URL", "http://localhost:5678")
	webhookURL := fmt.Sprintf("%s/webhook/match-users", n8nURL)

	log.Printf("Calling n8n webhook: %s", webhookURL)
	payload, _ := json.Marshal(req)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		// Fallback to local matcher
		if s.matcher != nil && len(req.UserIDs) > 0 {
			result, err := s.matcher.ProcessNewUsers(r.Context(), req.UserIDs)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, Response{
					Success: false,
					Error:   "Matching failed: " + err.Error(),
				})
				return
			}

			writeJSON(w, http.StatusOK, Response{
				Success: true,
				Message: "Matching completed (local)",
				Data:    result,
			})
			return
		}

		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Message: "Matching trigger sent (n8n may be offline)",
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Matching workflow triggered",
		Data: map[string]interface{}{
			"n8n_status": resp.StatusCode,
			"response":   string(body),
		},
	})
}

func (s *Server) triggerNotificationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse request body to get user email
	var reqBody struct {
		UserEmail string `json:"user_email"`
		UserName  string `json:"user_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	log.Printf("Notification request for: %s", reqBody.UserEmail)

	// Check if database is available
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "Database not available",
		})
		return
	}

	// Fetch user's matched loans from database (case-insensitive email)
	// Note: Removed status filter to get all matches regardless of status
	query := `
		SELECT 
			u.user_id,
			u.email,
			lp.product_name,
			lp.provider_name,
			lp.interest_rate_min,
			lp.interest_rate_max,
			lp.loan_amount_min,
			lp.loan_amount_max,
			m.match_score
		FROM matches m
		JOIN users u ON m.user_id = u.id
		JOIN loan_products lp ON m.product_id = lp.id
		WHERE LOWER(u.email) = LOWER($1)
		ORDER BY m.match_score DESC
		LIMIT 10
	`

	log.Printf("Querying matches for email: %s", reqBody.UserEmail)
	log.Printf("Query: %s", query)

	rows, err := s.db.QueryContext(ctx, query, reqBody.UserEmail)
	if err != nil {
		log.Printf("Failed to fetch matches: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to fetch user matches",
		})
		return
	}
	defer rows.Close()

	log.Printf("🔍 Rows returned, starting to scan...")

	var matchedProducts []map[string]interface{}
	var userName string
	var rowCount int

	for rows.Next() {
		rowCount++
		var userID, email, productName, providerName string
		var interestMin, interestMax, amountMin, amountMax, matchScore float64

		if err := rows.Scan(&userID, &email, &productName, &providerName,
			&interestMin, &interestMax, &amountMin, &amountMax, &matchScore); err != nil {
			log.Printf("Failed to scan match row %d: %v", rowCount, err)
			continue
		}

		log.Printf("Scanned row %d: userID=%s, email=%s, product=%s", rowCount, userID, email, productName)
		if userName == "" {
			userName = userID // Use user_id as name if not provided
		}

		matchedProducts = append(matchedProducts, map[string]interface{}{
			"product_name":  productName,
			"provider":      providerName,
			"interest_rate": (interestMin + interestMax) / 2, // Average rate
			"min_amount":    amountMin,
			"max_amount":    amountMax,
			"match_score":   int(matchScore),
		})
	}

	log.Printf("🔍 Total rows scanned: %d, Products collected: %d", rowCount, len(matchedProducts))

	if len(matchedProducts) == 0 {
		log.Printf("No matches found in database for: %s", reqBody.UserEmail)

		// Debug: Check if user exists at all
		var userCount int
		countQuery := `SELECT COUNT(*) FROM users WHERE LOWER(email) = LOWER($1)`
		s.db.QueryRowContext(ctx, countQuery, reqBody.UserEmail).Scan(&userCount)
		log.Printf("Debug: Found %d users with this email", userCount)

		// Debug: Check total matches
		var totalMatches int
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM matches`).Scan(&totalMatches)
		log.Printf("Debug: Total matches in database: %d", totalMatches)

		writeJSON(w, http.StatusOK, Response{
			Success: false,
			Error:   fmt.Sprintf("No matches found for this user. Users with email: %d, Total matches: %d", userCount, totalMatches),
		})
		return
	}

	// Use provided user_name or fallback to user_id from database
	if reqBody.UserName != "" {
		userName = reqBody.UserName
	}

	log.Printf("Found %d matches for %s", len(matchedProducts), reqBody.UserEmail)

	// Prepare payload for n8n
	payload := map[string]interface{}{
		"user_email":       reqBody.UserEmail,
		"user_name":        userName,
		"match_id":         fmt.Sprintf("match-%d", time.Now().Unix()),
		"matched_products": matchedProducts,
	}

	payloadJSON, _ := json.Marshal(payload)

	// Trigger n8n notification workflow
	n8nURL := getEnvOrDefault("N8N_WEBHOOK_URL", "http://localhost:5678")
	webhookURL := fmt.Sprintf("%s/webhook/notify-user", n8nURL)

	log.Printf("Calling n8n webhook: %s", webhookURL)

	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(payloadJSON)))
	if err != nil {
		writeJSON(w, http.StatusOK, Response{
			Success: false,
			Message: "Notification trigger failed (n8n may be offline)",
			Data: map[string]interface{}{
				"n8n_url": webhookURL,
				"error":   err.Error(),
			},
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	writeJSON(w, http.StatusOK, Response{
		Success: resp.StatusCode == 200,
		Message: "Notification workflow triggered",
		Data: map[string]interface{}{
			"n8n_status":    resp.StatusCode,
			"response":      string(respBody),
			"matched_count": len(matchedProducts),
		},
	})
}

func (s *Server) clearDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Clearing all data (users and matches)")

	// Clear matches table
	if _, err := s.db.ExecContext(r.Context(), "DELETE FROM matches"); err != nil {
		log.Printf("Error clearing matches: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to clear matches: " + err.Error(),
		})
		return
	}

	// Clear users table
	if _, err := s.db.ExecContext(r.Context(), "DELETE FROM users"); err != nil {
		log.Printf("Error clearing users: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to clear users: " + err.Error(),
		})
		return
	}

	log.Printf("All data cleared successfully")

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "All data cleared successfully",
	})
}

func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	// Serve frontend files - use absolute path or relative to executable
	frontendDir := "./frontend"

	// Try to find frontend directory
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		// If not found, try parent directory (when running from bin/)
		frontendDir = "../frontend"
	}

	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	filePath := filepath.Join(frontendDir, path)

	// Security check: prevent directory traversal
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	absFrontendDir, _ := filepath.Abs(frontendDir)
	if !strings.HasPrefix(absPath, absFrontendDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Serve index.html for SPA routing or return 404
		indexPath := filepath.Join(frontendDir, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			http.Error(w, "Frontend not found", http.StatusNotFound)
			return
		}
		filePath = indexPath
	}

	http.ServeFile(w, r, filePath)
}

func (s *Server) usersWithMatchesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Check if database is available
	if s.db == nil || s.matchRepo == nil {
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    []map[string]interface{}{},
		})
		return
	}

	// Query to get users who have matches with their match details
	query := `
		SELECT DISTINCT 
			u.id,
			u.user_id,
			u.email,
			COUNT(m.id) as match_count
		FROM users u
		INNER JOIN matches m ON u.id = m.user_id
		WHERE u.is_active = true
		GROUP BY u.id, u.user_id, u.email
		ORDER BY u.user_id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Failed to get users with matches: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to fetch users",
		})
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id int64
		var userID, email string
		var matchCount int

		if err := rows.Scan(&id, &userID, &email, &matchCount); err != nil {
			log.Printf("Failed to scan user row: %v", err)
			continue
		}

		users = append(users, map[string]interface{}{
			"id":          id,
			"user_id":     userID,
			"email":       email,
			"match_count": matchCount,
		})
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    users,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
