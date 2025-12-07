//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Println("⚠️  No .env file found, using environment variables")
	}

	fmt.Println("🔍 Testing AWS Connections...\n")

	// Test 1: Check environment variables
	fmt.Println("1️⃣  Checking Environment Variables:")
	checkEnvVar("AWS_REGION")
	checkEnvVar("S3_BUCKET")
	checkEnvVar("DATABASE_URL")
	checkEnvVar("SES_SENDER_EMAIL")
	checkEnvVar("GEMINI_API_KEY")
	fmt.Println()

	// Test 2: Database connection
	fmt.Println("2️⃣  Testing Database Connection:")
	testDatabaseConnection()
	fmt.Println()

	fmt.Println("✅ Connection tests complete!")
}

func checkEnvVar(name string) {
	value := os.Getenv(name)
	if value == "" {
		fmt.Printf("   ❌ %s: NOT SET\n", name)
	} else {
		// Mask sensitive values
		masked := value
		if len(value) > 8 && (name == "DATABASE_URL" || name == "GEMINI_API_KEY") {
			masked = value[:8] + "..." + value[len(value)-4:]
		}
		fmt.Printf("   ✅ %s: %s\n", name, masked)
	}
}

func testDatabaseConnection() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("   ❌ DATABASE_URL not set, skipping database test")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Printf("   ❌ Database connection failed: %v\n", err)
		return
	}
	defer conn.Close(ctx)

	// Test query
	var result int
	err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		fmt.Printf("   ❌ Database query failed: %v\n", err)
		return
	}

	fmt.Println("   ✅ Database connection successful!")

	// Check if tables exist
	var tableCount int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables 
		WHERE table_schema = 'public' AND table_name IN ('users', 'loan_products', 'matches')
	`).Scan(&tableCount)

	if err == nil {
		fmt.Printf("   📊 Tables found: %d/3 (users, loan_products, matches)\n", tableCount)
	}
}
