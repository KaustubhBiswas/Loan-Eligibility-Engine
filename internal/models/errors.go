// Package models defines the data structures for the loan eligibility engine.
package models

import (
	"errors"
	"strings"
)

// Common errors
var (
	ErrInvalidEmploymentStatus = errors.New("invalid employment status")
	ErrInvalidCreditScore      = errors.New("credit score must be between 300 and 900")
	ErrInvalidAge              = errors.New("age must be between 18 and 120")
	ErrInvalidIncome           = errors.New("monthly income cannot be negative")
	ErrInvalidEmail            = errors.New("invalid email address")
	ErrEmptyUserID             = errors.New("user_id cannot be empty")
)

// NormalizeEmploymentStatus converts various employment status formats to standard values.
func NormalizeEmploymentStatus(status string) EmploymentStatus {
	// Convert to lowercase and normalize separators
	normalized := strings.ToLower(strings.TrimSpace(status))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	// Map common variations
	statusMap := map[string]EmploymentStatus{
		"employed":        EmploymentStatusEmployed,
		"salaried":        EmploymentStatusEmployed,
		"full_time":       EmploymentStatusEmployed,
		"fulltime":        EmploymentStatusEmployed,
		"part_time":       EmploymentStatusEmployed,
		"parttime":        EmploymentStatusEmployed,
		"self_employed":   EmploymentStatusSelfEmployed,
		"selfemployed":    EmploymentStatusSelfEmployed,
		"self_employment": EmploymentStatusSelfEmployed,
		"business":        EmploymentStatusSelfEmployed,
		"business_owner":  EmploymentStatusSelfEmployed,
		"businessowner":   EmploymentStatusSelfEmployed,
		"entrepreneur":    EmploymentStatusSelfEmployed,
		"freelancer":      EmploymentStatusSelfEmployed,
		"unemployed":      EmploymentStatusUnemployed,
		"jobless":         EmploymentStatusUnemployed,
		"not_employed":    EmploymentStatusUnemployed,
		"retired":         EmploymentStatusRetired,
		"pensioner":       EmploymentStatusRetired,
		"student":         EmploymentStatusStudent,
		"studying":        EmploymentStatusStudent,
	}

	if mapped, ok := statusMap[normalized]; ok {
		return mapped
	}

	// Return as-is if no mapping found (will fail validation)
	return EmploymentStatus(normalized)
}

// ValidateUserCreate validates user creation data.
func ValidateUserCreate(u *UserCreate) error {
	if strings.TrimSpace(u.UserID) == "" {
		return ErrEmptyUserID
	}

	if !isValidEmail(u.Email) {
		return ErrInvalidEmail
	}

	if u.MonthlyIncome < 0 {
		return ErrInvalidIncome
	}

	if u.CreditScore < 300 || u.CreditScore > 900 {
		return ErrInvalidCreditScore
	}

	if u.Age < 18 || u.Age > 120 {
		return ErrInvalidAge
	}

	if !u.EmploymentStatus.IsValid() {
		return ErrInvalidEmploymentStatus
	}

	return nil
}

// isValidEmail performs basic email validation.
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}

	// Basic check: must contain @ and have content before and after
	atIndex := strings.Index(email, "@")
	if atIndex <= 0 || atIndex == len(email)-1 {
		return false
	}

	// Must have a dot after @
	dotIndex := strings.LastIndex(email, ".")
	if dotIndex <= atIndex+1 || dotIndex == len(email)-1 {
		return false
	}

	return true
}
