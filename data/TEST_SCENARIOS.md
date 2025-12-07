# CSV Test Files - End-to-End Testing Guide

## Test Files Overview

### 1. **test_high_income_users.csv** (5 users)
**Profile**: High-earning professionals with excellent credit
- **Annual Income**: ₹12L - ₹30L (Monthly: ₹1L - ₹2.5L)
- **Credit Score**: 750-850
- **Employment**: Salaried, Self-Employed, Business Owners
- **Age**: 30-45 years

**Expected Outcome**: 
- ✅ Should match with **premium loans** (Home Loan, Business Loan, Gold Loan)
- ✅ High match count (15-20+ matches expected)
- ✅ All 3 optimization stages should pass easily
- ✅ LLM should give highly positive qualitative assessment

---

### 2. **test_low_credit_users.csv** (5 users)
**Profile**: Average earners with poor to fair credit scores
- **Monthly Income**: ₹25K - ₹35K
- **Credit Score**: 550-650 (Below average)
- **Employment**: Salaried, Self-Employed, Part-Time
- **Age**: 26-32 years

**Expected Outcome**:
- ⚠️ Should match with **limited loans** (Personal Loan Small, Microfinance)
- ⚠️ Low match count (2-5 matches expected)
- ⚠️ SQL Prefilter will reject most premium loans
- ⚠️ LLM should flag credit risk concerns

---

### 3. **test_young_professionals.csv** (5 users)
**Profile**: Young salaried professionals, early career
- **Monthly Salary**: ₹42K - ₹55K
- **Credit Score**: 700-760 (Good)
- **Employment**: Mostly Salaried, one Self-Employed
- **Age**: 23-27 years

**Expected Outcome**:
- ✅ Should match with **starter loans** (Personal Loan, Two-Wheeler, Education)
- ✅ Moderate match count (8-12 matches expected)
- ✅ Good credit helps but income limits premium loans
- ✅ LLM should recognize young professional potential

---

### 4. **test_senior_users.csv** (5 users)
**Profile**: Senior citizens with stable income and excellent credit
- **Monthly Income**: ₹70K - ₹90K
- **Credit Score**: 760-820 (Excellent)
- **Employment**: Self-Employed, Business Owner, Salaried
- **Age**: 48-55 years

**Expected Outcome**:
- ✅ Should match with **senior-friendly loans** (Gold Loan, Home Loan, Personal Loan)
- ✅ High match count (12-18 matches expected)
- ⚠️ Age filter might exclude some loans with upper age limits
- ✅ LLM should recognize stability and experience

---

### 5. **test_mixed_profiles.csv** (8 users)
**Profile**: Diverse mix - low to high income, varied credit, all employment types
- **Annual Income**: ₹4.5L - ₹11L (Monthly: ₹37.5K - ₹92K)
- **Credit Score**: 640-790 (Full spectrum)
- **Employment**: Salaried, Self-Employed, Part-Time, Business Owner
- **Age**: 29-41 years

**Expected Outcome**:
- 🎯 **Most realistic test** - mimics real-world diversity
- 🎯 Variable match counts per user (2-15 matches)
- 🎯 Tests all 3 optimization stages thoroughly
- 🎯 LLM should provide nuanced assessments for different profiles

---

### 6. **test_edge_cases.csv** (6 users)
**Profile**: Extreme/boundary cases to test system limits
- **Very High Income**: ₹5L/month (₹60L annual)
- **Very Low Income**: ₹15K/month
- **Min Credit**: 300 (Very Poor)
- **Max Credit**: 900 (Exceptional)
- **Young Age**: 18 years
- **Senior Age**: 65 years

**Expected Outcome**:
- 🧪 Tests system boundaries and validation
- 🧪 Min credit user should get 0-1 matches
- 🧪 Max credit + high income should get 20+ matches
- 🧪 Age extremes test min/max age filters
- 🧪 Validates error handling and edge case logic

---

## Testing Workflow

### Step-by-Step Process:

1. **Clear Data** (Before Each Test)
   - Open: `http://localhost:8080/dashboard.html`
   - Click: **"Clear All Data"** button
   - Confirm the deletion

2. **Upload CSV**
   - Open: `http://localhost:8080/`
   - Select one test CSV file
   - Click: **"Upload CSV"**
   - Verify: "✅ X valid users" message

3. **Run Matching**
   - Go to: `http://localhost:8080/dashboard.html`
   - Click: **"Run Matching Workflow"**
   - Wait for: "✅ Matching workflow triggered successfully"
   - Check: Match count displays (e.g., "45 matches found")

4. **Send Test Notification**
   - Click: **"Send Test Notification"**
   - Enter: Test email address
   - Check email: Should receive loan recommendations with Gemini analysis

5. **Verify Results**
   - Check match count matches expected range
   - Verify email contains relevant loan products
   - Confirm LLM qualitative assessment makes sense

---

## Column Name Variations Tested

Each CSV uses **different column names** to test the parser's flexibility:

| Standard Column | Variations Used |
|----------------|-----------------|
| `user_id` | name, username, full_name, customer_id, user_id |
| `monthly_income` | annual_income (÷12), salary, monthly_income, income |
| `credit_score` | credit_score, cibil_score, cibil |
| `employment_status` | employment_status, occupation, job_status |
| `email` | email |
| `age` | age |

---

## Expected Performance Metrics

### Optimization Pipeline Reduction:
- **Stage 1 (SQL Prefilter)**: 70-80% reduction
- **Stage 2 (Logic Filter)**: 50-60% reduction
- **Stage 3 (LLM Check)**: Top 50 candidates for qualitative assessment

### Processing Times:
- CSV Upload & Parse: < 1 second
- Stage 1 + 2: 2-5 seconds
- Stage 3 (Gemini LLM): 10-30 seconds (depends on API)
- Email Sending: 1-3 seconds

---

## Troubleshooting

### If you get 0 matches:
1. Check server logs for parse errors
2. Verify column names are recognized
3. Ensure database has loan products (check `loan_products` table)
4. Confirm n8n workflows are **activated** (not just saved)

### If parsing fails:
1. Check for missing required columns
2. Verify numeric values (no commas, proper format)
3. Look for special characters in email addresses
4. Check age/credit score ranges (age: 18-100, credit: 300-900)

### If no email received:
1. Check AWS SES configuration
2. Verify email is verified in SES (sandbox mode)
3. Check n8n workflow C is activated
4. Look for errors in n8n execution logs

---

## Success Criteria

✅ **CSV Upload**: All files parse with expected user counts  
✅ **Matching**: Match counts vary based on user profiles  
✅ **Optimization**: 3-stage pipeline completes successfully  
✅ **LLM Integration**: Gemini provides relevant qualitative assessments  
✅ **Notifications**: Email received with personalized loan recommendations  
✅ **Data Isolation**: Clearing data resets system completely  

---

## Quick Reference Commands

```bash
# Start server
go run cmd/server/main.go

# Check server is running
curl http://localhost:8080/health

# Clear data via API
curl -X POST http://localhost:8080/api/clear-data

# Check n8n workflows
docker ps | findstr n8n
```

Happy Testing! 🚀
