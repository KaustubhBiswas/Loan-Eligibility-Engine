// Package database provides database operations for the loan eligibility engine.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"loan-eligibility-engine/internal/models"
)

// ProductRepository handles loan product database operations.
type ProductRepository struct {
	db *DB
}

// NewProductRepository creates a new product repository.
func NewProductRepository(db *DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// Create inserts a new loan product into the database.
func (r *ProductRepository) Create(ctx context.Context, product *models.LoanProductCreate) (int64, error) {
	empStatusJSON, err := json.Marshal(product.AcceptedEmploymentStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal employment status: %w", err)
	}

	query := `
		INSERT INTO loan_products (
			product_name, provider_name, product_type, interest_rate_min, interest_rate_max,
			loan_amount_min, loan_amount_max, tenure_min_months, tenure_max_months,
			min_monthly_income, min_credit_score, max_credit_score, min_age, max_age,
			accepted_employment_status, processing_fee_percent, source_url,
			created_at, updated_at, is_active, last_crawled_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $18, true, $18)
		RETURNING id`

	var id int64
	now := time.Now().UTC()

	err = r.db.QueryRowContext(ctx, query,
		product.ProductName,
		product.ProviderName,
		string(product.ProductType),
		product.InterestRateMin,
		product.InterestRateMax,
		product.LoanAmountMin,
		product.LoanAmountMax,
		product.TenureMinMonths,
		product.TenureMaxMonths,
		product.MinMonthlyIncome,
		product.MinCreditScore,
		product.MaxCreditScore,
		product.MinAge,
		product.MaxAge,
		string(empStatusJSON),
		product.ProcessingFeePercent,
		product.SourceURL,
		now,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create loan product: %w", err)
	}

	return id, nil
}

// GetByID retrieves a loan product by its ID.
func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*models.LoanProduct, error) {
	query := `
		SELECT id, product_name, provider_name, product_type, interest_rate_min, interest_rate_max,
			loan_amount_min, loan_amount_max, tenure_min_months, tenure_max_months,
			min_monthly_income, min_credit_score, max_credit_score, min_age, max_age,
			accepted_employment_status, processing_fee_percent, source_url,
			created_at, updated_at, is_active, last_crawled_at
		FROM loan_products
		WHERE id = $1`

	product, err := r.scanProduct(r.db.QueryRowContext(ctx, query, id))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get loan product: %w", err)
	}

	return product, nil
}

// GetAllActive retrieves all active loan products.
func (r *ProductRepository) GetAllActive(ctx context.Context) ([]*models.LoanProduct, error) {
	query := `
		SELECT id, product_name, provider_name, product_type, interest_rate_min, interest_rate_max,
			loan_amount_min, loan_amount_max, tenure_min_months, tenure_max_months,
			min_monthly_income, min_credit_score, max_credit_score, min_age, max_age,
			accepted_employment_status, processing_fee_percent, source_url,
			created_at, updated_at, is_active, last_crawled_at
		FROM loan_products
		WHERE is_active = true
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query loan products: %w", err)
	}
	defer rows.Close()

	var products []*models.LoanProduct
	for rows.Next() {
		product, err := r.scanProductRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan loan product: %w", err)
		}
		products = append(products, product)
	}

	return products, nil
}

// UpdateLastCrawledAt updates the last crawled timestamp for a product.
func (r *ProductRepository) UpdateLastCrawledAt(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE loan_products SET last_crawled_at = $1, updated_at = $1 WHERE id = $2",
		time.Now().UTC(), id)
	return err
}

// Deactivate marks a loan product as inactive.
func (r *ProductRepository) Deactivate(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE loan_products SET is_active = false, updated_at = $1 WHERE id = $2",
		time.Now().UTC(), id)
	return err
}

// scanProduct scans a single row into a LoanProduct.
func (r *ProductRepository) scanProduct(row pgx.Row) (*models.LoanProduct, error) {
	var product models.LoanProduct
	var productType string
	var empStatus []string

	err := row.Scan(
		&product.ID,
		&product.ProductName,
		&product.ProviderName,
		&productType,
		&product.InterestRateMin,
		&product.InterestRateMax,
		&product.LoanAmountMin,
		&product.LoanAmountMax,
		&product.TenureMinMonths,
		&product.TenureMaxMonths,
		&product.MinMonthlyIncome,
		&product.MinCreditScore,
		&product.MaxCreditScore,
		&product.MinAge,
		&product.MaxAge,
		&empStatus,
		&product.ProcessingFeePercent,
		&product.SourceURL,
		&product.CreatedAt,
		&product.UpdatedAt,
		&product.IsActive,
		&product.LastCrawledAt,
	)

	if err != nil {
		return nil, err
	}

	product.ProductType = models.LoanProductType(productType)

	// Convert string slice to EmploymentStatus slice
	for _, s := range empStatus {
		product.AcceptedEmploymentStatus = append(product.AcceptedEmploymentStatus, models.EmploymentStatus(s))
	}

	return &product, nil
}

// scanProductRow scans a row from pgx.Rows into a LoanProduct.
func (r *ProductRepository) scanProductRow(rows pgx.Rows) (*models.LoanProduct, error) {
	var product models.LoanProduct
	var productType string
	var empStatus []string

	err := rows.Scan(
		&product.ID,
		&product.ProductName,
		&product.ProviderName,
		&productType,
		&product.InterestRateMin,
		&product.InterestRateMax,
		&product.LoanAmountMin,
		&product.LoanAmountMax,
		&product.TenureMinMonths,
		&product.TenureMaxMonths,
		&product.MinMonthlyIncome,
		&product.MinCreditScore,
		&product.MaxCreditScore,
		&product.MinAge,
		&product.MaxAge,
		&empStatus,
		&product.ProcessingFeePercent,
		&product.SourceURL,
		&product.CreatedAt,
		&product.UpdatedAt,
		&product.IsActive,
		&product.LastCrawledAt,
	)

	if err != nil {
		return nil, err
	}

	product.ProductType = models.LoanProductType(productType)

	// Convert string slice to EmploymentStatus slice
	for _, s := range empStatus {
		product.AcceptedEmploymentStatus = append(product.AcceptedEmploymentStatus, models.EmploymentStatus(s))
	}

	return &product, nil
}
