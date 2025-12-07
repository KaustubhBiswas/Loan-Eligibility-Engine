// Package utils provides utility functions for the loan eligibility engine.
package utils

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"loan-eligibility-engine/internal/models"
)

// CSVParser errors
var (
	ErrEmptyCSV       = errors.New("CSV content is empty")
	ErrMissingColumns = errors.New("missing required columns")
	ErrNoDataRows     = errors.New("CSV file contains no data rows")
	ErrInvalidRowData = errors.New("invalid row data")
)

// RequiredColumns defines the columns that must be present in the CSV.
var RequiredColumns = []string{
	"user_id",
	"email",
	"monthly_income",
	"credit_score",
	"employment_status",
	"age",
}

// ColumnAliases maps alternative column names to standard names.
var ColumnAliases = map[string]string{
	// user_id aliases
	"userid":      "user_id",
	"user id":     "user_id",
	"name":        "user_id",
	"username":    "user_id",
	"user_name":   "user_id",
	"full_name":   "user_id",
	"fullname":    "user_id",
	"customer_id": "user_id",
	"customerid":  "user_id",

	// email aliases
	"emailaddress":  "email",
	"email_address": "email",
	"mail":          "email",

	// income aliases
	"income":         "monthly_income",
	"monthlyincome":  "monthly_income",
	"monthly income": "monthly_income",
	"annual_income":  "monthly_income", // Will divide by 12
	"annualincome":   "monthly_income",
	"annual income":  "monthly_income",
	"salary":         "monthly_income",
	"monthly_salary": "monthly_income",
	"monthlysalary":  "monthly_income",

	// credit_score aliases
	"creditscore":  "credit_score",
	"credit score": "credit_score",
	"score":        "credit_score",
	"cibil":        "credit_score",
	"cibil_score":  "credit_score",
	"cibilscore":   "credit_score",

	// employment_status aliases
	"employmentstatus":  "employment_status",
	"employment status": "employment_status",
	"employment":        "employment_status",
	"status":            "employment_status",
	"job_status":        "employment_status",
	"jobstatus":         "employment_status",
	"occupation":        "employment_status",
}

// CSVParser handles parsing of user CSV files.
type CSVParser struct {
	columnMapping   map[string]int
	originalHeaders map[string]string // Maps normalized column name to original header
}

// NewCSVParser creates a new CSV parser instance.
func NewCSVParser() *CSVParser {
	return &CSVParser{
		columnMapping:   make(map[string]int),
		originalHeaders: make(map[string]string),
	}
}

// ParseUsers parses CSV content and returns a slice of UserCreate objects.
func (p *CSVParser) ParseUsers(content string, batchID string) ([]*models.UserCreate, []error) {
	if strings.TrimSpace(content) == "" {
		return nil, []error{ErrEmptyCSV}
	}

	reader := csv.NewReader(strings.NewReader(content))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read header: %w", err)}
	}

	// Build column mapping
	if err := p.buildColumnMapping(header); err != nil {
		return nil, []error{err}
	}

	// Parse data rows
	var users []*models.UserCreate
	var parseErrors []error
	lineNum := 1 // Header is line 1

	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("line %d: %w", lineNum, err))
			continue
		}

		user, err := p.parseRow(record, batchID)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("line %d: %w", lineNum, err))
			continue
		}

		// Validate user
		if err := models.ValidateUserCreate(user); err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("line %d: %w", lineNum, err))
			continue
		}

		users = append(users, user)
	}

	if len(users) == 0 && len(parseErrors) > 0 {
		return nil, append([]error{ErrNoDataRows}, parseErrors...)
	}

	return users, parseErrors
}

// buildColumnMapping creates a mapping of standard column names to their indices.
func (p *CSVParser) buildColumnMapping(header []string) error {
	p.columnMapping = make(map[string]int)
	p.originalHeaders = make(map[string]string)

	for i, col := range header {
		// Normalize column name
		normalized := strings.ToLower(strings.TrimSpace(col))
		original := normalized

		// Apply alias if exists
		if alias, ok := ColumnAliases[normalized]; ok {
			normalized = alias
		}

		p.columnMapping[normalized] = i
		p.originalHeaders[normalized] = original // Store original header name
	}

	// Check for required columns
	var missing []string
	for _, required := range RequiredColumns {
		if _, ok := p.columnMapping[required]; !ok {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingColumns, strings.Join(missing, ", "))
	}

	return nil
}

