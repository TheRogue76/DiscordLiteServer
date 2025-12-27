#!/bin/bash

# Database migration script for Discord Lite Server
set -e

# Load environment variables from .env file if it exists
if [ -f .env ]; then
    export $(cat .env | grep -v '#' | xargs)
fi

# Set default values if not provided
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-discordlite}
DB_PASSWORD=${DB_PASSWORD:-}
DB_NAME=${DB_NAME:-discordlite_db}

echo "========================================"
echo "Discord Lite Server - Database Migration"
echo "========================================"
echo "Host: $DB_HOST:$DB_PORT"
echo "Database: $DB_NAME"
echo "User: $DB_USER"
echo ""

# Check if PostgreSQL client is installed
if ! command -v psql &> /dev/null; then
    echo "Error: psql (PostgreSQL client) is not installed"
    echo "Please install it using:"
    echo "  macOS: brew install postgresql"
    echo "  Ubuntu/Debian: sudo apt-get install postgresql-client"
    exit 1
fi

# Check database connection
echo "Checking database connection..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT 1;" > /dev/null 2>&1

if [ $? -ne 0 ]; then
    echo "Error: Could not connect to the database"
    echo "Please ensure PostgreSQL is running and credentials are correct"
    exit 1
fi

echo "Database connection successful!"
echo ""

# Run migrations
MIGRATION_FILE="internal/database/migrations/001_initial.sql"

if [ ! -f "$MIGRATION_FILE" ]; then
    echo "Error: Migration file not found: $MIGRATION_FILE"
    exit 1
fi

echo "Running migration: $MIGRATION_FILE"
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f $MIGRATION_FILE

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "Migration completed successfully!"
    echo "========================================"
else
    echo ""
    echo "========================================"
    echo "Migration failed!"
    echo "========================================"
    exit 1
fi
