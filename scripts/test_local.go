package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"loan-eligibility-engine/internal/utils"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("=== Loan Eligibility Engine - Local Test ===")
	fmt.Println()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Printf("⚠️  Warning: Could not load .env file: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to database
	databaseURL := os.Getenv("DATABASE_URL")
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Printf("❌ Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)
	fmt.Println("✅ Connected to database")

	// Parse sample CSV
	fmt.Println()
	fmt.Println("📖 Parsing sample CSV...")

	csvContent, err := os.ReadFile("data/sample_users.csv")
	if err != nil {
		fmt.Printf("❌ Failed to read CSV: %v\n", err)
		os.Exit(1)
	}

	parser := utils.NewCSVParser()
	users, errors := parser.ParseUsers(string(csvContent), "test-batch-001")
	if len(errors) > 0 {
		fmt.Printf("⚠️  CSV parsing errors: %v\n", errors)
	}
	fmt.Printf("✅ Parsed %d users from CSV\n", len(users))

	// Insert users into database
	fmt.Println()
	fmt.Println("📥 Inserting users into database...")

	for _, user := range users {
		_, err := conn.Exec(ctx, `
			INSERT INTO users (user_id, email, monthly_income, credit_score, employment_status, age, batch_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (user_id) DO UPDATE SET
				email = EXCLUDED.email,
				monthly_income = EXCLUDED.monthly_income,
				credit_score = EXCLUDED.credit_score,
				employment_status = EXCLUDED.employment_status,
				age = EXCLUDED.age,
				updated_at = CURRENT_TIMESTAMP
		`, user.UserID, user.Email, user.MonthlyIncome, user.CreditScore, user.EmploymentStatus, user.Age, user.BatchID)
		if err != nil {
			fmt.Printf("   ⚠️  Error inserting user %s: %v\n", user.UserID, err)
		}
	}
	fmt.Println("✅ Users inserted!")

	// Count users
	var userCount int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	fmt.Printf("   📊 Total users in database: %d\n", userCount)

	// Run basic matching query
	fmt.Println()
	fmt.Println("🎯 Running loan matching...")

	rows, err := conn.Query(ctx, `
		SELECT 
			u.user_id, 
			u.email,
			u.credit_score,
			u.monthly_income,
			p.product_name,
			p.provider_name,
			p.min_credit_score,
			p.min_monthly_income
		FROM users u
		CROSS JOIN loan_products p
		WHERE u.credit_score >= p.min_credit_score
		  AND u.monthly_income >= p.min_monthly_income
		  AND u.age >= p.min_age
		  AND u.age <= p.max_age
		  AND p.is_active = true
		ORDER BY u.user_id, p.product_name
		LIMIT 20
	`)
	if err != nil {
		fmt.Printf("❌ Failed to query matches: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	matchCount := 0
	currentUser := ""
	for rows.Next() {
		var userID, email, productName, providerName string
		var creditScore int
		var monthlyIncome float64
		var minCredit int
		var minIncome float64

		err := rows.Scan(&userID, &email, &creditScore, &monthlyIncome, &productName, &providerName, &minCredit, &minIncome)
		if err != nil {
			continue
		}

		if userID != currentUser {
			if currentUser != "" {
				fmt.Println()
			}
			fmt.Printf("👤 User: %s (Credit: %d, Income: ₹%.0f)\n", userID, creditScore, monthlyIncome)
			currentUser = userID
		}
		fmt.Printf("   ✓ %s from %s\n", productName, providerName)
		matchCount++
	}

	fmt.Println()
	fmt.Printf("🎉 Found %d total matches!\n", matchCount)

	// Insert matches into database
	fmt.Println()
	fmt.Println("💾 Saving matches to database...")

	result, err := conn.Exec(ctx, `
		INSERT INTO matches (user_id, product_id, match_score, status, income_eligible, credit_score_eligible, age_eligible, employment_eligible, batch_id)
		SELECT 
			u.id,
			p.id,
			CASE 
				WHEN u.credit_score >= p.min_credit_score + 50 THEN 90
				WHEN u.credit_score >= p.min_credit_score + 20 THEN 75
				ELSE 60
			END as match_score,
			'pending',
			true,
			true,
			true,
			CASE WHEN p.accepted_employment_status IS NULL OR u.employment_status = ANY(p.accepted_employment_status) THEN true ELSE false END,
			u.batch_id
		FROM users u
		CROSS JOIN loan_products p
		WHERE u.credit_score >= p.min_credit_score
		  AND u.monthly_income >= p.min_monthly_income
		  AND u.age >= p.min_age
		  AND u.age <= p.max_age
		  AND p.is_active = true
		ON CONFLICT (user_id, product_id) DO UPDATE SET
			match_score = EXCLUDED.match_score,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		fmt.Printf("⚠️  Error saving matches: %v\n", err)
	} else {
		fmt.Printf("✅ Saved %d matches!\n", result.RowsAffected())
	}

	// Summary
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("              TEST COMPLETE")
	fmt.Println("═══════════════════════════════════════════")

	var totalUsers, totalProducts, totalMatches int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM loan_products").Scan(&totalProducts)
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM matches").Scan(&totalMatches)

	fmt.Printf("📊 Users:    %d\n", totalUsers)
	fmt.Printf("📦 Products: %d\n", totalProducts)
	fmt.Printf("🎯 Matches:  %d\n", totalMatches)
	fmt.Println("═══════════════════════════════════════════")
}
