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

// MatchRepository handles match database operations.
type MatchRepository struct {
	db *DB
}

// NewMatchRepository creates a new match repository.
func NewMatchRepository(db *DB) *MatchRepository {
	return &MatchRepository{db: db}
}

// Create inserts a new match into the database.
func (r *MatchRepository) Create(ctx context.Context, match *models.MatchCreate) (int64, error) {
	query := `
		INSERT INTO matches (
			user_id, product_id, match_score, status, match_source,
			income_eligible, credit_score_eligible, age_eligible, employment_eligible,
			llm_analysis, llm_confidence, batch_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)
		ON CONFLICT (user_id, product_id) DO UPDATE SET
			match_score = EXCLUDED.match_score,
			status = EXCLUDED.status,
			match_source = EXCLUDED.match_source,
			income_eligible = EXCLUDED.income_eligible,
			credit_score_eligible = EXCLUDED.credit_score_eligible,
			age_eligible = EXCLUDED.age_eligible,
			employment_eligible = EXCLUDED.employment_eligible,
			llm_analysis = EXCLUDED.llm_analysis,
			llm_confidence = EXCLUDED.llm_confidence,
			batch_id = EXCLUDED.batch_id,
			updated_at = EXCLUDED.updated_at
		RETURNING id`

	var id int64
	now := time.Now().UTC()

	err := r.db.QueryRowContext(ctx, query,
		match.UserID,
		match.ProductID,
		match.MatchScore,
		string(match.Status),
		string(match.MatchSource),
		match.IncomeEligible,
		match.CreditScoreEligible,
		match.AgeEligible,
		match.EmploymentEligible,
		match.LLMAnalysis,
		match.LLMConfidence,
		match.BatchID,
		now,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create match: %w", err)
	}

	return id, nil
}

// BulkInsert inserts multiple matches into the database.
func (r *MatchRepository) BulkInsert(ctx context.Context, matches []*models.MatchCreate) (int, int, error) {
	inserted := 0
	failed := 0

	err := r.db.WithTransaction(ctx, func(tx pgx.Tx) error {
		now := time.Now().UTC()

		for _, match := range matches {
			_, err := tx.Exec(ctx, `
				INSERT INTO matches (
					user_id, product_id, match_score, status, match_source,
					income_eligible, credit_score_eligible, age_eligible, employment_eligible,
					llm_analysis, llm_confidence, batch_id, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)
				ON CONFLICT (user_id, product_id) DO UPDATE SET
					match_score = EXCLUDED.match_score,
					status = EXCLUDED.status,
					updated_at = EXCLUDED.updated_at`,
				match.UserID,
				match.ProductID,
				match.MatchScore,
				string(match.Status),
				string(match.MatchSource),
				match.IncomeEligible,
				match.CreditScoreEligible,
				match.AgeEligible,
				match.EmploymentEligible,
				match.LLMAnalysis,
				match.LLMConfidence,
				match.BatchID,
				now,
			)

			if err != nil {
				failed++
			} else {
				inserted++
			}
		}
		return nil
	})

	return inserted, failed, err
}

