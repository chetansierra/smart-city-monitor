#!/usr/bin/env bash
# setup-ec2.sh — Bootstrap a fresh EC2 instance (Amazon Linux 2023 or Ubuntu 22.04+)
# for running the Smart City Monitor backend services.
#
# Run once after launching the EC2 instance:
#   chmod +x setup-ec2.sh && ./setup-ec2.sh
#
# After this script completes:
#   1. Edit .env.prod with your managed service credentials
#   2. Run: docker compose -f docker-compose.prod.yml up -d --build

set -euo pipefail

REPO_URL="https://github.com/chetansierra/smart-city-monitor.git"
REPO_DIR="$HOME/smart-city-monitor"

echo "==> Detecting OS..."
if [ -f /etc/os-release ]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  OS_ID="$ID"
else
  echo "Cannot detect OS. Exiting."
  exit 1
fi

# ── Install Docker ─────────────────────────────────────────────────────────────
install_docker_amazon_linux() {
  echo "==> Installing Docker on Amazon Linux 2023..."
  sudo dnf install -y docker
  sudo systemctl enable --now docker
  sudo usermod -aG docker "$USER"
}

install_docker_ubuntu() {
  echo "==> Installing Docker on Ubuntu..."
  sudo apt-get update -y
  sudo apt-get install -y ca-certificates curl gnupg lsb-release
  sudo install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  sudo chmod a+r /etc/apt/keyrings/docker.gpg
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | \
    sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  sudo apt-get update -y
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
  sudo systemctl enable --now docker
  sudo usermod -aG docker "$USER"
}

case "$OS_ID" in
  amzn)   install_docker_amazon_linux ;;
  ubuntu) install_docker_ubuntu ;;
  *)
    echo "Unsupported OS: $OS_ID. Install Docker manually and re-run."
    exit 1
    ;;
esac

echo "==> Docker installed: $(docker --version)"

# ── Install git ────────────────────────────────────────────────────────────────
if ! command -v git &>/dev/null; then
  echo "==> Installing git..."
  case "$OS_ID" in
    amzn)   sudo dnf install -y git ;;
    ubuntu) sudo apt-get install -y git ;;
  esac
fi

# ── Clone repository ───────────────────────────────────────────────────────────
if [ -d "$REPO_DIR" ]; then
  echo "==> Repository already exists at $REPO_DIR. Pulling latest..."
  git -C "$REPO_DIR" pull
else
  echo "==> Cloning repository..."
  echo "    Make sure REPO_URL at the top of this script is correct."
  git clone "$REPO_URL" "$REPO_DIR"
fi

cd "$REPO_DIR"

# ── Set up production env ──────────────────────────────────────────────────────
if [ ! -f ".env.prod" ]; then
  echo ""
  echo "==> Creating .env.prod from template..."
  cp .env.prod.example .env.prod
  echo ""
  echo "  ┌─────────────────────────────────────────────────────────────────┐"
  echo "  │  ACTION REQUIRED: Edit .env.prod with your managed service      │"
  echo "  │  credentials before starting the services.                      │"
  echo "  │                                                                 │"
  echo "  │  nano .env.prod                                                 │"
  echo "  │                                                                 │"
  echo "  │  Required values to fill in:                                    │"
  echo "  │    POSTGRES_HOST, POSTGRES_USER, POSTGRES_PASSWORD             │"
  echo "  │    REDIS_ADDR, REDIS_PASSWORD                                   │"
  echo "  │    ALLOWED_ORIGINS (your Vercel URL)                            │"
  echo "  └─────────────────────────────────────────────────────────────────┘"
  echo ""
  echo "  After editing, run:"
  echo "    cd $REPO_DIR"
  echo "    docker compose -f docker-compose.prod.yml up -d --build"
  echo ""
else
  echo "==> .env.prod already exists. Skipping template copy."
fi

# ── Add swap file (recommended on t2.micro / 1 GB RAM) ────────────────────────
# Kafka's JVM can occasionally spike; swap prevents OOM kills.
if [ ! -f /swapfile ]; then
  echo "==> Creating 1 GB swap file..."
  sudo fallocate -l 1G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
  sudo swapon /swapfile
  # Make swap persistent across reboots
  echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
  echo "==> Swap enabled."
else
  echo "==> Swap already exists. Skipping."
fi

# ── Open firewall port 8080 ────────────────────────────────────────────────────
# AWS Security Groups control inbound traffic — open port 8080 in your
# EC2 Security Group in the AWS Console (inbound rule: TCP 8080 from 0.0.0.0/0).
# The ufw commands below are for OS-level firewall (usually not needed on EC2).
if command -v ufw &>/dev/null; then
  echo "==> Opening port 8080 in ufw..."
  sudo ufw allow 8080/tcp || true
fi

echo ""
echo "==> Setup complete."
echo ""
echo "Next steps:"
echo "  1. Edit $REPO_DIR/.env.prod — only two things to fill in:"
echo "       POSTGRES_HOST / USER / PASSWORD  (from Neon dashboard)"
echo "       ALLOWED_ORIGINS                  (your Vercel URL)"
echo "  2. cd $REPO_DIR"
echo "  3. docker compose -f docker-compose.prod.yml up -d --build"
echo "  4. docker compose -f docker-compose.prod.yml ps   # all containers should be healthy"
echo "  5. curl http://localhost:8080/health              # verify API is up"
echo ""
echo "Note: You may need to log out and back in for the docker group to take effect,"
echo "      or run 'newgrp docker' to apply it in the current session."
