#!/usr/bin/env bash
# =============================================================================
# Gyeon — GCP Compute Engine Setup Script
# Usage: ./setup-gcp.sh
# Requires: gcloud CLI installed and authenticated
# =============================================================================
set -euo pipefail

# ── Config (edit these) ──────────────────────────────────────────────────────
GCP_PROJECT=""           # Your GCP project ID, e.g. "my-project-123"
GCP_ZONE="asia-east1-b"  # Hong Kong region
VM_NAME="gyeon"
MACHINE_TYPE="e2-small"  # 2 vCPU, 2 GB RAM (~USD $15/mo)
GITHUB_REPO="https://github.com/eddytsoi/gyeon.git"
# ─────────────────────────────────────────────────────────────────────────────

if [[ -z "$GCP_PROJECT" ]]; then
  echo "❌  Please set GCP_PROJECT at the top of this script before running."
  exit 1
fi

echo "🔧  Setting GCP project to: $GCP_PROJECT"
gcloud config set project "$GCP_PROJECT"

# ── 1. Create VM ──────────────────────────────────────────────────────────────
echo "🖥   Creating Compute Engine VM: $VM_NAME ($MACHINE_TYPE) in $GCP_ZONE..."
gcloud compute instances create "$VM_NAME" \
  --zone="$GCP_ZONE" \
  --machine-type="$MACHINE_TYPE" \
  --image-family=ubuntu-2204-lts \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=20GB \
  --boot-disk-type=pd-standard \
  --tags=http-server,https-server \
  --metadata=enable-oslogin=true

# ── 2. Firewall rules (skip if already exist) ─────────────────────────────────
echo "🔥  Configuring firewall rules..."
gcloud compute firewall-rules create allow-http \
  --allow=tcp:80 \
  --target-tags=http-server \
  --description="Allow HTTP" 2>/dev/null || echo "   allow-http rule already exists, skipping."

gcloud compute firewall-rules create allow-https \
  --allow=tcp:443 \
  --target-tags=https-server \
  --description="Allow HTTPS" 2>/dev/null || echo "   allow-https rule already exists, skipping."

# ── 3. Get external IP ────────────────────────────────────────────────────────
EXTERNAL_IP=$(gcloud compute instances describe "$VM_NAME" \
  --zone="$GCP_ZONE" \
  --format="get(networkInterfaces[0].accessConfigs[0].natIP)")
echo ""
echo "✅  VM created! External IP: $EXTERNAL_IP"
echo ""
echo "📝  Before deploying, edit .env on the VM with your real secrets."
echo "    BASE_URL should be: http://$EXTERNAL_IP"
echo ""

# ── 4. Install Docker + clone repo on VM ──────────────────────────────────────
echo "🐳  Installing Docker and deploying Gyeon on the VM..."
echo "    (this may take a few minutes)"

gcloud compute ssh "$VM_NAME" --zone="$GCP_ZONE" -- bash -s << REMOTE
set -euo pipefail

# Install Docker
if ! command -v docker &>/dev/null; then
  echo "→ Installing Docker..."
  curl -fsSL https://get.docker.com | sudo sh
  sudo usermod -aG docker \$USER
  echo "→ Docker installed."
fi

# Install Docker Compose plugin
if ! docker compose version &>/dev/null 2>&1; then
  sudo apt-get install -y docker-compose-plugin
fi

# Clone repo
if [ ! -d "/opt/gyeon" ]; then
  echo "→ Cloning repository..."
  sudo git clone $GITHUB_REPO /opt/gyeon
  sudo chown -R \$USER:\$USER /opt/gyeon
fi

cd /opt/gyeon

# Create .env from example if not present
if [ ! -f ".env" ]; then
  cp .env.example .env
  # Generate random secrets
  JWT_ADMIN=\$(openssl rand -hex 32)
  JWT_CUSTOMER=\$(openssl rand -hex 32)
  sed -i "s/change-me-admin-jwt-secret/\$JWT_ADMIN/" .env
  sed -i "s/change-me-customer-jwt-secret/\$JWT_CUSTOMER/" .env
  sed -i "s|YOUR_GCP_EXTERNAL_IP|$EXTERNAL_IP|g" .env
  echo ""
  echo "⚠️   .env created at /opt/gyeon/.env"
  echo "    Please SSH in and update DB_PASSWORD, ADMIN_EMAIL, ADMIN_PASSWORD:"
  echo "    gcloud compute ssh $VM_NAME --zone=$GCP_ZONE"
  echo "    nano /opt/gyeon/.env"
fi

echo "→ Done bootstrapping. Run deploy.sh to start Gyeon."
REMOTE

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  Gyeon VM is ready at http://$EXTERNAL_IP"
echo ""
echo "  Next steps:"
echo "  1. SSH into the VM:"
echo "     gcloud compute ssh $VM_NAME --zone=$GCP_ZONE"
echo ""
echo "  2. Edit secrets in /opt/gyeon/.env:"
echo "     nano /opt/gyeon/.env"
echo ""
echo "  3. Run the deploy script:"
echo "     cd /opt/gyeon && bash deploy.sh"
echo "═══════════════════════════════════════════════════════"
