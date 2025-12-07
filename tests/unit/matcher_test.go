// Package unit_test contains tests for the matching service
package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"loan-eligibility-engine/internal/models"
)

// mockUser creates a test user with default values
func mockUser(overrides map[string]interface{}) *models.User {
	user := &models.User{
		ID:               1,
		UserID:           "USR001",
		Email:            "test@example.com",
		MonthlyIncome:    50000,
		CreditScore:      750,
		EmploymentStatus: models.EmploymentStatusEmployed,
		Age:              35,
		BatchID:          "batch-001",
		IsActive:         true,
	}

	if v, ok := overrides["id"]; ok {
		user.ID = v.(int64)
	}
	if v, ok := overrides["user_id"]; ok {
		user.UserID = v.(string)
	}
	if v, ok := overrides["age"]; ok {
		user.Age = v.(int)
	}
	if v, ok := overrides["monthly_income"]; ok {
		user.MonthlyIncome = v.(float64)
	}
	if v, ok := overrides["credit_score"]; ok {
		user.CreditScore = v.(int)
	}
	if v, ok := overrides["employment_status"]; ok {
		user.EmploymentStatus = v.(models.EmploymentStatus)
	}

	return user
}

// mockProduct creates a test loan product with default values
func mockProduct(overrides map[string]interface{}) *models.LoanProduct {
	product := &models.LoanProduct{
		ID:                       1,
		ProductName:              "Test Personal Loan",
		ProviderName:             "Test Bank",
		ProductType:              models.LoanProductTypePersonal,
		InterestRateMin:          10.5,
		InterestRateMax:          18.0,
		LoanAmountMin:            50000,
		LoanAmountMax:            2500000,
		TenureMinMonths:          12,
		TenureMaxMonths:          60,
		MinMonthlyIncome:         25000,
		MinCreditScore:           700,
		MinAge:                   21,
		MaxAge:                   60,
		AcceptedEmploymentStatus: []models.EmploymentStatus{models.EmploymentStatusEmployed, models.EmploymentStatusSelfEmployed},
		IsActive:                 true,
	}

	if v, ok := overrides["id"]; ok {
		product.ID = v.(int64)
	}
	if v, ok := overrides["min_credit_score"]; ok {
		product.MinCreditScore = v.(int)
	}
	if v, ok := overrides["min_monthly_income"]; ok {
		product.MinMonthlyIncome = v.(float64)
	}
	if v, ok := overrides["min_age"]; ok {
		product.MinAge = v.(int)
	}
	if v, ok := overrides["max_age"]; ok {
		product.MaxAge = v.(int)
	}
	if v, ok := overrides["accepted_employment_status"]; ok {
		product.AcceptedEmploymentStatus = v.([]models.EmploymentStatus)
	}

	return product
}

// checkBasicEligibility checks if a user meets basic eligibility criteria for a product
func checkBasicEligibility(user *models.User, product *models.LoanProduct) (bool, bool, bool, bool) {
	incomeEligible := user.MonthlyIncome >= product.MinMonthlyIncome
	creditEligible := user.CreditScore >= product.MinCreditScore
	ageEligible := user.Age >= product.MinAge && user.Age <= product.MaxAge

	employmentEligible := false
	if len(product.AcceptedEmploymentStatus) == 0 {
		employmentEligible = true
	} else {
		for _, status := range product.AcceptedEmploymentStatus {
			if user.EmploymentStatus == status {
				employmentEligible = true
				break
			}
		}
	}

	return incomeEligible, creditEligible, ageEligible, employmentEligible
}

func TestBasicEligibility_AllCriteriaMet(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"monthly_income":    float64(60000),
		"credit_score":      780,
		"age":               35,
		"employment_status": models.EmploymentStatusEmployed,
	})

	product := mockProduct(map[string]interface{}{
		"min_monthly_income": float64(25000),
		"min_credit_score":   700,
		"min_age":            21,
		"max_age":            60,
	})

	income, credit, age, employment := checkBasicEligibility(user, product)

	assert.True(t, income, "Income should be eligible")
	assert.True(t, credit, "Credit score should be eligible")
	assert.True(t, age, "Age should be eligible")
	assert.True(t, employment, "Employment should be eligible")
}

func TestBasicEligibility_IncomeTooLow(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"monthly_income": float64(20000),
	})

	product := mockProduct(map[string]interface{}{
		"min_monthly_income": float64(25000),
	})

	income, _, _, _ := checkBasicEligibility(user, product)

	assert.False(t, income, "Income should not be eligible")
}

func TestBasicEligibility_CreditScoreTooLow(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"credit_score": 650,
	})

	product := mockProduct(map[string]interface{}{
		"min_credit_score": 700,
	})

	_, credit, _, _ := checkBasicEligibility(user, product)

	assert.False(t, credit, "Credit score should not be eligible")
}

