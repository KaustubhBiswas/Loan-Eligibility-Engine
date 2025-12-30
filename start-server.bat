@echo off
REM Start Loan Eligibility Engine API Server
REM This connects to AWS RDS PostgreSQL

echo Starting Loan Eligibility Engine API Server...
echo.

cd /d "%~dp0"

REM Set environment variables (or use existing ones)
REM To set these permanently, use: setx VARIABLE_NAME "value"
if not defined DB_HOST set DB_HOST=your-db-host.rds.amazonaws.com
if not defined DB_PORT set DB_PORT=5432
if not defined DB_NAME set DB_NAME=loanengine
if not defined DB_USER set DB_USER=postgres
if not defined DB_PASSWORD (
    echo ERROR: DB_PASSWORD environment variable is not set!
    echo Please set it using: setx DB_PASSWORD "your-password"
    pause
    exit /b 1
)
if not defined PORT set PORT=8080

echo Database: %DB_HOST%/%DB_NAME%
echo Port: %PORT%
echo.

bin\server.exe

pause
