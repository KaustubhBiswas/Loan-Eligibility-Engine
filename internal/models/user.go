// Package models defines the data structures for the loan eligibility engine.
package models

import (
	"time"
)

// EmploymentStatus represents the employment status of a user.
type EmploymentStatus string

const (
	EmploymentStatusEmployed     EmploymentStatus = "employed"
	EmploymentStatusSelfEmployed EmploymentStatus = "self_employed"
	EmploymentStatusUnemployed   EmploymentStatus = "unemployed"
	EmploymentStatusRetired      EmploymentStatus = "retired"
	EmploymentStatusStudent      EmploymentStatus = "student"
)

// ValidEmploymentStatuses returns all valid employment status values.
func ValidEmploymentStatuses() []EmploymentStatus {
	return []EmploymentStatus{
		EmploymentStatusEmployed,
		EmploymentStatusSelfEmployed,
		EmploymentStatusUnemployed,
		EmploymentStatusRetired,
		EmploymentStatusStudent,
	}
}

// IsValid checks if the employment status is valid.
func (e EmploymentStatus) IsValid() bool {
	for _, valid := range ValidEmploymentStatuses() {
		if e == valid {
			return true
		}
	}
	return false
}

// User represents a user in the system.
type User struct {
	ID               int64            `json:"id" db:"id"`
	UserID           string           `json:"user_id" db:"user_id"`
	Email            string           `json:"email" db:"email"`
	MonthlyIncome    float64          `json:"monthly_income" db:"monthly_income"`
	CreditScore      int              `json:"credit_score" db:"credit_score"`
	EmploymentStatus EmploymentStatus `json:"employment_status" db:"employment_status"`
	Age              int              `json:"age" db:"age"`
	BatchID          string           `json:"batch_id,omitempty" db:"batch_id"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at" db:"updated_at"`
	IsActive         bool             `json:"is_active" db:"is_active"`
}

// UserCreate represents the data needed to create a new user.
type UserCreate struct {
	UserID           string           `json:"user_id" validate:"required,min=1,max=50"`
	Email            string           `json:"email" validate:"required,email"`
	MonthlyIncome    float64          `json:"monthly_income" validate:"required,gte=0"`
	CreditScore      int              `json:"credit_score" validate:"required,gte=300,lte=900"`
	EmploymentStatus EmploymentStatus `json:"employment_status" validate:"required"`
	Age              int              `json:"age" validate:"required,gte=18,lte=120"`
	BatchID          string           `json:"batch_id,omitempty"`
}

// UserSummary is a lightweight view of user for matching operations.
type UserSummary struct {
	ID               int64            `json:"id"`
	UserID           string           `json:"user_id"`
	Email            string           `json:"email"`
	MonthlyIncome    float64          `json:"monthly_income"`
	CreditScore      int              `json:"credit_score"`
	EmploymentStatus EmploymentStatus `json:"employment_status"`
	Age              int              `json:"age"`
}

// ToSummary converts a User to UserSummary.
func (u *User) ToSummary() UserSummary {
	return UserSummary{
		ID:               u.ID,
		UserID:           u.UserID,
		Email:            u.Email,
		MonthlyIncome:    u.MonthlyIncome,
		CreditScore:      u.CreditScore,
		EmploymentStatus: u.EmploymentStatus,
		Age:              u.Age,
	}
}

// CSVUserRow represents a row from the uploaded CSV file.
type CSVUserRow struct {
	UserID           string  `csv:"user_id"`
	Email            string  `csv:"email"`
	MonthlyIncome    float64 `csv:"monthly_income"`
	CreditScore      int     `csv:"credit_score"`
	EmploymentStatus string  `csv:"employment_status"`
	Age              int     `csv:"age"`
}

// ToUserCreate converts a CSV row to UserCreate model.
func (r *CSVUserRow) ToUserCreate(batchID string) (*UserCreate, error) {
	status := NormalizeEmploymentStatus(r.EmploymentStatus)
	if !status.IsValid() {
		return nil, ErrInvalidEmploymentStatus
	}

	return &UserCreate{
		UserID:           r.UserID,
		Email:            r.Email,
		MonthlyIncome:    r.MonthlyIncome,
		CreditScore:      r.CreditScore,
		EmploymentStatus: status,
		Age:              r.Age,
		BatchID:          batchID,
	}, nil
}

// BulkInsertResult contains the results of a bulk insert operation.
type BulkInsertResult struct {
	InsertedCount int      `json:"inserted_count"`
	FailedCount   int      `json:"failed_count"`
	Errors        []string `json:"errors,omitempty"`
}
