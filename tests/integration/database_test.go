// Package integration_test contains integration tests
package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"loan-eligibility-engine/internal/config"
	"loan-eligibility-engine/internal/models"
	"loan-eligibility-engine/internal/services/database"
)

var testDB *database.DB

func TestMain(m *testing.M) {
	// Skip integration tests if no database URL is provided
	if os.Getenv("DATABASE_URL") == "" {
		os.Exit(0)
	}

	// Setup
	var err error
	testDB, err = database.New(context.Background())
	if err != nil {
		panic("Failed to connect to test database: " + err.Error())
	}

	// Run tests
	code := m.Run()

	// Cleanup
	testDB.Close()
	os.Exit(code)
}

func TestDatabaseConnection(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := testDB.Ping(ctx); err != nil {
		t.Errorf("Database ping failed: %v", err)
	}
}

func TestUserRepository_CRUD(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}

	ctx := context.Background()

	// Create
	user := &models.User{
		Name:               "Test User",
		Email:              "test-" + time.Now().Format("20060102150405") + "@example.com",
		Age:                30,
		AnnualIncome:       500000,
		CreditScore:        750,
		EmploymentStatus:   models.EmploymentStatusSalaried,
		LoanAmountRequired: 200000,
		Location:           "Mumbai",
	}

	err := testDB.Users().Create(ctx, user)
	if err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	if user.ID == "" {
		t.Error("User ID should be set after creation")
	}

	// Read
	retrieved, err := testDB.Users().GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Get user failed: %v", err)
	}

	if retrieved.Email != user.Email {
		t.Errorf("Retrieved email = %q, want %q", retrieved.Email, user.Email)
	}

	// Update
	user.CreditScore = 800
	err = testDB.Users().Update(ctx, user)
	if err != nil {
		t.Fatalf("Update user failed: %v", err)
	}

	updated, _ := testDB.Users().GetByID(ctx, user.ID)
	if updated.CreditScore != 800 {
		t.Errorf("Credit score not updated: got %d, want 800", updated.CreditScore)
	}

	// Delete
	err = testDB.Users().Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("Delete user failed: %v", err)
	}

	_, err = testDB.Users().GetByID(ctx, user.ID)
	if err == nil {
		t.Error("User should not exist after deletion")
	}
}

func TestUserRepository_BulkInsert(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}

	ctx := context.Background()
	timestamp := time.Now().Format("20060102150405")

	users := []*models.User{
		{
			Name:               "Bulk User 1",
			Email:              "bulk1-" + timestamp + "@example.com",
			Age:                25,
			AnnualIncome:       400000,
			CreditScore:        700,
			EmploymentStatus:   models.EmploymentStatusSalaried,
			LoanAmountRequired: 150000,
		},
		{
			Name:               "Bulk User 2",
			Email:              "bulk2-" + timestamp + "@example.com",
			Age:                35,
			AnnualIncome:       600000,
			CreditScore:        750,
			EmploymentStatus:   models.EmploymentStatusSelfEmployed,
			LoanAmountRequired: 300000,
		},
		{
			Name:               "Bulk User 3",
			Email:              "bulk3-" + timestamp + "@example.com",
			Age:                45,
			AnnualIncome:       800000,
			CreditScore:        800,
			EmploymentStatus:   models.EmploymentStatusBusiness,
			LoanAmountRequired: 500000,
		},
	}

	inserted, err := testDB.Users().BulkInsert(ctx, users)
	if err != nil {
		t.Fatalf("Bulk insert failed: %v", err)
	}

	if inserted != len(users) {
		t.Errorf("Inserted count = %d, want %d", inserted, len(users))
	}

	// Verify each user was created
	for _, user := range users {
		if user.ID == "" {
			t.Error("User ID should be set after bulk insert")
		}
	}

	// Cleanup
	for _, user := range users {
		testDB.Users().Delete(ctx, user.ID)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}

	ctx := context.Background()
	email := "getbyemail-" + time.Now().Format("20060102150405") + "@example.com"

	user := &models.User{
		Name:               "Email Test User",
		Email:              email,
		Age:                30,
		AnnualIncome:       500000,
		CreditScore:        750,
		EmploymentStatus:   models.EmploymentStatusSalaried,
		LoanAmountRequired: 200000,
	}

	testDB.Users().Create(ctx, user)
	defer testDB.Users().Delete(ctx, user.ID)

	retrieved, err := testDB.Users().GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("Retrieved wrong user: got ID %s, want %s", retrieved.ID, user.ID)
	}
}

