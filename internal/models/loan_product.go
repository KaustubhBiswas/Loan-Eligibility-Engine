// Package models defines the data structures for the loan eligibility engine.
package models

import (
	"time"
)

// LoanProductType represents the type of loan product.
type LoanProductType string

const (
	LoanProductTypePersonal  LoanProductType = "personal"
	LoanProductTypeHome      LoanProductType = "home"
	LoanProductTypeAuto      LoanProductType = "auto"
	LoanProductTypeEducation LoanProductType = "education"
	LoanProductTypeBusiness  LoanProductType = "business"
)

// LoanProduct represents a loan product from a financial institution.
type LoanProduct struct {
	ID                       int64              `json:"id" db:"id"`
	ProductName              string             `json:"product_name" db:"product_name"`
	ProviderName             string             `json:"provider_name" db:"provider_name"`
	ProductType              LoanProductType    `json:"product_type" db:"product_type"`
	InterestRateMin          float64            `json:"interest_rate_min" db:"interest_rate_min"`
	InterestRateMax          float64            `json:"interest_rate_max" db:"interest_rate_max"`
	LoanAmountMin            float64            `json:"loan_amount_min" db:"loan_amount_min"`
	LoanAmountMax            float64            `json:"loan_amount_max" db:"loan_amount_max"`
	TenureMinMonths          int                `json:"tenure_min_months" db:"tenure_min_months"`
	TenureMaxMonths          int                `json:"tenure_max_months" db:"tenure_max_months"`
	MinMonthlyIncome         float64            `json:"min_monthly_income" db:"min_monthly_income"`
	MinCreditScore           int                `json:"min_credit_score" db:"min_credit_score"`
	MaxCreditScore           *int               `json:"max_credit_score,omitempty" db:"max_credit_score"`
	MinAge                   int                `json:"min_age" db:"min_age"`
	MaxAge                   int                `json:"max_age" db:"max_age"`
	AcceptedEmploymentStatus []EmploymentStatus `json:"accepted_employment_status" db:"accepted_employment_status"`
	ProcessingFeePercent     *float64           `json:"processing_fee_percent,omitempty" db:"processing_fee_percent"`
	SourceURL                string             `json:"source_url,omitempty" db:"source_url"`
	CreatedAt                time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time          `json:"updated_at" db:"updated_at"`
	IsActive                 bool               `json:"is_active" db:"is_active"`
	LastCrawledAt            *time.Time         `json:"last_crawled_at,omitempty" db:"last_crawled_at"`
}

// LoanProductCreate represents data needed to create a new loan product.
type LoanProductCreate struct {
	ProductName              string             `json:"product_name" validate:"required,min=1,max=200"`
	ProviderName             string             `json:"provider_name" validate:"required,min=1,max=200"`
	ProductType              LoanProductType    `json:"product_type"`
	InterestRateMin          float64            `json:"interest_rate_min" validate:"required,gte=0,lte=100"`
	InterestRateMax          float64            `json:"interest_rate_max" validate:"required,gte=0,lte=100"`
	LoanAmountMin            float64            `json:"loan_amount_min" validate:"required,gte=0"`
	LoanAmountMax            float64            `json:"loan_amount_max" validate:"required,gte=0"`
	TenureMinMonths          int                `json:"tenure_min_months" validate:"required,gte=1"`
	TenureMaxMonths          int                `json:"tenure_max_months" validate:"required,gte=1"`
	MinMonthlyIncome         float64            `json:"min_monthly_income" validate:"required,gte=0"`
	MinCreditScore           int                `json:"min_credit_score" validate:"required,gte=300,lte=900"`
	MaxCreditScore           *int               `json:"max_credit_score,omitempty"`
	MinAge                   int                `json:"min_age" validate:"gte=18"`
	MaxAge                   int                `json:"max_age" validate:"lte=120"`
	AcceptedEmploymentStatus []EmploymentStatus `json:"accepted_employment_status"`
	ProcessingFeePercent     *float64           `json:"processing_fee_percent,omitempty"`
	SourceURL                string             `json:"source_url,omitempty"`
}

// LoanProductSummary is a lightweight view for display purposes.
type LoanProductSummary struct {
	ID               int64   `json:"id"`
	ProductName      string  `json:"product_name"`
	ProviderName     string  `json:"provider_name"`
	InterestRateMin  float64 `json:"interest_rate_min"`
	InterestRateMax  float64 `json:"interest_rate_max"`
	LoanAmountMin    float64 `json:"loan_amount_min"`
	LoanAmountMax    float64 `json:"loan_amount_max"`
	MinMonthlyIncome float64 `json:"min_monthly_income"`
	MinCreditScore   int     `json:"min_credit_score"`
}

// ToSummary converts a LoanProduct to LoanProductSummary.
func (p *LoanProduct) ToSummary() LoanProductSummary {
	return LoanProductSummary{
		ID:               p.ID,
		ProductName:      p.ProductName,
		ProviderName:     p.ProviderName,
		InterestRateMin:  p.InterestRateMin,
		InterestRateMax:  p.InterestRateMax,
		LoanAmountMin:    p.LoanAmountMin,
		LoanAmountMax:    p.LoanAmountMax,
		MinMonthlyIncome: p.MinMonthlyIncome,
		MinCreditScore:   p.MinCreditScore,
	}
}

// EligibilityCriteria contains the eligibility requirements for a loan product.
type EligibilityCriteria struct {
	ProductID                int64              `json:"product_id"`
	MinMonthlyIncome         float64            `json:"min_monthly_income"`
	MinCreditScore           int                `json:"min_credit_score"`
	MaxCreditScore           *int               `json:"max_credit_score,omitempty"`
	MinAge                   int                `json:"min_age"`
	MaxAge                   int                `json:"max_age"`
	AcceptedEmploymentStatus []EmploymentStatus `json:"accepted_employment_status"`
}

// GetEligibilityCriteria extracts eligibility criteria from a loan product.
func (p *LoanProduct) GetEligibilityCriteria() EligibilityCriteria {
	return EligibilityCriteria{
		ProductID:                p.ID,
		MinMonthlyIncome:         p.MinMonthlyIncome,
		MinCreditScore:           p.MinCreditScore,
		MaxCreditScore:           p.MaxCreditScore,
		MinAge:                   p.MinAge,
		MaxAge:                   p.MaxAge,
		AcceptedEmploymentStatus: p.AcceptedEmploymentStatus,
	}
}

// CrawledProduct represents a loan product extracted from web crawling.
type CrawledProduct struct {
	ProductName         string    `json:"product_name"`
	ProviderName        string    `json:"provider_name"`
	InterestRate        string    `json:"interest_rate"`
	MinIncome           string    `json:"min_income"`
	MinCreditScore      string    `json:"min_credit_score"`
	EligibilityCriteria string    `json:"eligibility_criteria"`
	SourceURL           string    `json:"source_url"`
	CrawledAt           time.Time `json:"crawled_at"`
}
