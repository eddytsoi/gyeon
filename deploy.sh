#!/usr/bin/env bash
# =============================================================================
# Gyeon — Deploy / Update Script
# Run this on the GCP VM to pull pre-built images and (re)start all services.
# Images are built in GitHub Actions and pushed to GHCR — this script never
# builds anything on the VM. See DEPLOY.md for the full flow.
#
# Usage:
#   bash deploy.sh                  # pull :latest and restart
#   IMAGE_TAG=v0.9.67 bash deploy.sh  # pin to a specific version (rollback)
# =============================================================================
set -euo pipefail

cd /opt/gyeon

echo "📦  Pulling latest code from GitHub..."
git pull origin main

echo "🐳  Pulling images from GHCR..."
docker compose -f docker-compose.prod.yml --env-file .env pull

echo "🚀  Starting containers..."
docker compose -f docker-compose.prod.yml --env-file .env up -d

echo "🧹  Reclaiming old image layers..."
docker image prune -f

echo ""
echo "✅  Deploy complete!"
echo "   Services:"
docker compose -f docker-compose.prod.yml ps
