#!/usr/bin/env bash
# =============================================================================
# Gyeon — Deploy / Update Script
# Run this on the GCP VM to build and (re)start all services.
# Usage: bash deploy.sh
# =============================================================================
set -euo pipefail

cd /opt/gyeon

echo "📦  Pulling latest code from GitHub..."
git pull origin main

echo "🐳  Building and starting containers..."
docker compose -f docker-compose.prod.yml --env-file .env up -d --build

echo ""
echo "✅  Deploy complete!"
echo "   Services:"
docker compose -f docker-compose.prod.yml ps
