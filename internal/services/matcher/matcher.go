// Package matcher implements the user-loan matching optimization pipeline
package matcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"loan-eligibility-engine/internal/config"
	"loan-eligibility-engine/internal/models"
	"loan-eligibility-engine/internal/services/database"
	"loan-eligibility-engine/internal/utils"
)

// MatcherService handles the 3-stage matching pipeline
type MatcherService struct {
	db          *database.DB
	userRepo    *database.UserRepository
	productRepo *database.ProductRepository
	matchRepo   *database.MatchRepository
	llmClient   *LLMClient
	config      *config.Config
}

// LLMClient handles calls to Gemini/GPT API
type LLMClient struct {
	apiKey string
	apiURL string
	model  string
	client *http.Client
}

// LLMResponse represents the LLM API response
type LLMResponse struct {
	Qualified   bool     `json:"qualified"`
	Confidence  float64  `json:"confidence"`
	Reasoning   string   `json:"reasoning"`
	RiskFactors []string `json:"risk_factors,omitempty"`
}

// MatchingResult contains the complete result of matching a batch of users
type MatchingResult struct {
	TotalUsers         int
	TotalProducts      int
	TotalPairs         int
	SQLPrefilterPassed int
	LogicFilterPassed  int
	LLMCheckPassed     int
	FinalMatches       int
	ProcessingTime     time.Duration
	Errors             []error
}

// MatchCandidate represents a candidate for matching
type MatchCandidate struct {
	UserID              int64
	ProductID           int64
	EligibilityScore    float64
	IncomeEligible      bool
	CreditScoreEligible bool
	AgeEligible         bool
	EmploymentEligible  bool
	LLMCheckPassed      bool
	LLMReasoning        string
	LLMConfidence       float64
}

// NewMatcherService creates a new matcher service
func NewMatcherService(db *database.DB) (*MatcherService, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	llmClient := &LLMClient{
		apiKey: cfg.GeminiAPIKey,
		apiURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent",
		model:  "gemini-pro",
		client: &http.Client{Timeout: 30 * time.Second},
	}

	return &MatcherService{
		db:          db,
		userRepo:    database.NewUserRepository(db),
		productRepo: database.NewProductRepository(db),
		matchRepo:   database.NewMatchRepository(db),
		llmClient:   llmClient,
		config:      cfg,
	}, nil
}

// ProcessNewUsers runs the matching pipeline for newly uploaded users
func (m *MatcherService) ProcessNewUsers(ctx context.Context, userIDs []int64) (*MatchingResult, error) {
	startTime := time.Now()
	result := &MatchingResult{}

	// Get users
	users, err := m.userRepo.GetByIDs(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	result.TotalUsers = len(users)

	// Get active products
	products, err := m.productRepo.GetAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}
	result.TotalProducts = len(products)
	result.TotalPairs = len(users) * len(products)

	utils.Logger.Info("Starting matching pipeline",
		zap.Int("users", len(users)),
		zap.Int("products", len(products)),
		zap.Int("total_pairs", result.TotalPairs),
	)

	// Stage 1: SQL Prefilter - basic eligibility checks
	candidates := m.sqlPrefilter(users, products)
	result.SQLPrefilterPassed = len(candidates)

	utils.Logger.Info("Stage 1 complete: SQL prefilter",
		zap.Int("passed", len(candidates)),
		zap.Int("filtered_out", result.TotalPairs-len(candidates)),
	)

	// Stage 2: Logic Filter
	candidates = m.logicFilter(candidates, users, products)
	result.LogicFilterPassed = len(candidates)

	utils.Logger.Info("Stage 2 complete: Logic filter",
		zap.Int("passed", len(candidates)),
		zap.Int("filtered_out", result.SQLPrefilterPassed-len(candidates)),
	)

	// Stage 3: LLM Check (for top candidates only)
	// Limit to top 100 candidates per batch to control API costs
	topCandidates := m.selectTopCandidates(candidates, 100)
	finalCandidates, err := m.llmCheck(ctx, topCandidates, users, products)
	if err != nil {
		utils.Logger.Warn("LLM check had errors", zap.Error(err))
		result.Errors = append(result.Errors, err)
	}
	result.LLMCheckPassed = len(finalCandidates)

	utils.Logger.Info("Stage 3 complete: LLM check",
		zap.Int("passed", len(finalCandidates)),
		zap.Int("filtered_out", len(topCandidates)-len(finalCandidates)),
	)

	// Save matches to database
	matches := m.createMatches(finalCandidates)
	if _, _, err := m.matchRepo.BulkInsert(ctx, matches); err != nil {
		return nil, fmt.Errorf("failed to save matches: %w", err)
	}
	result.FinalMatches = len(matches)

	result.ProcessingTime = time.Since(startTime)

	utils.Logger.Info("Matching pipeline complete",
		zap.Int("final_matches", result.FinalMatches),
		zap.Duration("processing_time", result.ProcessingTime),
	)

	return result, nil
}