func TestBasicEligibility_AgeTooYoung(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"age": 20,
	})

	product := mockProduct(map[string]interface{}{
		"min_age": 21,
		"max_age": 60,
	})

	_, _, age, _ := checkBasicEligibility(user, product)

	assert.False(t, age, "Age should not be eligible (too young)")
}

func TestBasicEligibility_AgeTooOld(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"age": 65,
	})

	product := mockProduct(map[string]interface{}{
		"min_age": 21,
		"max_age": 60,
	})

	_, _, age, _ := checkBasicEligibility(user, product)

	assert.False(t, age, "Age should not be eligible (too old)")
}

func TestBasicEligibility_EmploymentNotAccepted(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"employment_status": models.EmploymentStatusUnemployed,
	})

	product := mockProduct(map[string]interface{}{
		"accepted_employment_status": []models.EmploymentStatus{
			models.EmploymentStatusEmployed,
			models.EmploymentStatusSelfEmployed,
		},
	})

	_, _, _, employment := checkBasicEligibility(user, product)

	assert.False(t, employment, "Employment should not be eligible")
}

func TestBasicEligibility_EmptyEmploymentStatusAcceptsAll(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"employment_status": models.EmploymentStatusStudent,
	})

	product := mockProduct(map[string]interface{}{
		"accepted_employment_status": []models.EmploymentStatus{},
	})

	_, _, _, employment := checkBasicEligibility(user, product)

	assert.True(t, employment, "Empty accepted list should accept all employment statuses")
}

// calculateMatchScore calculates a simple match score based on how well a user exceeds requirements
func calculateMatchScore(user *models.User, product *models.LoanProduct) float64 {
	score := 0.0

	// Income factor (25%)
	if user.MonthlyIncome >= product.MinMonthlyIncome {
		incomeRatio := user.MonthlyIncome / product.MinMonthlyIncome
		if incomeRatio > 2 {
			incomeRatio = 2
		}
		score += (incomeRatio - 1) * 25
	}

	// Credit score factor (40%)
	if user.CreditScore >= product.MinCreditScore {
		creditExcess := float64(user.CreditScore - product.MinCreditScore)
		maxExcess := float64(900 - product.MinCreditScore)
		if maxExcess > 0 {
			score += (creditExcess / maxExcess) * 40
		}
	}

	// Age factor (15%) - prefer middle age
	if user.Age >= product.MinAge && user.Age <= product.MaxAge {
		midAge := (product.MinAge + product.MaxAge) / 2
		ageDiff := float64(abs(user.Age - midAge))
		maxDiff := float64((product.MaxAge - product.MinAge) / 2)
		if maxDiff > 0 {
			score += (1 - ageDiff/maxDiff) * 15
		} else {
			score += 15
		}
	}

	// Employment factor (20%)
	if len(product.AcceptedEmploymentStatus) == 0 {
		score += 20
	} else {
		for _, status := range product.AcceptedEmploymentStatus {
			if user.EmploymentStatus == status {
				score += 20
				break
			}
		}
	}

	// Normalize to 0-100
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestMatchScore_HighQualityMatch(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"monthly_income":    float64(100000),
		"credit_score":      850,
		"age":               40,
		"employment_status": models.EmploymentStatusEmployed,
	})

	product := mockProduct(map[string]interface{}{
		"min_monthly_income": float64(25000),
		"min_credit_score":   700,
		"min_age":            21,
		"max_age":            60,
	})

	score := calculateMatchScore(user, product)

	assert.GreaterOrEqual(t, score, 60.0, "High quality match should have score >= 60")
}

func TestMatchScore_MinimumQualifyingMatch(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"monthly_income":    float64(25000), // exactly minimum
		"credit_score":      700,            // exactly minimum
		"age":               21,             // exactly minimum age
		"employment_status": models.EmploymentStatusEmployed,
	})

	product := mockProduct(map[string]interface{}{
		"min_monthly_income": float64(25000),
		"min_credit_score":   700,
		"min_age":            21,
		"max_age":            60,
	})

	score := calculateMatchScore(user, product)

	// Minimum qualifying should have moderate score (20 from employment + some age score)
	assert.GreaterOrEqual(t, score, 20.0, "Minimum qualifying match should have score >= 20")
}

