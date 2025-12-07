// Package models defines the data structures for the loan eligibility engine.
package models

import (
	"time"
)

// MatchStatus represents the status of a user-product match.
type MatchStatus string

const (
	MatchStatusPending     MatchStatus = "pending"
	MatchStatusEligible    MatchStatus = "eligible"
	MatchStatusNotEligible MatchStatus = "not_eligible"
	MatchStatusNotified    MatchStatus = "notified"
	MatchStatusExpired     MatchStatus = "expired"
)

// MatchSource indicates how the match was determined.
type MatchSource string

const (
	MatchSourceSQLFilter   MatchSource = "sql_filter"
	MatchSourceLogicFilter MatchSource = "logic_filter"
	MatchSourceLLMCheck    MatchSource = "llm_check"
	MatchSourceManual      MatchSource = "manual"
)

// Match represents a user-loan product match.
type Match struct {
	ID                  int64       `json:"id" db:"id"`
	UserID              int64       `json:"user_id" db:"user_id"`
	ProductID           int64       `json:"product_id" db:"product_id"`
	MatchScore          float64     `json:"match_score" db:"match_score"`
	Status              MatchStatus `json:"status" db:"status"`
	MatchSource         MatchSource `json:"match_source" db:"match_source"`
	IncomeEligible      bool        `json:"income_eligible" db:"income_eligible"`
	CreditScoreEligible bool        `json:"credit_score_eligible" db:"credit_score_eligible"`
	AgeEligible         bool        `json:"age_eligible" db:"age_eligible"`
	EmploymentEligible  bool        `json:"employment_eligible" db:"employment_eligible"`
	LLMAnalysis         string      `json:"llm_analysis,omitempty" db:"llm_analysis"`
	LLMConfidence       *float64    `json:"llm_confidence,omitempty" db:"llm_confidence"`
	BatchID             string      `json:"batch_id,omitempty" db:"batch_id"`
	CreatedAt           time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at" db:"updated_at"`
	NotifiedAt          *time.Time  `json:"notified_at,omitempty" db:"notified_at"`
}

// MatchCreate represents data needed to create a new match.
type MatchCreate struct {
	UserID              int64       `json:"user_id" validate:"required"`
	ProductID           int64       `json:"product_id" validate:"required"`
	MatchScore          float64     `json:"match_score" validate:"gte=0,lte=100"`
	Status              MatchStatus `json:"status"`
	MatchSource         MatchSource `json:"match_source"`
	IncomeEligible      bool        `json:"income_eligible"`
	CreditScoreEligible bool        `json:"credit_score_eligible"`
	AgeEligible         bool        `json:"age_eligible"`
	EmploymentEligible  bool        `json:"employment_eligible"`
	LLMAnalysis         string      `json:"llm_analysis,omitempty"`
	LLMConfidence       *float64    `json:"llm_confidence,omitempty"`
	BatchID             string      `json:"batch_id,omitempty"`
}

// MatchWithDetails contains full match information with user and product details.
type MatchWithDetails struct {
	Match
	UserEmail       string  `json:"user_email"`
	UserName        string  `json:"user_name,omitempty"`
	ProductName     string  `json:"product_name"`
	ProviderName    string  `json:"provider_name"`
	InterestRateMin float64 `json:"interest_rate_min"`
	InterestRateMax float64 `json:"interest_rate_max"`
	LoanAmountMin   float64 `json:"loan_amount_min"`
	LoanAmountMax   float64 `json:"loan_amount_max"`
}

// BatchMatchSummary provides summary statistics for a matching batch.
type BatchMatchSummary struct {
	BatchID               string  `json:"batch_id"`
	TotalUsers            int     `json:"total_users"`
	TotalProducts         int     `json:"total_products"`
	TotalMatches          int     `json:"total_matches"`
	UsersWithMatches      int     `json:"users_with_matches"`
	AvgMatchesPerUser     float64 `json:"avg_matches_per_user"`
	ProcessingTimeSeconds float64 `json:"processing_time_seconds"`
	SQLFilterMatches      int     `json:"sql_filter_matches"`
	LogicFilterMatches    int     `json:"logic_filter_matches"`
	LLMCheckMatches       int     `json:"llm_check_matches"`
}

