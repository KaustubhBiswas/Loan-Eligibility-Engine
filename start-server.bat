@echo off
REM Start Loan Eligibility Engine API Server
REM This connects to AWS RDS PostgreSQL

echo Starting Loan Eligibility Engine API Server...
echo.

cd /d "%~dp0"

set DB_HOST=loan-eligibility-db.cpok88yeay22.ap-south-1.rds.amazonaws.com
set DB_PORT=5432
set DB_NAME=loanengine
set DB_USER=postgres
set DB_PASSWORD=loaneligibilitydb
set PORT=8080

echo Database: %DB_HOST%/%DB_NAME%
echo Port: %PORT%
echo.

bin\server.exe

pause