// sqlPrefilter performs basic eligibility checks
func (m *MatcherService) sqlPrefilter(users []*models.User, products []*models.LoanProduct) []*MatchCandidate {
	candidates := make([]*MatchCandidate, 0)

	for _, user := range users {
		for _, product := range products {
			// Check basic eligibility criteria
			incomeEligible := user.MonthlyIncome >= product.MinMonthlyIncome
			creditScoreEligible := user.CreditScore >= product.MinCreditScore
			ageEligible := user.Age >= product.MinAge && user.Age <= product.MaxAge

			// Check employment status
			employmentEligible := false
			for _, empStatus := range product.AcceptedEmploymentStatus {
				if empStatus == user.EmploymentStatus {
					employmentEligible = true
					break
				}
			}
			// If no employment restrictions, all are eligible
			if len(product.AcceptedEmploymentStatus) == 0 {
				employmentEligible = true
			}

			// Only include if basic criteria pass
			if incomeEligible && creditScoreEligible && ageEligible && employmentEligible {
				candidates = append(candidates, &MatchCandidate{
					UserID:              user.ID,
					ProductID:           product.ID,
					IncomeEligible:      incomeEligible,
					CreditScoreEligible: creditScoreEligible,
					AgeEligible:         ageEligible,
					EmploymentEligible:  employmentEligible,
				})
			}
		}
	}

	return candidates
}