func TestMatchScore_SelfEmployedPremium(t *testing.T) {
	employedUser := mockUser(map[string]interface{}{
		"employment_status": models.EmploymentStatusEmployed,
	})

	selfEmployedUser := mockUser(map[string]interface{}{
		"employment_status": models.EmploymentStatusSelfEmployed,
	})

	product := mockProduct(map[string]interface{}{
		"accepted_employment_status": []models.EmploymentStatus{
			models.EmploymentStatusEmployed,
			models.EmploymentStatusSelfEmployed,
		},
	})

	employedScore := calculateMatchScore(employedUser, product)
	selfEmployedScore := calculateMatchScore(selfEmployedUser, product)

	// Both should qualify with similar scores
	assert.InDelta(t, employedScore, selfEmployedScore, 5.0, "Similar employment types should have similar scores")
}

func TestBatchMatching(t *testing.T) {
	users := []*models.User{
		mockUser(map[string]interface{}{"id": int64(1), "user_id": "USR001", "credit_score": 800, "monthly_income": float64(80000)}),
		mockUser(map[string]interface{}{"id": int64(2), "user_id": "USR002", "credit_score": 650, "monthly_income": float64(30000)}),
		mockUser(map[string]interface{}{"id": int64(3), "user_id": "USR003", "credit_score": 720, "monthly_income": float64(45000)}),
	}

	product := mockProduct(map[string]interface{}{
		"min_credit_score":   700,
		"min_monthly_income": float64(25000),
	})

	eligibleCount := 0
	for _, user := range users {
		income, credit, age, employment := checkBasicEligibility(user, product)
		if income && credit && age && employment {
			eligibleCount++
		}
	}

	// USR001 and USR003 should be eligible
	assert.Equal(t, 2, eligibleCount, "Expected 2 eligible users")
}

func TestMultiProductMatching(t *testing.T) {
	user := mockUser(map[string]interface{}{
		"credit_score":   750,
		"monthly_income": float64(50000),
		"age":            35,
	})

	products := []*models.LoanProduct{
		mockProduct(map[string]interface{}{"id": int64(1), "min_credit_score": 700, "min_monthly_income": float64(25000)}), // eligible
		mockProduct(map[string]interface{}{"id": int64(2), "min_credit_score": 800, "min_monthly_income": float64(25000)}), // not eligible (credit)
		mockProduct(map[string]interface{}{"id": int64(3), "min_credit_score": 700, "min_monthly_income": float64(60000)}), // not eligible (income)
		mockProduct(map[string]interface{}{"id": int64(4), "min_credit_score": 650, "min_monthly_income": float64(30000)}), // eligible
	}

	eligibleCount := 0
	for _, product := range products {
		income, credit, age, employment := checkBasicEligibility(user, product)
		if income && credit && age && employment {
			eligibleCount++
		}
	}

	assert.Equal(t, 2, eligibleCount, "Expected user to be eligible for 2 products")
}

func TestMatchCreate_AllEligibilityFlags(t *testing.T) {
	match := &models.MatchCreate{
		UserID:              1,
		ProductID:           1,
		MatchScore:          85.5,
		Status:              models.MatchStatusEligible,
		MatchSource:         models.MatchSourceSQLFilter,
		IncomeEligible:      true,
		CreditScoreEligible: true,
		AgeEligible:         true,
		EmploymentEligible:  true,
		BatchID:             "batch-001",
	}

	assert.Equal(t, int64(1), match.UserID)
	assert.Equal(t, int64(1), match.ProductID)
	assert.Equal(t, 85.5, match.MatchScore)
	assert.Equal(t, models.MatchStatusEligible, match.Status)
	assert.Equal(t, models.MatchSourceSQLFilter, match.MatchSource)
	assert.True(t, match.IncomeEligible)
	assert.True(t, match.CreditScoreEligible)
	assert.True(t, match.AgeEligible)
	assert.True(t, match.EmploymentEligible)
}

func TestMatchWithDetails_Contains_AllFields(t *testing.T) {
	match := models.MatchWithDetails{
		Match: models.Match{
			ID:                  1,
			UserID:              1,
			ProductID:           1,
			MatchScore:          85.0,
			Status:              models.MatchStatusEligible,
			MatchSource:         models.MatchSourceLogicFilter,
			IncomeEligible:      true,
			CreditScoreEligible: true,
			AgeEligible:         true,
			EmploymentEligible:  true,
		},
		UserEmail:       "test@example.com",
		ProductName:     "Personal Loan",
		ProviderName:    "Test Bank",
		InterestRateMin: 10.5,
		InterestRateMax: 18.0,
		LoanAmountMin:   50000,
		LoanAmountMax:   500000,
	}

	assert.Equal(t, "test@example.com", match.UserEmail)
	assert.Equal(t, "Personal Loan", match.ProductName)
	assert.Equal(t, "Test Bank", match.ProviderName)
	assert.Equal(t, 10.5, match.InterestRateMin)
	assert.Equal(t, 18.0, match.InterestRateMax)
}
