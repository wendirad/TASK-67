#!/bin/sh
set -e
if [ -f /run/secrets/db_password ]; then
  echo "Secrets already exist, skipping generation."
  exit 0
fi
echo "Generating secrets for first-time startup..."
openssl rand -base64 24 | tr -d '\n' > /run/secrets/db_password
openssl rand -hex 32 > /run/secrets/jwt_secret
openssl rand -hex 32 > /run/secrets/wechat_merchant_key
openssl rand -hex 32 > /run/secrets/backup_encryption_key
openssl rand -base64 18 | tr -d '\n' > /run/secrets/admin_bootstrap_password
chmod 444 /run/secrets/*
echo "Secrets generated."