// parseRow parses a single CSV row into a UserCreate object.
func (p *CSVParser) parseRow(record []string, batchID string) (*models.UserCreate, error) {
	getValue := func(column string) (string, error) {
		idx, ok := p.columnMapping[column]
		if !ok {
			return "", fmt.Errorf("column %s not found", column)
		}
		if idx >= len(record) {
			return "", fmt.Errorf("column %s index out of range", column)
		}
		return strings.TrimSpace(record[idx]), nil
	}

	// Parse user_id
	userID, err := getValue("user_id")
	if err != nil {
		return nil, err
	}

	// Parse email
	email, err := getValue("email")
	if err != nil {
		return nil, err
	}

	// Parse monthly_income
	incomeStr, err := getValue("monthly_income")
	if err != nil {
		return nil, err
	}
	income, err := parseFloat(incomeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid monthly_income: %w", err)
	}

	// Check if the original column was annual income - if so, divide by 12
	if originalHeader, ok := p.originalHeaders["monthly_income"]; ok {
		if strings.Contains(originalHeader, "annual") {
			income = income / 12.0
		}
	}

	// Parse credit_score
	scoreStr, err := getValue("credit_score")
	if err != nil {
		return nil, err
	}
	creditScore, err := parseInt(scoreStr)
	if err != nil {
		return nil, fmt.Errorf("invalid credit_score: %w", err)
	}

	// Parse employment_status
	statusStr, err := getValue("employment_status")
	if err != nil {
		return nil, err
	}
	employmentStatus := models.NormalizeEmploymentStatus(statusStr)

	// Parse age
	ageStr, err := getValue("age")
	if err != nil {
		return nil, err
	}
	age, err := parseInt(ageStr)
	if err != nil {
		return nil, fmt.Errorf("invalid age: %w", err)
	}

	return &models.UserCreate{
		UserID:           userID,
		Email:            email,
		MonthlyIncome:    income,
		CreditScore:      creditScore,
		EmploymentStatus: employmentStatus,
		Age:              age,
		BatchID:          batchID,
	}, nil
}

// parseFloat parses a string to float64, handling common formats.
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, errors.New("empty value")
	}

	// Remove commas and currency symbols
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimPrefix(s, "₹")
	s = strings.TrimSpace(s)

	return strconv.ParseFloat(s, 64)
}

// parseInt parses a string to int, handling common formats.
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty value")
	}

	// Remove commas
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)

	// Handle float strings (e.g., "750.0")
	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, err
		}
		return int(f), nil
	}

	return strconv.Atoi(s)
}

// ValidateCSVStructure performs a quick validation of CSV structure without full parsing.
func ValidateCSVStructure(content string) (*CSVValidationResult, error) {
	result := &CSVValidationResult{
		Valid:          false,
		RowCount:       0,
		Columns:        []string{},
		MissingColumns: []string{},
		Errors:         []string{},
	}

	if strings.TrimSpace(content) == "" {
		result.Errors = append(result.Errors, "empty file")
		return result, nil
	}

	reader := csv.NewReader(strings.NewReader(content))

	// Read header
	header, err := reader.Read()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read header: %v", err))
		return result, nil
	}

	// Normalize and check columns
	normalizedColumns := make(map[string]bool)
	for _, col := range header {
		normalized := strings.ToLower(strings.TrimSpace(col))
		if alias, ok := ColumnAliases[normalized]; ok {
			normalized = alias
		}
		normalizedColumns[normalized] = true
		result.Columns = append(result.Columns, col)
	}

	// Check for required columns
	for _, required := range RequiredColumns {
		if !normalizedColumns[required] {
			result.MissingColumns = append(result.MissingColumns, required)
		}
	}

	// Count rows
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("row error: %v", err))
			continue
		}
		result.RowCount++
	}

	result.Valid = len(result.MissingColumns) == 0 && result.RowCount > 0

	return result, nil
}

// CSVValidationResult contains the results of CSV validation.
type CSVValidationResult struct {
	Valid          bool     `json:"valid"`
	RowCount       int      `json:"row_count"`
	Columns        []string `json:"columns"`
	MissingColumns []string `json:"missing_columns"`
	Errors         []string `json:"errors"`
}
