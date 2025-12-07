-- Loan Eligibility Engine Database Schema
-- PostgreSQL 15+

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Employment Status Enum
CREATE TYPE employment_status AS ENUM (
    'salaried',
    'self_employed',
    'business',
    'unemployed',
    'retired',
    'student'
);

-- Product Type Enum
CREATE TYPE product_type AS ENUM (
    'personal_loan',
    'home_loan',
    'car_loan',
    'education_loan',
    'business_loan',
    'credit_card'
);

-- Match Status Enum
CREATE TYPE match_status AS ENUM (
    'pending',
    'qualified',
    'disqualified',
    'notified',
    'applied',
    'error'
);

-- Users Table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    age INTEGER NOT NULL CHECK (age >= 18 AND age <= 100),
    annual_income DECIMAL(15, 2) NOT NULL CHECK (annual_income >= 0),
    credit_score INTEGER NOT NULL CHECK (credit_score >= 300 AND credit_score <= 900),
    employment_status employment_status NOT NULL,
    loan_amount_required DECIMAL(15, 2) NOT NULL CHECK (loan_amount_required > 0),
    location VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for users
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_credit_score ON users(credit_score);
CREATE INDEX idx_users_annual_income ON users(annual_income);
CREATE INDEX idx_users_employment_status ON users(employment_status);
CREATE INDEX idx_users_location ON users(location);
CREATE INDEX idx_users_created_at ON users(created_at);

-- Loan Products Table
CREATE TABLE IF NOT EXISTS loan_products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(255) NOT NULL,
    product_type product_type NOT NULL DEFAULT 'personal_loan',
    interest_rate_min DECIMAL(5, 2) NOT NULL CHECK (interest_rate_min >= 0),
    interest_rate_max DECIMAL(5, 2) NOT NULL CHECK (interest_rate_max >= 0),
    min_loan_amount DECIMAL(15, 2) NOT NULL CHECK (min_loan_amount >= 0),
    max_loan_amount DECIMAL(15, 2) NOT NULL CHECK (max_loan_amount >= 0),
    min_credit_score INTEGER NOT NULL CHECK (min_credit_score >= 300 AND min_credit_score <= 900),
    min_annual_income DECIMAL(15, 2) NOT NULL CHECK (min_annual_income >= 0),
    min_age INTEGER NOT NULL DEFAULT 18 CHECK (min_age >= 18),
    max_age INTEGER NOT NULL DEFAULT 65 CHECK (max_age <= 100),
    allowed_employment_types TEXT[] NOT NULL DEFAULT '{}',
    processing_fee_percent DECIMAL(5, 2) DEFAULT 0,
    tenure_min_months INTEGER DEFAULT 12,
    tenure_max_months INTEGER DEFAULT 60,
    eligible_locations TEXT[] DEFAULT '{}',
    source_url TEXT,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_crawled_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_loan_amount_range CHECK (max_loan_amount >= min_loan_amount),
    CONSTRAINT valid_interest_rate_range CHECK (interest_rate_max >= interest_rate_min),
    CONSTRAINT valid_age_range CHECK (max_age >= min_age),
    CONSTRAINT valid_tenure_range CHECK (tenure_max_months >= tenure_min_months)
);

-- Indexes for loan_products
CREATE INDEX idx_loan_products_provider ON loan_products(provider);
CREATE INDEX idx_loan_products_product_type ON loan_products(product_type);
CREATE INDEX idx_loan_products_min_credit_score ON loan_products(min_credit_score);
CREATE INDEX idx_loan_products_min_annual_income ON loan_products(min_annual_income);
CREATE INDEX idx_loan_products_is_active ON loan_products(is_active);
CREATE INDEX idx_loan_products_interest_rate ON loan_products(interest_rate_min, interest_rate_max);
CREATE INDEX idx_loan_products_loan_amount ON loan_products(min_loan_amount, max_loan_amount);