func TestProductRepository_CRUD(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}

	ctx := context.Background()

	product := &models.LoanProduct{
		Name:                   "Test Product",
		Provider:               "Test Bank",
		ProductType:            models.ProductTypePersonalLoan,
		InterestRateMin:        10.5,
		InterestRateMax:        18.0,
		MinLoanAmount:          50000,
		MaxLoanAmount:          2500000,
		MinCreditScore:         700,
		MinAnnualIncome:        300000,
		MinAge:                 21,
		MaxAge:                 60,
		AllowedEmploymentTypes: []string{"salaried", "self_employed"},
		IsActive:               true,
	}

	err := testDB.Products().Create(ctx, product)
	if err != nil {
		t.Fatalf("Create product failed: %v", err)
	}

	if product.ID == "" {
		t.Error("Product ID should be set after creation")
	}

	// Read
	retrieved, err := testDB.Products().GetByID(ctx, product.ID)
	if err != nil {
		t.Fatalf("Get product failed: %v", err)
	}

	if retrieved.Name != product.Name {
		t.Errorf("Retrieved name = %q, want %q", retrieved.Name, product.Name)
	}

	// Update
	product.InterestRateMin = 11.0
	err = testDB.Products().Update(ctx, product)
	if err != nil {
		t.Fatalf("Update product failed: %v", err)
	}

	// Get active products
	active, err := testDB.Products().GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive failed: %v", err)
	}

	found := false
	for _, p := range active {
		if p.ID == product.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created product should be in active products list")
	}

	// Cleanup
	testDB.Products().Delete(ctx, product.ID)
}

func TestMatchRepository_SQLPrefilter(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}

	ctx := context.Background()

	// Create test users
	timestamp := time.Now().Format("20060102150405")
	users := []*models.User{
		{
			Name:               "Prefilter User 1",
			Email:              "prefilter1-" + timestamp + "@example.com",
			Age:                30,
			AnnualIncome:       600000,
			CreditScore:        750,
			EmploymentStatus:   models.EmploymentStatusSalaried,
			LoanAmountRequired: 500000,
		},
		{
			Name:               "Prefilter User 2",
			Email:              "prefilter2-" + timestamp + "@example.com",
			Age:                25,
			AnnualIncome:       200000, // Below minimum
			CreditScore:        650,    // Below minimum
			EmploymentStatus:   models.EmploymentStatusStudent,
			LoanAmountRequired: 100000,
		},
	}

	for _, user := range users {
		testDB.Users().Create(ctx, user)
	}
	defer func() {
		for _, user := range users {
			testDB.Users().Delete(ctx, user.ID)
		}
	}()

	// Create test product
	product := &models.LoanProduct{
		Name:                   "Prefilter Test Product",
		Provider:               "Test Bank",
		InterestRateMin:        10.5,
		InterestRateMax:        18.0,
		MinLoanAmount:          50000,
		MaxLoanAmount:          2500000,
		MinCreditScore:         700,
		MinAnnualIncome:        300000,
		MinAge:                 21,
		MaxAge:                 60,
		AllowedEmploymentTypes: []string{"salaried", "self_employed"},
		IsActive:               true,
	}
	testDB.Products().Create(ctx, product)
	defer testDB.Products().Delete(ctx, product.ID)

	// Run SQL prefilter
	userIDs := []string{users[0].ID, users[1].ID}
	candidates, err := testDB.Matches().SQLPrefilterMatches(ctx, userIDs)
	if err != nil {
		t.Fatalf("SQLPrefilterMatches failed: %v", err)
	}

	// User 1 should match, User 2 should not
	matchedUser1 := false
	matchedUser2 := false
	for _, c := range candidates {
		if c.UserID == users[0].ID {
			matchedUser1 = true
		}
		if c.UserID == users[1].ID {
			matchedUser2 = true
		}
	}

	if !matchedUser1 {
		t.Error("User 1 should match the prefilter criteria")
	}
	if matchedUser2 {
		t.Error("User 2 should NOT match the prefilter criteria")
	}
}

func TestConfigLoading(t *testing.T) {
	cfg := config.Get()

	if cfg.AWSRegion == "" {
		cfg.AWSRegion = "ap-south-1"
	}

	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	// Verify defaults are set
	if cfg.Environment != "development" && cfg.Environment != "production" && cfg.Environment != "staging" {
		t.Errorf("Invalid environment: %s", cfg.Environment)
	}
}

func TestDatabaseTransaction(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not configured")
	}

	ctx := context.Background()
	timestamp := time.Now().Format("20060102150405")

	err := testDB.Transaction(ctx, func(tx *database.DB) error {
		// Create user in transaction
		user := &models.User{
			Name:               "Transaction User",
			Email:              "tx-" + timestamp + "@example.com",
			Age:                30,
			AnnualIncome:       500000,
			CreditScore:        750,
			EmploymentStatus:   models.EmploymentStatusSalaried,
			LoanAmountRequired: 200000,
		}

		err := tx.Users().Create(ctx, user)
		if err != nil {
			return err
		}

		// Verify user exists in transaction
		_, err = tx.Users().GetByID(ctx, user.ID)
		if err != nil {
			return err
		}

		// Cleanup
		return tx.Users().Delete(ctx, user.ID)
	})

	if err != nil {
		t.Errorf("Transaction failed: %v", err)
	}
}
