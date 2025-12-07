// Package database provides database operations for the loan eligibility engine.
package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"loan-eligibility-engine/internal/models"
)

// UserRepository handles user database operations.
type UserRepository struct {
	db *DB
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user into the database.
func (r *UserRepository) Create(ctx context.Context, user *models.UserCreate) (int64, error) {
	query := `
		INSERT INTO users (user_id, email, monthly_income, credit_score, employment_status, age, batch_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			email = EXCLUDED.email,
			monthly_income = EXCLUDED.monthly_income,
			credit_score = EXCLUDED.credit_score,
			employment_status = EXCLUDED.employment_status,
			age = EXCLUDED.age,
			batch_id = EXCLUDED.batch_id,
			updated_at = EXCLUDED.updated_at
		RETURNING id`

	var id int64
	err := r.db.QueryRowContext(ctx, query,
		user.UserID,
		user.Email,
		user.MonthlyIncome,
		user.CreditScore,
		string(user.EmploymentStatus),
		user.Age,
		user.BatchID,
		time.Now().UTC(),
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return id, nil
}

// BulkInsert inserts multiple users into the database.
func (r *UserRepository) BulkInsert(ctx context.Context, users []*models.UserCreate) (*models.BulkInsertResult, error) {
	result := &models.BulkInsertResult{
		InsertedCount: 0,
		FailedCount:   0,
		Errors:        []string{},
	}

	// Use a transaction for bulk insert
	err := r.db.WithTransaction(ctx, func(tx pgx.Tx) error {
		for _, user := range users {
			_, err := tx.Exec(ctx, `
				INSERT INTO users (user_id, email, monthly_income, credit_score, employment_status, age, batch_id, created_at, updated_at, is_active)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, true)
				ON CONFLICT (user_id) DO UPDATE SET
					email = EXCLUDED.email,
					monthly_income = EXCLUDED.monthly_income,
					credit_score = EXCLUDED.credit_score,
					employment_status = EXCLUDED.employment_status,
					age = EXCLUDED.age,
					batch_id = EXCLUDED.batch_id,
					updated_at = EXCLUDED.updated_at`,
				user.UserID,
				user.Email,
				user.MonthlyIncome,
				user.CreditScore,
				string(user.EmploymentStatus),
				user.Age,
				user.BatchID,
				time.Now().UTC(),
			)

			if err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Sprintf("user %s: %v", user.UserID, err))
			} else {
				result.InsertedCount++
			}
		}
		return nil
	})

	if err != nil {
		return result, fmt.Errorf("bulk insert failed: %w", err)
	}

	return result, nil
}

// GetByID retrieves a user by their database ID.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `
		SELECT id, user_id, email, monthly_income, credit_score, employment_status, age, batch_id, created_at, updated_at, is_active
		FROM users
		WHERE id = $1`

	var user models.User
	var empStatus string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.UserID,
		&user.Email,
		&user.MonthlyIncome,
		&user.CreditScore,
		&empStatus,
		&user.Age,
		&user.BatchID,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsActive,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.EmploymentStatus = models.EmploymentStatus(empStatus)
	return &user, nil
}

// GetByIDs retrieves multiple users by their database IDs.
func (r *UserRepository) GetByIDs(ctx context.Context, ids []int64) ([]*models.User, error) {
	if len(ids) == 0 {
		return []*models.User{}, nil
	}

	// Build the query with placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, email, monthly_income, credit_score, employment_status, age, batch_id, created_at, updated_at, is_active
		FROM users
		WHERE id IN (%s) AND is_active = true
		ORDER BY id`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var empStatus string

		err := rows.Scan(
			&user.ID,
			&user.UserID,
			&user.Email,
			&user.MonthlyIncome,
			&user.CreditScore,
			&empStatus,
			&user.Age,
			&user.BatchID,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user.EmploymentStatus = models.EmploymentStatus(empStatus)
		users = append(users, &user)
	}

	return users, nil
}

// GetByUserID retrieves a user by their external user ID.
func (r *UserRepository) GetByUserID(ctx context.Context, userID string) (*models.User, error) {
	query := `
		SELECT id, user_id, email, monthly_income, credit_score, employment_status, age, batch_id, created_at, updated_at, is_active
		FROM users
		WHERE user_id = $1 AND is_active = true`

	var user models.User
	var empStatus string

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.UserID,
		&user.Email,
		&user.MonthlyIncome,
		&user.CreditScore,
		&empStatus,
		&user.Age,
		&user.BatchID,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsActive,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.EmploymentStatus = models.EmploymentStatus(empStatus)
	return &user, nil
}

// GetByBatchID retrieves all users from a specific batch.
func (r *UserRepository) GetByBatchID(ctx context.Context, batchID string) ([]*models.User, error) {
	query := `
		SELECT id, user_id, email, monthly_income, credit_score, employment_status, age, batch_id, created_at, updated_at, is_active
		FROM users
		WHERE batch_id = $1 AND is_active = true
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var empStatus string

		err := rows.Scan(
			&user.ID,
			&user.UserID,
			&user.Email,
			&user.MonthlyIncome,
			&user.CreditScore,
			&empStatus,
			&user.Age,
			&user.BatchID,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user.EmploymentStatus = models.EmploymentStatus(empStatus)
		users = append(users, &user)
	}

	return users, nil
}

// GetAllActive retrieves all active users.
func (r *UserRepository) GetAllActive(ctx context.Context) ([]*models.User, error) {
	query := `
		SELECT id, user_id, email, monthly_income, credit_score, employment_status, age, batch_id, created_at, updated_at, is_active
		FROM users
		WHERE is_active = true
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var empStatus string

		err := rows.Scan(
			&user.ID,
			&user.UserID,
			&user.Email,
			&user.MonthlyIncome,
			&user.CreditScore,
			&empStatus,
			&user.Age,
			&user.BatchID,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user.EmploymentStatus = models.EmploymentStatus(empStatus)
		users = append(users, &user)
	}

	return users, nil
}

// CountByBatchID returns the number of users in a batch.
func (r *UserRepository) CountByBatchID(ctx context.Context, batchID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE batch_id = $1", batchID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}