-- User-Loan Matches Table
CREATE TABLE IF NOT EXISTS user_loan_matches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    loan_product_id UUID NOT NULL REFERENCES loan_products(id) ON DELETE CASCADE,
    status match_status NOT NULL DEFAULT 'pending',
    eligibility_score DECIMAL(5, 2) CHECK (eligibility_score >= 0 AND eligibility_score <= 100),
    
    -- Pre-filter results
    sql_prefilter_passed BOOLEAN DEFAULT FALSE,
    logic_filter_passed BOOLEAN DEFAULT FALSE,
    llm_check_passed BOOLEAN,
    
    -- LLM analysis
    llm_reasoning TEXT,
    llm_confidence DECIMAL(5, 2),
    llm_model VARCHAR(100),
    
    -- Notification tracking
    notification_sent_at TIMESTAMP WITH TIME ZONE,
    notification_type VARCHAR(50),
    notification_id VARCHAR(255),
    
    -- Processing metadata
    processing_time_ms INTEGER,
    error_message TEXT,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, loan_product_id)
);

-- Indexes for user_loan_matches
CREATE INDEX idx_matches_user_id ON user_loan_matches(user_id);
CREATE INDEX idx_matches_loan_product_id ON user_loan_matches(loan_product_id);
CREATE INDEX idx_matches_status ON user_loan_matches(status);
CREATE INDEX idx_matches_eligibility_score ON user_loan_matches(eligibility_score DESC);
CREATE INDEX idx_matches_created_at ON user_loan_matches(created_at);
CREATE INDEX idx_matches_notification_sent ON user_loan_matches(notification_sent_at);

-- Composite index for efficient matching queries
CREATE INDEX idx_matches_user_status ON user_loan_matches(user_id, status);
CREATE INDEX idx_matches_product_status ON user_loan_matches(loan_product_id, status);

-- Upload Batches Table (track CSV uploads)
CREATE TABLE IF NOT EXISTS upload_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    s3_key VARCHAR(512) NOT NULL,
    file_name VARCHAR(255),
    total_rows INTEGER NOT NULL DEFAULT 0,
    successful_rows INTEGER NOT NULL DEFAULT 0,
    failed_rows INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'processing',
    error_details JSONB,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_upload_batches_status ON upload_batches(status);
CREATE INDEX idx_upload_batches_created_at ON upload_batches(created_at);

-- Crawler Runs Table (track web crawler executions)
CREATE TABLE IF NOT EXISTS crawler_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_name VARCHAR(255) NOT NULL,
    source_url TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    products_found INTEGER DEFAULT 0,
    products_added INTEGER DEFAULT 0,
    products_updated INTEGER DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_crawler_runs_source ON crawler_runs(source_name);
CREATE INDEX idx_crawler_runs_status ON crawler_runs(status);
CREATE INDEX idx_crawler_runs_created_at ON crawler_runs(created_at);

