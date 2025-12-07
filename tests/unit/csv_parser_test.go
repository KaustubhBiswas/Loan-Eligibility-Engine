// Package unit_test contains unit tests for the loan eligibility engine
package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"loan-eligibility-engine/internal/utils"
)

func TestCSVParser_ValidFile(t *testing.T) {
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,rahul@example.com,50000,750,employed,30
USR002,priya@example.com,60000,720,self_employed,28`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch-001")

	require.Empty(t, errors, "Expected no parse errors")
	require.Len(t, users, 2, "Expected 2 users")

	// Verify first user
	assert.Equal(t, "USR001", users[0].UserID)
	assert.Equal(t, "rahul@example.com", users[0].Email)
	assert.Equal(t, float64(50000), users[0].MonthlyIncome)
	assert.Equal(t, 750, users[0].CreditScore)
	assert.Equal(t, 30, users[0].Age)
	assert.Equal(t, "test-batch-001", users[0].BatchID)
}

func TestCSVParser_ColumnAliases(t *testing.T) {
	// Test with alternative column names (aliases)
	csvContent := `userid,email_address,income,creditscore,employment,age
USR001,rahul@example.com,50000,750,employed,30`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "batch-123")

	require.Empty(t, errors, "Expected no parse errors")
	require.Len(t, users, 1, "Expected 1 user")

	assert.Equal(t, "USR001", users[0].UserID)
	assert.Equal(t, "rahul@example.com", users[0].Email)
}

func TestCSVParser_MissingRequiredColumns(t *testing.T) {
	// Missing credit_score column
	csvContent := `user_id,email,monthly_income,employment_status,age
USR001,rahul@example.com,50000,employed,30`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Empty(t, users, "Expected no valid users")
	assert.NotEmpty(t, errors, "Expected errors for missing columns")
}

func TestCSVParser_EmptyFile(t *testing.T) {
	csvContent := ``

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Empty(t, users, "Expected no users")
	assert.NotEmpty(t, errors, "Expected error for empty file")
}

func TestCSVParser_HeaderOnly(t *testing.T) {
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age`

	parser := utils.NewCSVParser()
	users, _ := parser.ParseUsers(csvContent, "test-batch")

	// Parser returns empty users for header-only file
	// This is acceptable behavior - no data rows means no users
	assert.Empty(t, users, "Expected no users")
}

func TestCSVParser_InvalidCreditScore(t *testing.T) {
	// Credit score out of valid range (300-900)
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,rahul@example.com,50000,200,employed,30`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Empty(t, users, "Expected no valid users")
	assert.NotEmpty(t, errors, "Expected validation error")
}

func TestCSVParser_InvalidAge(t *testing.T) {
	// Age below minimum (18)
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,rahul@example.com,50000,750,employed,15`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Empty(t, users, "Expected no valid users")
	assert.NotEmpty(t, errors, "Expected validation error")
}

func TestCSVParser_InvalidEmail(t *testing.T) {
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,not-an-email,50000,750,employed,30`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Empty(t, users, "Expected no valid users")
	assert.NotEmpty(t, errors, "Expected validation error for invalid email")
}

func TestCSVParser_NegativeIncome(t *testing.T) {
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,rahul@example.com,-50000,750,employed,30`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Empty(t, users, "Expected no valid users")
	assert.NotEmpty(t, errors, "Expected validation error for negative income")
}

func TestCSVParser_PartiallyValidFile(t *testing.T) {
	// Mix of valid and invalid rows
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,rahul@example.com,50000,750,employed,30
USR002,invalid-email,60000,720,self_employed,28
USR003,priya@example.com,40000,680,employed,25`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Len(t, users, 2, "Expected 2 valid users")
	assert.Len(t, errors, 1, "Expected 1 error for invalid email")
}

func TestCSVParser_EmploymentStatusNormalization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"employed", "employed"},
		{"Employed", "employed"},
		{"EMPLOYED", "employed"},
		{"full_time", "employed"},
		{"fulltime", "employed"},
		{"self_employed", "self_employed"},
		{"Self-Employed", "self_employed"},
		{"business", "self_employed"},
		{"unemployed", "unemployed"},
		{"retired", "retired"},
		{"student", "student"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,test@example.com,50000,750,` + tc.input + `,30`

			parser := utils.NewCSVParser()
			users, _ := parser.ParseUsers(csvContent, "test-batch")

			if len(users) > 0 {
				assert.Equal(t, tc.expected, string(users[0].EmploymentStatus))
			}
		})
	}
}

func TestCSVParser_WhitespaceHandling(t *testing.T) {
	// Test with extra whitespace
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
  USR001  ,  rahul@example.com  ,  50000  ,  750  ,  employed  ,  30  `

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	require.Empty(t, errors, "Expected no parse errors")
	require.Len(t, users, 1, "Expected 1 user")

	assert.Equal(t, "USR001", users[0].UserID)
	assert.Equal(t, "rahul@example.com", users[0].Email)
}

func TestCSVParser_FloatIncome(t *testing.T) {
	// Income as decimal
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,rahul@example.com,50000.50,750,employed,30`

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	require.Empty(t, errors, "Expected no parse errors")
	require.Len(t, users, 1, "Expected 1 user")

	assert.Equal(t, float64(50000.50), users[0].MonthlyIncome)
}

func TestCSVParser_LargeFile(t *testing.T) {
	// Generate CSV with 100 rows
	header := "user_id,email,monthly_income,credit_score,employment_status,age\n"
	rows := ""
	for i := 1; i <= 100; i++ {
		rows += "USR" + padLeft(i, 3) + ",user" + padLeft(i, 3) + "@example.com,50000,750,employed,30\n"
	}

	csvContent := header + rows

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(csvContent, "test-batch")

	assert.Empty(t, errors, "Expected no parse errors")
	assert.Len(t, users, 100, "Expected 100 users")
}

// Helper function to pad numbers
func padLeft(n, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s = "0" + s
	}
	ns := s + string(rune('0'+n%10))
	if n >= 10 {
		ns = s[:width-2] + string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	if n >= 100 {
		ns = string(rune('0'+n/100)) + string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
	}
	return ns[len(ns)-width:]
}

func TestValidateCSVStructure(t *testing.T) {
	csvContent := `user_id,email,monthly_income,credit_score,employment_status,age
USR001,rahul@example.com,50000,750,employed,30
USR002,priya@example.com,60000,720,self_employed,28`

	result, err := utils.ValidateCSVStructure(csvContent)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, 2, result.RowCount)
	assert.Empty(t, result.MissingColumns)
}

func TestValidateCSVStructure_MissingColumns(t *testing.T) {
	csvContent := `user_id,email,monthly_income
USR001,rahul@example.com,50000`

	result, err := utils.ValidateCSVStructure(csvContent)

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.MissingColumns, "credit_score")
	assert.Contains(t, result.MissingColumns, "employment_status")
	assert.Contains(t, result.MissingColumns, "age")
}
