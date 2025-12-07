-- Loan Eligibility Engine Database Schema
-- PostgreSQL 15+ (Aligned with Go models using SERIAL IDs)

-- Drop existing tables if they exist (for clean setup)
DROP TABLE IF EXISTS notification_logs CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS matches CASCADE;
DROP TABLE IF EXISTS user_loan_matches CASCADE;
DROP TABLE IF EXISTS upload_batches CASCADE;
DROP TABLE IF EXISTS crawler_runs CASCADE;
DROP TABLE IF EXISTS loan_products CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Drop existing types if they exist
DROP TYPE IF EXISTS employment_status CASCADE;
DROP TYPE IF EXISTS product_type CASCADE;
DROP TYPE IF EXISTS match_status CASCADE;
DROP TYPE IF EXISTS match_source CASCADE;

-- Users Table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) NOT NULL,
    monthly_income DECIMAL(12,2) NOT NULL,
    credit_score INTEGER NOT NULL CHECK (credit_score >= 300 AND credit_score <= 900),
    employment_status VARCHAR(50) NOT NULL,
    age INTEGER NOT NULL CHECK (age >= 18 AND age <= 120),
    batch_id VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
);

-- Indexes for users
CREATE INDEX idx_users_user_id ON users(user_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_credit_score ON users(credit_score);
CREATE INDEX idx_users_monthly_income ON users(monthly_income);
CREATE INDEX idx_users_batch_id ON users(batch_id);
CREATE INDEX idx_users_employment_status ON users(employment_status);

-- Loan Products Table
CREATE TABLE loan_products (
    id SERIAL PRIMARY KEY,
    product_name VARCHAR(200) NOT NULL,
    provider_name VARCHAR(200) NOT NULL,
    product_type VARCHAR(50) DEFAULT 'personal',
    interest_rate_min DECIMAL(5,2) NOT NULL CHECK (interest_rate_min >= 0),
    interest_rate_max DECIMAL(5,2) NOT NULL CHECK (interest_rate_max >= 0),
    loan_amount_min DECIMAL(15,2) NOT NULL CHECK (loan_amount_min >= 0),
    loan_amount_max DECIMAL(15,2) NOT NULL CHECK (loan_amount_max >= 0),
    tenure_min_months INTEGER NOT NULL DEFAULT 12,
    tenure_max_months INTEGER NOT NULL DEFAULT 60,
    min_monthly_income DECIMAL(12,2) NOT NULL,
    min_credit_score INTEGER NOT NULL CHECK (min_credit_score >= 300 AND min_credit_score <= 900),
    max_credit_score INTEGER,
    min_age INTEGER DEFAULT 21 CHECK (min_age >= 18),
    max_age INTEGER DEFAULT 65 CHECK (max_age <= 120),
    accepted_employment_status TEXT[],
    processing_fee_percent DECIMAL(5,2),
    source_url VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    last_crawled_at TIMESTAMP,
    
    CONSTRAINT valid_loan_amount_range CHECK (loan_amount_max >= loan_amount_min),
    CONSTRAINT valid_interest_rate_range CHECK (interest_rate_max >= interest_rate_min),
    CONSTRAINT valid_age_range CHECK (max_age >= min_age),
    CONSTRAINT valid_tenure_range CHECK (tenure_max_months >= tenure_min_months),
    CONSTRAINT unique_provider_product UNIQUE (provider_name, product_name)
);

-- Indexes for loan_products
CREATE INDEX idx_products_provider ON loan_products(provider_name);
CREATE INDEX idx_products_min_income ON loan_products(min_monthly_income);
CREATE INDEX idx_products_min_credit ON loan_products(min_credit_score);
CREATE INDEX idx_products_is_active ON loan_products(is_active);

-- Matches Table
CREATE TABLE matches (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES loan_products(id) ON DELETE CASCADE,
    match_score DECIMAL(5,2) DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    match_source VARCHAR(50) DEFAULT 'sql_filter',
    income_eligible BOOLEAN DEFAULT FALSE,
    credit_score_eligible BOOLEAN DEFAULT FALSE,
    age_eligible BOOLEAN DEFAULT FALSE,
    employment_eligible BOOLEAN DEFAULT FALSE,
    llm_analysis TEXT,
    llm_confidence DECIMAL(3,2),
    batch_id VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notified_at TIMESTAMP,
    UNIQUE(user_id, product_id)
);

-- Indexes for matches
CREATE INDEX idx_matches_user_id ON matches(user_id);
CREATE INDEX idx_matches_product_id ON matches(product_id);
CREATE INDEX idx_matches_status ON matches(status);
CREATE INDEX idx_matches_batch_id ON matches(batch_id);
CREATE INDEX idx_matches_score ON matches(match_score DESC);

-- Notifications Table
CREATE TABLE notifications (
    id SERIAL PRIMARY KEY,
    match_id INTEGER REFERENCES matches(id) ON DELETE SET NULL,
    user_db_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending',
    message_id VARCHAR(255),
    error_message TEXT
);

-- Indexes for notifications
CREATE INDEX idx_notifications_user ON notifications(user_db_id);
CREATE INDEX idx_notifications_status ON notifications(status);

-- Upload Batches Table
CREATE TABLE upload_batches (
    id SERIAL PRIMARY KEY,
    batch_id VARCHAR(50) UNIQUE NOT NULL,
    s3_key VARCHAR(512) NOT NULL,
    file_name VARCHAR(255),
    total_rows INTEGER NOT NULL DEFAULT 0,
    successful_rows INTEGER NOT NULL DEFAULT 0,
    failed_rows INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'processing',
    error_details TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_batches_batch_id ON upload_batches(batch_id);
CREATE INDEX idx_batches_status ON upload_batches(status);

-- Crawler Runs Table
CREATE TABLE crawler_runs (
    id SERIAL PRIMARY KEY,
    source_name VARCHAR(255),
    source_url TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    products_found INTEGER DEFAULT 0,
    products_added INTEGER DEFAULT 0,
    products_updated INTEGER DEFAULT 0,
    products_failed INTEGER DEFAULT 0,
    sources_crawled INTEGER DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_crawler_source ON crawler_runs(source_name);
CREATE INDEX idx_crawler_status ON crawler_runs(status);

-- Notification Logs Table (for n8n workflow tracking)
CREATE TABLE notification_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    email VARCHAR(255) NOT NULL,
    notification_type VARCHAR(50) NOT NULL DEFAULT 'loan_match',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    subject VARCHAR(500),
    message_id VARCHAR(255),
    error_message TEXT,
    match_count INTEGER DEFAULT 0,
    sent_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notification_logs_user ON notification_logs(user_id);
CREATE INDEX idx_notification_logs_status ON notification_logs(status);
CREATE INDEX idx_notification_logs_email ON notification_logs(email);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_loan_products_updated_at
    BEFORE UPDATE ON loan_products
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_matches_updated_at
    BEFORE UPDATE ON matches
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert sample loan products
INSERT INTO loan_products (
    product_name, provider_name, product_type,
    interest_rate_min, interest_rate_max,
    loan_amount_min, loan_amount_max,
    tenure_min_months, tenure_max_months,
    min_monthly_income, min_credit_score,
    min_age, max_age,
    accepted_employment_status,
    processing_fee_percent,
    source_url
) VALUES 
(
    'HDFC Personal Loan',
    'HDFC Bank',
    'personal',
    10.50, 21.00,
    50000, 4000000,
    12, 60,
    25000, 700,
    21, 60,
    ARRAY['employed', 'self_employed'],
    2.50,
    'https://www.hdfcbank.com/personal/borrow/popular-loans/personal-loan'
),
(
    'ICICI Instant Personal Loan',
    'ICICI Bank',
    'personal',
    10.75, 19.00,
    50000, 2500000,
    12, 72,
    20833, 680,
    23, 58,
    ARRAY['employed'],
    1.99,
    'https://www.icicibank.com/personal-banking/loans/personal-loan'
),
(
    'SBI Express Personal Loan',
    'State Bank of India',
    'personal',
    11.00, 14.50,
    100000, 3500000,
    12, 84,
    16666, 650,
    21, 65,
    ARRAY['employed', 'self_employed', 'retired'],
    1.00,
    'https://sbi.co.in/web/personal-banking/loans/personal-loans'
),
(
    'Bajaj Finserv Flexi Loan',
    'Bajaj Finserv',
    'personal',
    12.00, 24.00,
    25000, 2500000,
    12, 60,
    29166, 720,
    25, 55,
    ARRAY['employed'],
    2.00,
    'https://www.bajajfinserv.in/personal-loan'
),
(
    'Axis Bank Personal Loan',
    'Axis Bank',
    'personal',
    10.49, 22.00,
    50000, 1500000,
    12, 60,
    15000, 700,
    21, 60,
    ARRAY['employed', 'self_employed'],
    1.50,
    'https://www.axisbank.com/retail/loans/personal-loan'
);

-- Summary comments
COMMENT ON TABLE users IS 'User profiles with financial information for loan eligibility';
COMMENT ON TABLE loan_products IS 'Loan products from various banks and financial institutions';
COMMENT ON TABLE matches IS 'User-to-loan product matching results with eligibility scores';
COMMENT ON TABLE notifications IS 'Email notification delivery tracking';
COMMENT ON TABLE upload_batches IS 'Tracking table for CSV upload processing';
COMMENT ON TABLE crawler_runs IS 'Execution history of the loan product web crawler';

-- Verify setup
SELECT 'Database schema created successfully!' AS status;
SELECT 'Tables created: ' || COUNT(*)::text FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE';
SELECT 'Sample loan products: ' || COUNT(*)::text FROM loan_products;