-- Notification Log Table
CREATE TABLE IF NOT EXISTS notification_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    match_id UUID REFERENCES user_loan_matches(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type VARCHAR(50) NOT NULL,
    recipient_email VARCHAR(255) NOT NULL,
    subject VARCHAR(500),
    template_name VARCHAR(100),
    ses_message_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    sent_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notification_logs_user_id ON notification_logs(user_id);
CREATE INDEX idx_notification_logs_match_id ON notification_logs(match_id);
CREATE INDEX idx_notification_logs_status ON notification_logs(status);
CREATE INDEX idx_notification_logs_sent_at ON notification_logs(sent_at);

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

CREATE TRIGGER update_user_loan_matches_updated_at
    BEFORE UPDATE ON user_loan_matches
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- View for user eligibility summary
CREATE OR REPLACE VIEW user_eligibility_summary AS
SELECT 
    u.id AS user_id,
    u.name,
    u.email,
    COUNT(DISTINCT m.loan_product_id) AS total_matches,
    COUNT(DISTINCT CASE WHEN m.status = 'qualified' THEN m.loan_product_id END) AS qualified_matches,
    COUNT(DISTINCT CASE WHEN m.status = 'notified' THEN m.loan_product_id END) AS notified_matches,
    MAX(m.eligibility_score) AS best_eligibility_score,
    AVG(m.eligibility_score) AS avg_eligibility_score
FROM users u
LEFT JOIN user_loan_matches m ON u.id = m.user_id
GROUP BY u.id, u.name, u.email;

-- View for product performance
CREATE OR REPLACE VIEW product_performance AS
SELECT 
    lp.id AS product_id,
    lp.name AS product_name,
    lp.provider,
    COUNT(DISTINCT m.user_id) AS total_interested_users,
    COUNT(DISTINCT CASE WHEN m.status = 'qualified' THEN m.user_id END) AS qualified_users,
    COUNT(DISTINCT CASE WHEN m.status = 'applied' THEN m.user_id END) AS applied_users,
    AVG(m.eligibility_score) AS avg_eligibility_score
FROM loan_products lp
LEFT JOIN user_loan_matches m ON lp.id = m.loan_product_id
GROUP BY lp.id, lp.name, lp.provider;

-- Sample data for testing
INSERT INTO loan_products (
    name, provider, product_type, 
    interest_rate_min, interest_rate_max,
    min_loan_amount, max_loan_amount,
    min_credit_score, min_annual_income,
    min_age, max_age,
    allowed_employment_types,
    processing_fee_percent,
    tenure_min_months, tenure_max_months,
    source_url, description
) VALUES 
(
    'HDFC Personal Loan',
    'HDFC Bank',
    'personal_loan',
    10.50, 21.00,
    50000, 4000000,
    700, 300000,
    21, 60,
    ARRAY['salaried', 'self_employed'],
    2.50,
    12, 60,
    'https://www.hdfcbank.com/personal/borrow/popular-loans/personal-loan',
    'Quick personal loans with minimal documentation for salaried and self-employed individuals.'
),
(
    'ICICI Instant Personal Loan',
    'ICICI Bank',
    'personal_loan',
    10.75, 19.00,
    50000, 2500000,
    680, 250000,
    23, 58,
    ARRAY['salaried'],
    1.99,
    12, 72,
    'https://www.icicibank.com/personal-banking/loans/personal-loan',
    'Instant approval personal loans for existing ICICI customers.'
),
(
    'SBI Express Personal Loan',
    'State Bank of India',
    'personal_loan',
    11.00, 14.50,
    100000, 3500000,
    650, 200000,
    21, 65,
    ARRAY['salaried', 'self_employed', 'business'],
    1.00,
    12, 84,
    'https://sbi.co.in/web/personal-banking/loans/personal-loans',
    'Affordable personal loans from India''s largest public sector bank.'
),
(
    'Bajaj Finserv Flexi Loan',
    'Bajaj Finserv',
    'personal_loan',
    12.00, 24.00,
    25000, 2500000,
    720, 350000,
    25, 55,
    ARRAY['salaried'],
    2.00,
    12, 60,
    'https://www.bajajfinserv.in/personal-loan',
    'Flexi loan with interest-only EMI option for better cash flow management.'
),
(
    'Axis Bank Personal Loan',
    'Axis Bank',
    'personal_loan',
    10.49, 22.00,
    50000, 1500000,
    700, 180000,
    21, 60,
    ARRAY['salaried', 'self_employed'],
    1.50,
    12, 60,
    'https://www.axisbank.com/retail/loans/personal-loan',
    'Personal loans with competitive rates and quick disbursement.'
);

-- Grant permissions (adjust role name as needed)
-- GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO loan_engine_app;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO loan_engine_app;

COMMENT ON TABLE users IS 'User profiles with financial information for loan eligibility';
COMMENT ON TABLE loan_products IS 'Personal loan products crawled from various providers';
COMMENT ON TABLE user_loan_matches IS 'User-to-loan product matching results with eligibility scores';
COMMENT ON TABLE upload_batches IS 'Tracking table for CSV upload processing';
COMMENT ON TABLE crawler_runs IS 'Execution history of the loan product web crawler';
COMMENT ON TABLE notification_logs IS 'Email notification delivery tracking';