// GetPendingNotifications retrieves matches that need notification.
func (r *MatchRepository) GetPendingNotifications(ctx context.Context, batchID string) ([]*models.MatchWithDetails, error) {
	query := `
		SELECT 
			m.id, m.user_id, m.product_id, m.match_score, m.status, m.match_source,
			m.income_eligible, m.credit_score_eligible, m.age_eligible, m.employment_eligible,
			m.llm_analysis, m.llm_confidence, m.batch_id, m.created_at, m.updated_at, m.notified_at,
			u.email as user_email, u.user_id as user_name,
			p.product_name, p.provider_name, p.interest_rate_min, p.interest_rate_max,
			p.loan_amount_min, p.loan_amount_max
		FROM matches m
		JOIN users u ON m.user_id = u.id
		JOIN loan_products p ON m.product_id = p.id
		WHERE m.status = 'eligible' AND m.notified_at IS NULL`

	args := []interface{}{}
	if batchID != "" {
		query += " AND m.batch_id = $1"
		args = append(args, batchID)
	}

	query += " ORDER BY m.user_id, m.match_score DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending notifications: %w", err)
	}
	defer rows.Close()

	var results []*models.MatchWithDetails
	for rows.Next() {
		var m models.MatchWithDetails
		var status, source string

		err := rows.Scan(
			&m.ID, &m.UserID, &m.ProductID, &m.MatchScore, &status, &source,
			&m.IncomeEligible, &m.CreditScoreEligible, &m.AgeEligible, &m.EmploymentEligible,
			&m.LLMAnalysis, &m.LLMConfidence, &m.BatchID, &m.CreatedAt, &m.UpdatedAt, &m.NotifiedAt,
			&m.UserEmail, &m.UserName,
			&m.ProductName, &m.ProviderName, &m.InterestRateMin, &m.InterestRateMax,
			&m.LoanAmountMin, &m.LoanAmountMax,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan match: %w", err)
		}

		m.Status = models.MatchStatus(status)
		m.MatchSource = models.MatchSource(source)
		results = append(results, &m)
	}

	return results, nil
}

// MarkAsNotified marks a match as notified.
func (r *MatchRepository) MarkAsNotified(ctx context.Context, matchID int64) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		"UPDATE matches SET status = 'notified', notified_at = $1, updated_at = $1 WHERE id = $2",
		now, matchID)
	return err
}

// SQLPrefilterMatches performs fast SQL-based pre-filtering for matching.
// This is Stage 1 of the optimization pipeline.
func (r *MatchRepository) SQLPrefilterMatches(ctx context.Context, batchID string) ([]*models.MatchCandidate, error) {
	query := `
		SELECT 
			u.id as user_db_id,
			u.user_id as user_external_id,
			u.email,
			u.monthly_income,
			u.credit_score,
			u.employment_status,
			u.age,
			p.id as product_id,
			p.product_name,
			p.provider_name,
			p.min_monthly_income,
			p.min_credit_score,
			p.max_credit_score,
			p.min_age,
			p.max_age,
			p.accepted_employment_status,
			p.interest_rate_min,
			p.interest_rate_max
		FROM users u
		CROSS JOIN loan_products p
		WHERE u.is_active = true
		  AND p.is_active = true
		  AND u.monthly_income >= p.min_monthly_income
		  AND u.credit_score >= p.min_credit_score
		  AND (p.max_credit_score IS NULL OR u.credit_score <= p.max_credit_score)
		  AND u.age >= p.min_age
		  AND u.age <= p.max_age`

	args := []interface{}{}
	if batchID != "" {
		query += " AND u.batch_id = $1"
		args = append(args, batchID)
	}

	query += " ORDER BY u.id, p.id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute SQL prefilter: %w", err)
	}
	defer rows.Close()

	var candidates []*models.MatchCandidate
	for rows.Next() {
		var c models.MatchCandidate
		var empStatus, empStatusJSON string

		err := rows.Scan(
			&c.UserDBID,
			&c.UserExternalID,
			&c.Email,
			&c.MonthlyIncome,
			&c.CreditScore,
			&empStatus,
			&c.Age,
			&c.ProductID,
			&c.ProductName,
			&c.ProviderName,
			&c.MinMonthlyIncome,
			&c.MinCreditScore,
			&c.MaxCreditScore,
			&c.MinAge,
			&c.MaxAge,
			&empStatusJSON,
			&c.InterestRateMin,
			&c.InterestRateMax,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan candidate: %w", err)
		}

		c.EmploymentStatus = models.EmploymentStatus(empStatus)

		if empStatusJSON != "" {
			if err := json.Unmarshal([]byte(empStatusJSON), &c.AcceptedEmploymentStatus); err != nil {
				// If parsing fails, assume all statuses accepted
				c.AcceptedEmploymentStatus = []models.EmploymentStatus{}
			}
		}

		candidates = append(candidates, &c)
	}

	return candidates, nil
}