// logicFilter applies business logic rules
func (m *MatcherService) logicFilter(candidates []*MatchCandidate, users []*models.User, products []*models.LoanProduct) []*MatchCandidate {
	userMap := make(map[int64]*models.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	productMap := make(map[int64]*models.LoanProduct)
	for _, p := range products {
		productMap[p.ID] = p
	}

	filtered := make([]*MatchCandidate, 0, len(candidates)/2)

	for _, c := range candidates {
		user := userMap[c.UserID]
		product := productMap[c.ProductID]

		if user == nil || product == nil {
			continue
		}

		// Apply detailed business rules
		if !m.passesBusinessRules(user, product) {
			continue
		}

		// Calculate eligibility score
		c.EligibilityScore = m.calculateEligibilityScore(user, product)

		filtered = append(filtered, c)
	}

	return filtered
}

// passesBusinessRules checks detailed eligibility criteria
func (m *MatcherService) passesBusinessRules(user *models.User, product *models.LoanProduct) bool {
	// Debt-to-income ratio check (assuming max 50% of income can go to loan)
	maxEMI := user.MonthlyIncome * 0.5

	// Rough EMI calculation at max interest rate
	monthlyRate := product.InterestRateMax / 100 / 12
	tenure := float64(product.TenureMaxMonths)
	if tenure == 0 {
		tenure = 60
	}

	// EMI = P * r * (1+r)^n / ((1+r)^n - 1)
	if monthlyRate > 0 {
		emiForMinAmount := product.LoanAmountMin * monthlyRate *
			pow(1+monthlyRate, tenure) / (pow(1+monthlyRate, tenure) - 1)

		if emiForMinAmount > maxEMI*2 { // Some buffer
			return false
		}
	}

	return true
}

// pow calculates power for float64
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// calculateEligibilityScore computes a 0-100 score
func (m *MatcherService) calculateEligibilityScore(user *models.User, product *models.LoanProduct) float64 {
	var score float64 = 0

	// Credit score component (40% weight)
	creditScoreExcess := float64(user.CreditScore - product.MinCreditScore)
	creditScoreRange := float64(900 - product.MinCreditScore)
	if creditScoreRange > 0 {
		creditComponent := (creditScoreExcess / creditScoreRange) * 40
		if creditComponent > 40 {
			creditComponent = 40
		}
		if creditComponent < 0 {
			creditComponent = 0
		}
		score += creditComponent
	}

	// Income component (30% weight)
	incomeExcess := user.MonthlyIncome - product.MinMonthlyIncome
	incomeRange := product.MinMonthlyIncome * 2 // 2x min income is considered excellent
	if incomeRange > 0 {
		incomeComponent := (incomeExcess / incomeRange) * 30
		if incomeComponent > 30 {
			incomeComponent = 30
		}
		if incomeComponent < 0 {
			incomeComponent = 0
		}
		score += incomeComponent
	}

	// Loan amount fit (20% weight) - base score for eligible candidates
	score += 20

	// Age component (10% weight)
	ageRange := float64(product.MaxAge - product.MinAge)
	if ageRange > 0 {
		ageMidpoint := float64(product.MinAge+product.MaxAge) / 2
		ageDiff := abs(float64(user.Age) - ageMidpoint)
		ageComponent := (1 - ageDiff/(ageRange/2)) * 10
		if ageComponent < 0 {
			ageComponent = 0
		}
		score += ageComponent
	}

	return score
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// selectTopCandidates selects the top N candidates by score
func (m *MatcherService) selectTopCandidates(candidates []*MatchCandidate, limit int) []*MatchCandidate {
	if len(candidates) <= limit {
		return candidates
	}

	// Sort by eligibility score (simple bubble sort for small arrays)
	for i := 0; i < len(candidates)-1; i++ {
		for j := 0; j < len(candidates)-i-1; j++ {
			if candidates[j].EligibilityScore < candidates[j+1].EligibilityScore {
				candidates[j], candidates[j+1] = candidates[j+1], candidates[j]
			}
		}
	}

	return candidates[:limit]
}

// llmCheck uses LLM for qualitative assessment
func (m *MatcherService) llmCheck(ctx context.Context, candidates []*MatchCandidate, users []*models.User, products []*models.LoanProduct) ([]*MatchCandidate, error) {
	userMap := make(map[int64]*models.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	productMap := make(map[int64]*models.LoanProduct)
	for _, p := range products {
		productMap[p.ID] = p
	}

	passed := make([]*MatchCandidate, 0)
	var lastErr error

	for _, c := range candidates {
		user := userMap[c.UserID]
		product := productMap[c.ProductID]

		if user == nil || product == nil {
			continue
		}

		response, err := m.llmClient.EvaluateMatch(ctx, user, product)
		if err != nil {
			utils.Logger.Warn("LLM check failed for candidate",
				zap.Int64("user_id", c.UserID),
				zap.Int64("product_id", c.ProductID),
				zap.Error(err),
			)
			lastErr = err
			// On LLM failure, keep candidates that passed logic filter with high scores
			if c.EligibilityScore >= 60 {
				c.LLMCheckPassed = true
				c.LLMReasoning = "LLM check skipped due to API error, high score approved"
				passed = append(passed, c)
			}
			continue
		}

		c.LLMCheckPassed = response.Qualified
		c.LLMReasoning = response.Reasoning
		c.LLMConfidence = response.Confidence

		if response.Qualified {
			passed = append(passed, c)
		}
	}

	return passed, lastErr
}

// createMatches converts candidates to MatchCreate models
func (m *MatcherService) createMatches(candidates []*MatchCandidate) []*models.MatchCreate {
	matches := make([]*models.MatchCreate, len(candidates))

	for i, c := range candidates {
		var llmConfidence *float64
		if c.LLMConfidence > 0 {
			llmConfidence = &c.LLMConfidence
		}

		matches[i] = &models.MatchCreate{
			UserID:              c.UserID,
			ProductID:           c.ProductID,
			MatchScore:          c.EligibilityScore,
			Status:              models.MatchStatusEligible,
			MatchSource:         models.MatchSourceLLMCheck,
			IncomeEligible:      c.IncomeEligible,
			CreditScoreEligible: c.CreditScoreEligible,
			AgeEligible:         c.AgeEligible,
			EmploymentEligible:  c.EmploymentEligible,
			LLMAnalysis:         c.LLMReasoning,
			LLMConfidence:       llmConfidence,
		}
	}

	return matches
}

// EvaluateMatch calls the LLM API to evaluate a user-product match
func (c *LLMClient) EvaluateMatch(ctx context.Context, user *models.User, product *models.LoanProduct) (*LLMResponse, error) {
	if c.apiKey == "" {
		return &LLMResponse{
			Qualified:  true,
			Confidence: 0.7,
			Reasoning:  "LLM check skipped - no API key configured",
		}, nil
	}

	prompt := c.buildPrompt(user, product)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,
			"topK":            1,
			"topP":            1,
			"maxOutputTokens": 500,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", c.apiURL, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.parseResponse(result)
}

// buildPrompt creates the LLM prompt
func (c *LLMClient) buildPrompt(user *models.User, product *models.LoanProduct) string {
	return fmt.Sprintf(`You are a loan eligibility expert. Evaluate if this user is a good candidate for this loan product.

USER PROFILE:
- User ID: %s
- Age: %d years
- Monthly Income: ₹%.0f
- Credit Score: %d
- Employment Status: %s

LOAN PRODUCT:
- Name: %s
- Provider: %s
- Interest Rate: %.2f%% - %.2f%%
- Loan Amount Range: ₹%.0f - ₹%.0f
- Min Credit Score: %d
- Min Monthly Income: ₹%.0f
- Age Range: %d - %d years

Respond ONLY with valid JSON in this exact format:
{
  "qualified": true/false,
  "confidence": 0.0-1.0,
  "reasoning": "Brief explanation",
  "risk_factors": ["factor1", "factor2"]
}

Consider:
1. Does the user meet all hard requirements?
2. Is their income sufficient for loan EMI?
3. Are there any red flags or risk factors?
4. Overall likelihood of loan approval`,
		user.UserID, user.Age, user.MonthlyIncome, user.CreditScore,
		user.EmploymentStatus,
		product.ProductName, product.ProviderName, product.InterestRateMin, product.InterestRateMax,
		product.LoanAmountMin, product.LoanAmountMax, product.MinCreditScore,
		product.MinMonthlyIncome, product.MinAge, product.MaxAge,
	)
}

// parseResponse extracts LLMResponse from API response
func (c *LLMClient) parseResponse(result map[string]interface{}) (*LLMResponse, error) {
	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := candidates[0].(map[string]interface{})
	content := candidate["content"].(map[string]interface{})
	parts := content["parts"].([]interface{})
	if len(parts) == 0 {
		return nil, fmt.Errorf("no parts in response")
	}

	text := parts[0].(map[string]interface{})["text"].(string)

	// Extract JSON from response
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := text[start : end+1]

	var response LLMResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &response, nil
}
