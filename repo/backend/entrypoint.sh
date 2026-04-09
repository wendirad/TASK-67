#!/bin/sh
set -e

MODE="${1:-server}"
SECRETS_PATH="${SECRETS_PATH:-/run/secrets}"

# 1. Read secrets from files into environment variables
export DB_PASSWORD=$(cat "$SECRETS_PATH/db_password")
export JWT_SECRET=$(cat "$SECRETS_PATH/jwt_secret")
export WECHAT_MERCHANT_KEY=$(cat "$SECRETS_PATH/wechat_merchant_key")
export BACKUP_ENCRYPTION_KEY=$(cat "$SECRETS_PATH/backup_encryption_key")
export DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

if [ "$MODE" = "server" ]; then
  # 2. Run database migrations
  echo "Running migrations..."
  ./server migrate

  # 3. Bootstrap admin user (no-op if admin already exists)
  echo "Bootstrapping admin user..."
  ./server bootstrap-admin

  # 4. Start the API server
  echo "Starting server..."
  exec ./server serve
elif [ "$MODE" = "worker" ]; then
  # Worker only needs secrets and database — no migrations or bootstrap
  echo "Starting worker..."
  exec ./worker
else
  echo "Unknown mode: $MODE. Use 'server' or 'worker'."
  exit 1
fi