// GetBatchSummary returns summary statistics for a batch.
func (r *MatchRepository) GetBatchSummary(ctx context.Context, batchID string) (*models.BatchMatchSummary, error) {
	summary := &models.BatchMatchSummary{
		BatchID: batchID,
	}

	// Get total users in batch
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE batch_id = $1",
		batchID).Scan(&summary.TotalUsers)
	if err != nil {
		return nil, err
	}

	// Get total products
	err = r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM loan_products WHERE is_active = true").Scan(&summary.TotalProducts)
	if err != nil {
		return nil, err
	}

	// Get match statistics
	err = r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total_matches,
			COUNT(DISTINCT user_id) as users_with_matches,
			COUNT(CASE WHEN match_source = 'sql_filter' THEN 1 END) as sql_matches,
			COUNT(CASE WHEN match_source = 'logic_filter' THEN 1 END) as logic_matches,
			COUNT(CASE WHEN match_source = 'llm_check' THEN 1 END) as llm_matches
		FROM matches
		WHERE batch_id = $1 AND status = 'eligible'`,
		batchID).Scan(
		&summary.TotalMatches,
		&summary.UsersWithMatches,
		&summary.SQLFilterMatches,
		&summary.LogicFilterMatches,
		&summary.LLMCheckMatches,
	)
	if err != nil {
		return nil, err
	}

	if summary.UsersWithMatches > 0 {
		summary.AvgMatchesPerUser = float64(summary.TotalMatches) / float64(summary.UsersWithMatches)
	}

	return summary, nil
}

// GetByUserID retrieves all matches for a specific user.
func (r *MatchRepository) GetByUserID(ctx context.Context, userID int64) ([]models.Match, error) {
	query := `
		SELECT id, user_id, product_id, match_score, status, match_source,
			   income_eligible, credit_score_eligible, age_eligible, employment_eligible,
			   llm_analysis, llm_confidence, batch_id, created_at, updated_at, notified_at
		FROM matches
		WHERE user_id = $1
		ORDER BY match_score DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get matches by user: %w", err)
	}
	defer rows.Close()

	return scanMatches(rows)
}

// GetByBatchID retrieves matches for a specific batch with a limit.
func (r *MatchRepository) GetByBatchID(ctx context.Context, batchID string, limit int) ([]models.Match, error) {
	query := `
		SELECT id, user_id, product_id, match_score, status, match_source,
			   income_eligible, credit_score_eligible, age_eligible, employment_eligible,
			   llm_analysis, llm_confidence, batch_id, created_at, updated_at, notified_at
		FROM matches
		WHERE batch_id = $1
		ORDER BY match_score DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, batchID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get matches by batch: %w", err)
	}
	defer rows.Close()

	return scanMatches(rows)
}

// GetPending retrieves pending matches that haven't been notified.
func (r *MatchRepository) GetPending(ctx context.Context, limit int) ([]models.Match, error) {
	query := `
		SELECT id, user_id, product_id, match_score, status, match_source,
			   income_eligible, credit_score_eligible, age_eligible, employment_eligible,
			   llm_analysis, llm_confidence, batch_id, created_at, updated_at, notified_at
		FROM matches
		WHERE status = 'pending' OR notified_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending matches: %w", err)
	}
	defer rows.Close()

	return scanMatches(rows)
}

// scanMatches is a helper function to scan match rows into Match slice.
func scanMatches(rows interface {
	Next() bool
	Scan(...interface{}) error
}) ([]models.Match, error) {
	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var status, source string
		var llmAnalysis, batchID *string
		err := rows.Scan(
			&m.ID, &m.UserID, &m.ProductID, &m.MatchScore, &status, &source,
			&m.IncomeEligible, &m.CreditScoreEligible, &m.AgeEligible, &m.EmploymentEligible,
			&llmAnalysis, &m.LLMConfidence, &batchID, &m.CreatedAt, &m.UpdatedAt, &m.NotifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan match: %w", err)
		}
		m.Status = models.MatchStatus(status)
		m.MatchSource = models.MatchSource(source)
		if llmAnalysis != nil {
			m.LLMAnalysis = *llmAnalysis
		}
		if batchID != nil {
			m.BatchID = *batchID
		}
		matches = append(matches, m)
	}
	return matches, nil
}
