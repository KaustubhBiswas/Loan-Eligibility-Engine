package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("=== Database Initialization Script ===")
	fmt.Println()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Printf("⚠️  Warning: Could not load .env file: %v\n", err)
	}

	// Get database URL
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Println("❌ DATABASE_URL environment variable not set")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First connect to default 'postgres' database to create our database
	postgresURL := strings.Replace(databaseURL, "/loanengine", "/postgres", 1)
	fmt.Println("📡 Connecting to PostgreSQL server...")

	adminConn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		fmt.Printf("❌ Failed to connect to PostgreSQL: %v\n", err)
		os.Exit(1)
	}

	// Check if database exists
	var exists bool
	err = adminConn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = 'loanengine')").Scan(&exists)
	if err != nil {
		fmt.Printf("❌ Failed to check database existence: %v\n", err)
		adminConn.Close(ctx)
		os.Exit(1)
	}

	if !exists {
		fmt.Println("📦 Creating 'loanengine' database...")
		_, err = adminConn.Exec(ctx, "CREATE DATABASE loanengine")
		if err != nil {
			fmt.Printf("❌ Failed to create database: %v\n", err)
			adminConn.Close(ctx)
			os.Exit(1)
		}
		fmt.Println("✅ Database 'loanengine' created!")
	} else {
		fmt.Println("✅ Database 'loanengine' already exists")
	}
	adminConn.Close(ctx)

	// Now connect to the loanengine database
	fmt.Println("📡 Connecting to loanengine database...")
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Printf("❌ Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	fmt.Println("✅ Connected to database successfully!")
	fmt.Println()

	// Read SQL file
	fmt.Println("📖 Reading SQL schema file...")
	sqlBytes, err := os.ReadFile("scripts/init_database.sql")
	if err != nil {
		fmt.Printf("❌ Failed to read SQL file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ SQL file loaded successfully!")
	fmt.Println()

	// Execute SQL
	fmt.Println("🚀 Executing database schema...")
	_, err = conn.Exec(ctx, string(sqlBytes))
	if err != nil {
		fmt.Printf("❌ Failed to execute SQL: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Database schema executed successfully!")
	fmt.Println()

	// Verify by counting tables and products
	fmt.Println("🔍 Verifying database setup...")

	// Count loan products
	var productCount int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM loan_products").Scan(&productCount)
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not count products: %v\n", err)
	} else {
		fmt.Printf("   📦 Loan products in database: %d\n", productCount)
	}

	// List loan products
	rows, err := conn.Query(ctx, "SELECT id, product_name, provider_name, min_credit_score, min_monthly_income FROM loan_products")
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not fetch products: %v\n", err)
	} else {
		defer rows.Close()
		fmt.Println()
		fmt.Println("   📋 Loan Products:")
		fmt.Println("   ─────────────────────────────────────────────────────────")
		for rows.Next() {
			var id int
			var name, provider string
			var minCredit int
			var minIncome float64
			if err := rows.Scan(&id, &name, &provider, &minCredit, &minIncome); err == nil {
				fmt.Printf("   %d. %s (%s)\n", id, name, provider)
				fmt.Printf("      Min Credit: %d | Min Income: ₹%.0f\n", minCredit, minIncome)
			}
		}
		fmt.Println("   ─────────────────────────────────────────────────────────")
	}

	fmt.Println()
	fmt.Println("🎉 Database initialization completed successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Test the connection: go run scripts/test_connection.go")
	fmt.Println("  2. Run the Lambda locally or deploy to AWS")
}