// MatchCandidate represents a potential match from the SQL pre-filter.
type MatchCandidate struct {
	// User fields
	UserDBID         int64            `json:"user_db_id"`
	UserExternalID   string           `json:"user_external_id"`
	Email            string           `json:"email"`
	MonthlyIncome    float64          `json:"monthly_income"`
	CreditScore      int              `json:"credit_score"`
	EmploymentStatus EmploymentStatus `json:"employment_status"`
	Age              int              `json:"age"`

	// Product fields
	ProductID                int64              `json:"product_id"`
	ProductName              string             `json:"product_name"`
	ProviderName             string             `json:"provider_name"`
	MinMonthlyIncome         float64            `json:"min_monthly_income"`
	MinCreditScore           int                `json:"min_credit_score"`
	MaxCreditScore           *int               `json:"max_credit_score"`
	MinAge                   int                `json:"min_age"`
	MaxAge                   int                `json:"max_age"`
	AcceptedEmploymentStatus []EmploymentStatus `json:"accepted_employment_status"`
	InterestRateMin          float64            `json:"interest_rate_min"`
	InterestRateMax          float64            `json:"interest_rate_max"`
}

// IsFullyEligible checks if the candidate passes all basic eligibility checks.
func (c *MatchCandidate) IsFullyEligible() bool {
	// Income check
	if c.MonthlyIncome < c.MinMonthlyIncome {
		return false
	}

	// Credit score check
	if c.CreditScore < c.MinCreditScore {
		return false
	}
	if c.MaxCreditScore != nil && c.CreditScore > *c.MaxCreditScore {
		return false
	}

	// Age check
	if c.Age < c.MinAge || c.Age > c.MaxAge {
		return false
	}

	// Employment status check
	if len(c.AcceptedEmploymentStatus) > 0 {
		found := false
		for _, status := range c.AcceptedEmploymentStatus {
			if status == c.EmploymentStatus {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// CalculateMatchScore calculates a match score based on eligibility factors.
func (c *MatchCandidate) CalculateMatchScore() float64 {
	score := 0.0
	maxScore := 100.0

	// Income score (30 points max)
	if c.MonthlyIncome >= c.MinMonthlyIncome {
		incomeRatio := c.MonthlyIncome / c.MinMonthlyIncome
		if incomeRatio > 2 {
			incomeRatio = 2
		}
		score += 15 * incomeRatio
	}

	// Credit score (40 points max)
	if c.CreditScore >= c.MinCreditScore {
		// Normalize credit score (300-900 range)
		normalizedScore := float64(c.CreditScore-300) / 600.0
		score += 40 * normalizedScore
	}

	// Age score (15 points max)
	if c.Age >= c.MinAge && c.Age <= c.MaxAge {
		score += 15
	}

	// Employment score (15 points max)
	if len(c.AcceptedEmploymentStatus) > 0 {
		for _, status := range c.AcceptedEmploymentStatus {
			if status == c.EmploymentStatus {
				score += 15
				break
			}
		}
	} else {
		score += 15 // No restriction
	}

	// Normalize to 0-100
	if score > maxScore {
		score = maxScore
	}

	return score
}

// NotificationRecord represents a record of an email notification sent.
type NotificationRecord struct {
	ID           int64     `json:"id" db:"id"`
	MatchID      int64     `json:"match_id" db:"match_id"`
	UserDBID     int64     `json:"user_db_id" db:"user_db_id"`
	Email        string    `json:"email" db:"email"`
	SentAt       time.Time `json:"sent_at" db:"sent_at"`
	Status       string    `json:"status" db:"status"`
	MessageID    string    `json:"message_id,omitempty" db:"message_id"`
	ErrorMessage string    `json:"error_message,omitempty" db:"error_message"`
}
