# 🚀 Deployment Guide

> **Complete instructions for deploying the Loan Eligibility Engine to AWS and configuring all services**

---

## 📋 Table of Contents

- [Prerequisites](#prerequisites)
- [AWS Account Setup](#aws-account-setup)
- [Database Deployment (RDS)](#database-deployment-rds)
- [Email Service Setup (SES)](#email-service-setup-ses)
- [n8n Configuration](#n8n-configuration)
- [Go Server Deployment](#go-server-deployment)
- [Environment Variables](#environment-variables)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## ✅ Prerequisites

Before starting, ensure you have:

- **AWS Account** with billing enabled
- **Docker & Docker Compose** installed
- **Go 1.21+** installed
- **PostgreSQL client** (psql) installed
- **Git** installed
- **AWS CLI** configured (optional but recommended)
- **Domain** or **Verified Email** for SES

**Estimated Setup Time**: 30-45 minutes

---

## 🔐 AWS Account Setup

### 1. Create AWS Account
1. Go to [aws.amazon.com](https://aws.amazon.com)
2. Click "Create an AWS Account"
3. Follow registration steps
4. Add payment method

### 2. Install AWS CLI
```bash
# macOS
brew install awscli

# Windows
choco install awscli

# Linux
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install
```

### 3. Create IAM User for Deployment
```bash
# In AWS Console:
1. Go to IAM > Users > Create User
2. User name: loan-engine-deploy
3. Attach policies:
   - AmazonRDSFullAccess
   - AmazonSESFullAccess
   - AmazonS3FullAccess (if using S3)
   - IAMReadOnlyAccess
4. Create Access Key > Download credentials
```

### 4. Configure AWS CLI
```bash
aws configure
# Enter:
# AWS Access Key ID: <your-access-key>
# AWS Secret Access Key: <your-secret-key>
# Default region name: ap-south-1  (or your preferred region)
# Default output format: json
```

---

## 🗄️ Database Deployment (RDS)

### Option A: AWS RDS (Production)

#### 1. Create RDS PostgreSQL Instance
```bash
# Via AWS Console:
1. Go to RDS > Databases > Create Database
2. Choose: PostgreSQL
3. Templates: Free tier (for testing) or Production
4. Settings:
   - DB instance identifier: loan-eligibility-db
   - Master username: postgres
   - Master password: <create-strong-password>
5. DB instance class: db.t3.micro (free tier) or db.t3.medium
6. Storage: 20 GB GP2
7. Connectivity:
   - Public access: Yes (for initial setup)
   - VPC security group: Create new (loan-db-sg)
   - Port: 5432
8. Create database
```

#### 2. Configure Security Group
```bash
# In AWS Console:
1. Go to EC2 > Security Groups
2. Find loan-db-sg
3. Inbound rules > Edit > Add rule:
   - Type: PostgreSQL
   - Protocol: TCP
   - Port: 5432
   - Source: My IP (your current IP)
   - Description: Dev access
4. Add another rule:
   - Source: <your-server-IP> (if deploying Go server on EC2)
5. Save rules
```

#### 3. Get RDS Endpoint
```bash
# In AWS Console:
1. Go to RDS > Databases > loan-eligibility-db
2. Copy "Endpoint" (e.g., loan-eligibility-db.abc123.ap-south-1.rds.amazonaws.com)
3. Note down:
   - Endpoint: <endpoint>
   - Port: 5432
   - Username: postgres
   - Password: <your-password>
   - Database: postgres (default, we'll create our schema)
```

#### 4. Initialize Database Schema
```bash
# Connect to RDS from your local machine
export DB_HOST=<rds-endpoint>
export DB_PASSWORD=<your-password>

psql -h $DB_HOST -U postgres -d postgres -c "CREATE DATABASE loan_eligibility;"

psql -h $DB_HOST -U postgres -d loan_eligibility -f scripts/init_database.sql
```

#### 5. Verify Tables Created
```bash
psql -h $DB_HOST -U postgres -d loan_eligibility -c "\dt"

# Expected output:
#           List of relations
#  Schema |      Name       | Type  |  Owner
# --------+-----------------+-------+----------
#  public | loan_products   | table | postgres
#  public | matches         | table | postgres
#  public | notifications   | table | postgres
#  public | users           | table | postgres
```

### Option B: Local PostgreSQL (Development)

```bash
# Using Docker
docker run -d \
  --name postgres \
  -e POSTGRES_DB=loan_eligibility \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:13

# Initialize schema
psql -h localhost -U postgres -d loan_eligibility -f scripts/init_database.sql
```

---

## 📧 Email Service Setup (SES)

### 1. Verify Sender Email
```bash
# In AWS Console:
1. Go to Amazon SES (Simple Email Service)
2. Select your region (ap-south-1 recommended)
3. Verified identities > Create identity
4. Identity type: Email address
5. Email address: kaustubhbiswas001@gmail.com (or your email)
6. Create identity
7. Check your inbox > Click verification link
8. Status should change to "Verified"
```

### 2. Move Out of Sandbox (For Production)
```bash
# In AWS Console:
1. Go to SES > Account dashboard
2. Click "Request production access"
3. Fill form:
   - Mail type: Transactional
   - Website URL: (your domain if available)
   - Use case description:
     "Sending loan eligibility notifications to users who uploaded
     their profiles. Emails contain personalized loan recommendations."
4. Submit request
5. Wait for approval (usually 24-48 hours)

# Until approved, you can only send to verified emails
```

### 3. Create SES SMTP Credentials
```bash
# In AWS Console:
1. Go to SES > SMTP settings
2. Create My SMTP Credentials
3. IAM User Name: ses-smtp-user
4. Create user
5. Download credentials:
   - SMTP Username: <smtp-username>
   - SMTP Password: <smtp-password>
6. Note down:
   - SMTP endpoint: email-smtp.ap-south-1.amazonaws.com
   - Port: 587 (TLS)
```

### 4. Get SES API Credentials
```bash
# Use the IAM user created earlier (loan-engine-deploy)
# Or create specific SES user:

1. Go to IAM > Users > Create user
2. User name: ses-api-user
3. Attach policy: AmazonSESFullAccess
4. Create access key > Application running outside AWS
5. Download credentials:
   - Access Key ID: <ses-access-key>
   - Secret Access Key: <ses-secret-key>
```

---

## 🔄 n8n Configuration

### 1. Start n8n Container
```bash
cd "ClickPe Task"
docker-compose up -d

# Verify running
docker ps | grep n8n

# Check logs
docker logs loan-n8n
```

### 2. Access n8n UI
```bash
# Open browser:
http://localhost:5678

# First-time setup:
1. Create owner account:
   - Email: your-email@example.com
   - Password: <strong-password>
2. Click "Get Started"
```

### 3. Import Workflows
```bash
# In n8n UI:
1. Click "Workflows" > "Import from File"
2. Import each workflow:
   - n8n/workflows/workflow_a_loan_crawler.json
   - n8n/workflows/workflow_b_user_matching.json
   - n8n/workflows/workflow_c_notification.json
3. Each workflow should appear in list
```

### 4. Configure AWS SES Credentials in n8n
```bash
# For Workflow C (Notification):

1. Open "Workflow C - User Notification"
2. Click on "Send Email via SES" node
3. Click "Credential to connect with"
4. Click "Create New Credential"
5. Select "AWS" credential type
6. Fill in:
   - Credential Name: AWS SES
   - Access Key ID: <ses-access-key>
   - Secret Access Key: <ses-secret-key>
   - Region: ap-south-1
7. Save credentials
8. Node should now show green checkmark
```

### 5. Configure Gemini API Key
```bash
# For Workflow B (Matching):

# Method 1: n8n Environment Variable
1. Stop n8n: docker-compose down
2. Edit docker-compose.yml:
   environment:
     - GEMINI_API_KEY=AIzaSyDmqrbK6B-9p2UODsKxuGAtv-vefYk_xrs
3. Restart: docker-compose up -d

# Method 2: Hardcode in Workflow (not recommended)
1. Open "Workflow B - User Matching"
2. Find "LLM Qualitative Check" HTTP node
3. Edit Headers:
   - Name: x-goog-api-key
   - Value: AIzaSyDmqrbK6B-9p2UODsKxuGAtv-vefYk_xrs
```

### 6. Activate Workflows
```bash
# In n8n UI:
1. Open "Workflow B - User Matching"
2. Toggle "Active" switch (top right) to ON
3. Webhook URL appears: http://localhost:5678/webhook/match-users

4. Open "Workflow C - User Notification"
5. Toggle "Active" to ON
6. Webhook URL: http://localhost:5678/webhook/notify-user

# Workflow A (Crawler) - Optional, activate when needed
```

### 7. Test Webhooks
```bash
# Test Workflow C:
curl -X POST http://localhost:5678/webhook/notify-user \
  -H "Content-Type: application/json" \
  -d '{
    "user_email": "kaustubhbiswas001@gmail.com",
    "user_name": "Test User",
    "match_id": "test-123",
    "matched_products": [{
      "product_name": "Test Loan",
      "provider": "Test Bank",
      "interest_rate": 10.5,
      "min_amount": 100000,
      "max_amount": 1000000,
      "match_score": 95
    }]
  }'

# Expected: Email sent, check inbox
```

---

## 🖥️ Go Server Deployment

### Option A: Local Development
```bash
# 1. Set environment variables
export DB_HOST=<rds-endpoint>  # or localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=<your-password>
export DB_NAME=loan_eligibility
export N8N_WEBHOOK_URL=http://localhost:5678

# 2. Run server
cd "ClickPe Task"
go run cmd/server/main.go

# Expected output:
# 2025/12/07 23:15:01 🚀 Loan Eligibility Engine API Server
# 2025/12/07 23:15:01 📡 Listening on http://localhost:8080
```

### Option B: Build Binary
```bash
# Build
go build -o loan-engine cmd/server/main.go

# Run
./loan-engine

# For Windows:
go build -o loan-engine.exe cmd/server/main.go
loan-engine.exe
```

### Option C: Deploy to AWS EC2
```bash
# 1. Launch EC2 instance
# In AWS Console:
# - AMI: Amazon Linux 2023 or Ubuntu 22.04
# - Instance type: t2.micro (free tier)
# - Security group: Allow TCP 8080, 22

# 2. SSH into instance
ssh -i your-key.pem ec2-user@<instance-ip>

# 3. Install Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 4. Clone repo & build
git clone <your-repo>
cd "ClickPe Task"
go build -o loan-engine cmd/server/main.go

# 5. Create systemd service
sudo tee /etc/systemd/system/loan-engine.service > /dev/null <<EOF
[Unit]
Description=Loan Eligibility Engine
After=network.target

[Service]
Type=simple
User=ec2-user
WorkingDirectory=/home/ec2-user/ClickPe\ Task
Environment="DB_HOST=<rds-endpoint>"
Environment="DB_PORT=5432"
Environment="DB_USER=postgres"
Environment="DB_PASSWORD=<password>"
Environment="DB_NAME=loan_eligibility"
Environment="N8N_WEBHOOK_URL=http://localhost:5678"
ExecStart=/home/ec2-user/ClickPe\ Task/loan-engine
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# 6. Start service
sudo systemctl daemon-reload
sudo systemctl start loan-engine
sudo systemctl enable loan-engine

# 7. Check status
sudo systemctl status loan-engine
```

---

## 🔑 Environment Variables

### Complete List
```bash
# Database (Required)
DB_HOST=your-rds-endpoint.ap-south-1.rds.amazonaws.com
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your-strong-password
DB_NAME=loan_eligibility

# n8n (Required)
N8N_WEBHOOK_URL=http://localhost:5678

# AWS SES (Configured in n8n, not in Go server)
# These are set in n8n credential manager
AWS_SES_ACCESS_KEY_ID=your-ses-access-key
AWS_SES_SECRET_ACCESS_KEY=your-ses-secret-key
AWS_REGION=ap-south-1

# Gemini API (Set in n8n environment)
GEMINI_API_KEY=AIzaSyDmqrbK6B-9p2UODsKxuGAtv-vefYk_xrs

# Server (Optional)
PORT=8080  # Default
CORS_ALLOWED_ORIGINS=http://localhost:8080,http://localhost:5678
```

### Setting Variables on Different Platforms

#### Linux/macOS
```bash
# Temporary (current session)
export DB_HOST=your-endpoint

# Permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export DB_HOST=your-endpoint' >> ~/.bashrc
source ~/.bashrc
```

#### Windows CMD
```cmd
set DB_HOST=your-endpoint
```

#### Windows PowerShell
```powershell
$env:DB_HOST="your-endpoint"

# Permanent
[Environment]::SetEnvironmentVariable("DB_HOST", "your-endpoint", "User")
```

#### Docker Compose (for n8n)
```yaml
# docker-compose.yml
services:
  n8n:
    environment:
      - GEMINI_API_KEY=your-api-key
      - DB_TYPE=postgresdb
      - DB_POSTGRESDB_HOST=your-rds-endpoint
```

---

## ✅ Verification

### 1. Check Database Connection
```bash
# From local machine
psql -h $DB_HOST -U postgres -d loan_eligibility -c "SELECT COUNT(*) FROM loan_products;"

# Expected: 5 (or number of seeded products)
```

### 2. Check n8n Webhooks
```bash
# Test Workflow B
curl -X POST http://localhost:5678/webhook/match-users \
  -H "Content-Type: application/json" \
  -d '{}'

# Expected: JSON response with matches

# Test Workflow C
curl -X POST http://localhost:5678/webhook/notify-user \
  -H "Content-Type: application/json" \
  -d '{"user_email":"kaustubhbiswas001@gmail.com","user_name":"Test","matched_products":[]}'

# Expected: Email error (no products) but webhook responds
```

### 3. Check Go Server
```bash
# Health check
curl http://localhost:8080/health

# Expected:
# {"status":"healthy","timestamp":"...","version":"1.0.0","database":"connected"}

# API endpoints
curl http://localhost:8080/api/users
curl http://localhost:8080/api/loan-products
curl http://localhost:8080/api/matches
```

### 4. Test Complete Flow
```bash
# 1. Open dashboard
open http://localhost:8080

# 2. Upload test CSV
# Click "Upload CSV" > Select data/test_high_income_users.csv > Upload

# 3. Trigger matching
# Click "Trigger Matching Workflow" button

# 4. Check matches
# Refresh page > "Matches Found" count should increase

# 5. Send notification
# Click "Send Email Notification" > Enter email > Send

# 6. Check inbox
# Email should arrive with loan recommendations
```

---

## 🐛 Troubleshooting

### Database Connection Failed
```bash
# Check security group allows your IP
# Update security group to allow your current IP:
curl ifconfig.me  # Get your IP
# Add to RDS security group inbound rules

# Test connection
telnet $DB_HOST 5432
# Should connect, press Ctrl+C to exit

# Check credentials
psql -h $DB_HOST -U postgres -d postgres
# If fails, verify username/password
```

### n8n Webhook 404
```bash
# Check if workflow is active
# In n8n UI: Workflow must have green "Active" toggle

# Check Docker container
docker ps | grep n8n
docker logs loan-n8n --tail 50

# Restart n8n
docker-compose restart
```

### Email Not Sending
```bash
# 1. Verify SES credentials in n8n
# Click node > Check credentials show green

# 2. Check SES sandbox status
# If in sandbox, recipient must be verified

# 3. Verify sender email
# Must be verified in SES

# 4. Check n8n execution logs
# Click "Executions" tab in n8n
# Find failed execution > View details

# 5. Test SES directly
aws ses send-email \
  --from kaustubhbiswas001@gmail.com \
  --destination ToAddresses=kaustubhbiswas001@gmail.com \
  --message Subject={Data="Test"},Body={Text={Data="Test email"}}
```

### Go Server Crashes
```bash
# Check logs
# Server prints detailed error logs

# Common issues:
# 1. Database connection: Check DB_HOST, DB_PASSWORD
# 2. Port in use: Change PORT env variable
# 3. Missing logger: Ensure utils.InitLogger() called

# Debug mode
go run -race cmd/server/main.go  # Detects race conditions
```

### Gemini API Errors
```bash
# Check API key
echo $GEMINI_API_KEY

# Test API directly
curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=$GEMINI_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"contents":[{"parts":[{"text":"Hello"}]}]}'

# Expected: JSON response with generated text

# If 403/401: API key invalid
# If 429: Rate limit exceeded
# If 404: Wrong API endpoint or model name
```

---

## 📊 Monitoring (Optional)

### RDS Performance
```bash
# In AWS Console:
1. Go to RDS > Databases > loan-eligibility-db
2. Click "Monitoring" tab
3. View CPU, connections, storage metrics
```

### SES Sending Statistics
```bash
# In AWS Console:
1. Go to SES > Account dashboard
2. View sending statistics:
   - Emails sent
   - Bounces
   - Complaints
```

### n8n Execution History
```bash
# In n8n UI:
1. Click "Executions" in left sidebar
2. View all workflow runs
3. Click any execution to see details
4. Green = Success, Red = Failed
```

---

## 🚀 Next Steps

After successful deployment:

1. **Test with real data**: Upload actual user CSV files
2. **Monitor performance**: Check execution times in n8n
3. **Scale as needed**: 
   - Increase RDS instance size if slow queries
   - Add read replicas for high traffic
   - Deploy Go server on larger EC2 instance
4. **Implement Workflow A**: Add web crawler for live loan products
5. **Add monitoring**: Set up CloudWatch alarms for errors
6. **Backup database**: Enable automated RDS backups

---

## 📞 Support

If you encounter issues:
- Check the main [README.md](../README.md)
- Review [ARCHITECTURE.md](ARCHITECTURE.md) for design details
- Email: kaustubhbiswas001@gmail.com

---

**Deployment Complete! 🎉**
