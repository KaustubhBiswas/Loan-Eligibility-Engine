// Package unit_test contains tests for the models package
package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"loan-eligibility-engine/internal/models"
)

func TestEmploymentStatus_IsValid(t *testing.T) {
	tests := []struct {
		status   models.EmploymentStatus
		expected bool
	}{
		{models.EmploymentStatusEmployed, true},
		{models.EmploymentStatusSelfEmployed, true},
		{models.EmploymentStatusUnemployed, true},
		{models.EmploymentStatusRetired, true},
		{models.EmploymentStatusStudent, true},
		{models.EmploymentStatus("invalid"), false},
		{models.EmploymentStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsValid())
		})
	}
}

func TestValidEmploymentStatuses(t *testing.T) {
	statuses := models.ValidEmploymentStatuses()
	assert.Len(t, statuses, 5)
	assert.Contains(t, statuses, models.EmploymentStatusEmployed)
	assert.Contains(t, statuses, models.EmploymentStatusSelfEmployed)
	assert.Contains(t, statuses, models.EmploymentStatusUnemployed)
	assert.Contains(t, statuses, models.EmploymentStatusRetired)
	assert.Contains(t, statuses, models.EmploymentStatusStudent)
}

func TestNormalizeEmploymentStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected models.EmploymentStatus
	}{
		{"employed", models.EmploymentStatusEmployed},
		{"Employed", models.EmploymentStatusEmployed},
		{"EMPLOYED", models.EmploymentStatusEmployed},
		{"full_time", models.EmploymentStatusEmployed},
		{"fulltime", models.EmploymentStatusEmployed},
		{"self_employed", models.EmploymentStatusSelfEmployed},
		{"self-employed", models.EmploymentStatusSelfEmployed},
		{"Self-Employed", models.EmploymentStatusSelfEmployed},
		{"business", models.EmploymentStatusSelfEmployed},
		{"entrepreneur", models.EmploymentStatusSelfEmployed},
		{"freelancer", models.EmploymentStatusSelfEmployed},
		{"unemployed", models.EmploymentStatusUnemployed},
		{"Unemployed", models.EmploymentStatusUnemployed},
		{"retired", models.EmploymentStatusRetired},
		{"Retired", models.EmploymentStatusRetired},
		{"pensioner", models.EmploymentStatusRetired},
		{"student", models.EmploymentStatusStudent},
		{"Student", models.EmploymentStatusStudent},
		{"unknown", models.EmploymentStatus("unknown")}, // Unknown defaults to input lowercase
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := models.NormalizeEmploymentStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateUserCreate_Valid(t *testing.T) {
	user := &models.UserCreate{
		UserID:           "USR001",
		Email:            "test@example.com",
		MonthlyIncome:    50000,
		CreditScore:      750,
		EmploymentStatus: models.EmploymentStatusEmployed,
		Age:              30,
	}

	err := models.ValidateUserCreate(user)
	assert.NoError(t, err)
}

func TestValidateUserCreate_EmptyUserID(t *testing.T) {
	user := &models.UserCreate{
		UserID:           "",
		Email:            "test@example.com",
		MonthlyIncome:    50000,
		CreditScore:      750,
		EmploymentStatus: models.EmploymentStatusEmployed,
		Age:              30,
	}

	err := models.ValidateUserCreate(user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user_id")
}

func TestValidateUserCreate_InvalidEmail(t *testing.T) {
	user := &models.UserCreate{
		UserID:           "USR001",
		Email:            "not-an-email",
		MonthlyIncome:    50000,
		CreditScore:      750,
		EmploymentStatus: models.EmploymentStatusEmployed,
		Age:              30,
	}

	err := models.ValidateUserCreate(user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}

func TestValidateUserCreate_NegativeIncome(t *testing.T) {
	user := &models.UserCreate{
		UserID:           "USR001",
		Email:            "test@example.com",
		MonthlyIncome:    -50000,
		CreditScore:      750,
		EmploymentStatus: models.EmploymentStatusEmployed,
		Age:              30,
	}

	err := models.ValidateUserCreate(user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "income")
}

func TestValidateUserCreate_InvalidCreditScore(t *testing.T) {
	tests := []struct {
		name        string
		creditScore int
		expectError bool
	}{
		{"too low", 299, true},
		{"minimum valid", 300, false},
		{"mid range", 600, false},
		{"maximum valid", 900, false},
		{"too high", 901, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &models.UserCreate{
				UserID:           "USR001",
				Email:            "test@example.com",
				MonthlyIncome:    50000,
				CreditScore:      tt.creditScore,
				EmploymentStatus: models.EmploymentStatusEmployed,
				Age:              30,
			}

			err := models.ValidateUserCreate(user)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUserCreate_InvalidAge(t *testing.T) {
	tests := []struct {
		name        string
		age         int
		expectError bool
	}{
		{"too young", 17, true},
		{"minimum valid", 18, false},
		{"mid age", 45, false},
		{"maximum valid", 120, false},
		{"too old", 121, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &models.UserCreate{
				UserID:           "USR001",
				Email:            "test@example.com",
				MonthlyIncome:    50000,
				CreditScore:      750,
				EmploymentStatus: models.EmploymentStatusEmployed,
				Age:              tt.age,
			}

			err := models.ValidateUserCreate(user)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUser_ToSummary(t *testing.T) {
	user := &models.User{
		ID:               1,
		UserID:           "USR001",
		Email:            "test@example.com",
		MonthlyIncome:    50000,
		CreditScore:      750,
		EmploymentStatus: models.EmploymentStatusEmployed,
		Age:              30,
	}

	summary := user.ToSummary()

	assert.Equal(t, user.ID, summary.ID)
	assert.Equal(t, user.UserID, summary.UserID)
	assert.Equal(t, user.Email, summary.Email)
	assert.Equal(t, user.MonthlyIncome, summary.MonthlyIncome)
	assert.Equal(t, user.CreditScore, summary.CreditScore)
	assert.Equal(t, user.EmploymentStatus, summary.EmploymentStatus)
	assert.Equal(t, user.Age, summary.Age)
}

func TestLoanProduct_ToSummary(t *testing.T) {
	product := &models.LoanProduct{
		ID:               1,
		ProductName:      "Personal Loan",
		ProviderName:     "Test Bank",
		InterestRateMin:  10.5,
		InterestRateMax:  15.0,
		LoanAmountMin:    50000,
		LoanAmountMax:    500000,
		MinMonthlyIncome: 25000,
		MinCreditScore:   700,
	}

	summary := product.ToSummary()

	assert.Equal(t, product.ID, summary.ID)
	assert.Equal(t, product.ProductName, summary.ProductName)
	assert.Equal(t, product.ProviderName, summary.ProviderName)
	assert.Equal(t, product.InterestRateMin, summary.InterestRateMin)
	assert.Equal(t, product.InterestRateMax, summary.InterestRateMax)
	assert.Equal(t, product.LoanAmountMin, summary.LoanAmountMin)
	assert.Equal(t, product.LoanAmountMax, summary.LoanAmountMax)
	assert.Equal(t, product.MinMonthlyIncome, summary.MinMonthlyIncome)
	assert.Equal(t, product.MinCreditScore, summary.MinCreditScore)
}

func TestMatchStatus_Constants(t *testing.T) {
	assert.Equal(t, models.MatchStatus("pending"), models.MatchStatusPending)
	assert.Equal(t, models.MatchStatus("eligible"), models.MatchStatusEligible)
	assert.Equal(t, models.MatchStatus("not_eligible"), models.MatchStatusNotEligible)
	assert.Equal(t, models.MatchStatus("notified"), models.MatchStatusNotified)
	assert.Equal(t, models.MatchStatus("expired"), models.MatchStatusExpired)
}

func TestMatchSource_Constants(t *testing.T) {
	assert.Equal(t, models.MatchSource("sql_filter"), models.MatchSourceSQLFilter)
	assert.Equal(t, models.MatchSource("logic_filter"), models.MatchSourceLogicFilter)
	assert.Equal(t, models.MatchSource("llm_check"), models.MatchSourceLLMCheck)
	assert.Equal(t, models.MatchSource("manual"), models.MatchSourceManual)
}

func TestLoanProductType_Constants(t *testing.T) {
	assert.Equal(t, models.LoanProductType("personal"), models.LoanProductTypePersonal)
	assert.Equal(t, models.LoanProductType("home"), models.LoanProductTypeHome)
	assert.Equal(t, models.LoanProductType("auto"), models.LoanProductTypeAuto)
	assert.Equal(t, models.LoanProductType("education"), models.LoanProductTypeEducation)
	assert.Equal(t, models.LoanProductType("business"), models.LoanProductTypeBusiness)
}

func TestMatchCreate_Defaults(t *testing.T) {
	match := &models.MatchCreate{
		UserID:    1,
		ProductID: 2,
	}

	// Default values should be zero values
	assert.Equal(t, float64(0), match.MatchScore)
	assert.Equal(t, models.MatchStatus(""), match.Status)
	assert.Equal(t, models.MatchSource(""), match.MatchSource)
	assert.False(t, match.IncomeEligible)
	assert.False(t, match.CreditScoreEligible)
	assert.False(t, match.AgeEligible)
	assert.False(t, match.EmploymentEligible)
}
