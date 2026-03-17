#!/usr/bin/env bash
# launch-ec2.sh — End-to-end: launch EC2, deploy backend, start services.
#
# Prerequisites:
#   - AWS CLI configured with valid credentials (aws sts get-caller-identity)
#   - .env.prod exists in project root with real credentials
#   - GitHub repo is public (or EC2 has deploy key — script uses HTTPS clone)
#
# Usage:
#   ./scripts/launch-ec2.sh                  # uses defaults
#   AWS_REGION=ap-southeast-1 ./scripts/launch-ec2.sh  # override region
#
# What it does:
#   1. Creates a key pair (smart-city-key.pem)
#   2. Creates a security group (ports 22, 8080)
#   3. Launches a t2.micro with Amazon Linux 2023
#   4. Waits for the instance to be ready
#   5. SCPs .env.prod to the instance
#   6. SSHs in, runs setup-ec2.sh, starts services
#   7. Verifies health endpoint
#
# Tear down:
#   ./scripts/teardown-ec2.sh

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REGION="${AWS_REGION:-ap-south-1}"
INSTANCE_TYPE="t2.micro"
KEY_NAME="smart-city-key"
KEY_FILE="$PROJECT_ROOT/$KEY_NAME.pem"
SG_NAME="smart-city-sg"
INSTANCE_TAG="smart-city-monitor"
STATE_FILE="$PROJECT_ROOT/ec2-instance.json"

echo "==> Smart City Monitor — EC2 Deployment"
echo "    Region: $REGION"
echo "    Instance type: $INSTANCE_TYPE"
echo ""

# ── Preflight checks ────────────────────────────────────────────────────────
if ! aws sts get-caller-identity &>/dev/null; then
  echo "ERROR: AWS CLI is not authenticated. Run 'aws configure' first."
  exit 1
fi

if [ ! -f "$PROJECT_ROOT/.env.prod" ]; then
  echo "ERROR: .env.prod not found in project root."
  echo "       Run: cp .env.prod.example .env.prod && edit it"
  exit 1
fi

# ── Step 1: Create key pair ──────────────────────────────────────────────────
if [ -f "$KEY_FILE" ]; then
  echo "==> Key pair file already exists: $KEY_FILE"
else
  echo "==> Creating key pair: $KEY_NAME"
  # Delete remote key if it exists from a previous run
  aws ec2 delete-key-pair --key-name "$KEY_NAME" --region "$REGION" 2>/dev/null || true
  aws ec2 create-key-pair \
    --key-name "$KEY_NAME" \
    --region "$REGION" \
    --query 'KeyMaterial' \
    --output text > "$KEY_FILE"
  chmod 400 "$KEY_FILE"
  echo "    Saved to $KEY_FILE"
fi

# ── Step 2: Create security group ────────────────────────────────────────────
# Get default VPC
VPC_ID=$(aws ec2 describe-vpcs \
  --region "$REGION" \
  --filters "Name=isDefault,Values=true" \
  --query 'Vpcs[0].VpcId' \
  --output text)

if [ "$VPC_ID" = "None" ] || [ -z "$VPC_ID" ]; then
  echo "ERROR: No default VPC found in $REGION. Create one or specify a VPC."
  exit 1
fi

# Check if security group already exists
SG_ID=$(aws ec2 describe-security-groups \
  --region "$REGION" \
  --filters "Name=group-name,Values=$SG_NAME" "Name=vpc-id,Values=$VPC_ID" \
  --query 'SecurityGroups[0].GroupId' \
  --output text 2>/dev/null || echo "None")

if [ "$SG_ID" = "None" ] || [ -z "$SG_ID" ]; then
  echo "==> Creating security group: $SG_NAME"
  SG_ID=$(aws ec2 create-security-group \
    --group-name "$SG_NAME" \
    --description "Smart City Monitor - SSH + API" \
    --vpc-id "$VPC_ID" \
    --region "$REGION" \
    --query 'GroupId' \
    --output text)

  # SSH from anywhere
  aws ec2 authorize-security-group-ingress \
    --group-id "$SG_ID" \
    --region "$REGION" \
    --protocol tcp --port 22 --cidr 0.0.0.0/0

  # API gateway
  aws ec2 authorize-security-group-ingress \
    --group-id "$SG_ID" \
    --region "$REGION" \
    --protocol tcp --port 8080 --cidr 0.0.0.0/0

  echo "    Security group created: $SG_ID (ports 22, 8080)"
else
  echo "==> Security group already exists: $SG_ID"
fi

# ── Step 3: Find AMI (Amazon Linux 2023, arm64 for t4g or x86_64 for t2) ────
# t2.micro uses x86_64
ARCH="x86_64"
AMI_ID=$(aws ec2 describe-images \
  --region "$REGION" \
  --owners amazon \
  --filters \
    "Name=name,Values=al2023-ami-2023.*-kernel-*-$ARCH" \
    "Name=state,Values=available" \
  --query 'sort_by(Images, &CreationDate)[-1].ImageId' \
  --output text)

if [ "$AMI_ID" = "None" ] || [ -z "$AMI_ID" ]; then
  echo "ERROR: Could not find Amazon Linux 2023 AMI in $REGION."
  exit 1
fi
echo "==> Using AMI: $AMI_ID (Amazon Linux 2023, $ARCH)"

# ── Step 4: Launch instance ──────────────────────────────────────────────────
# Check if we already have a running instance from a previous launch
if [ -f "$STATE_FILE" ]; then
  EXISTING_ID=$(jq -r '.instance_id' "$STATE_FILE" 2>/dev/null || echo "")
  if [ -n "$EXISTING_ID" ]; then
    EXISTING_STATE=$(aws ec2 describe-instances \
      --instance-ids "$EXISTING_ID" \
      --region "$REGION" \
      --query 'Reservations[0].Instances[0].State.Name' \
      --output text 2>/dev/null || echo "terminated")
    if [ "$EXISTING_STATE" = "running" ]; then
      echo ""
      echo "WARNING: An instance from a previous launch is still running: $EXISTING_ID"
      echo "         Run ./scripts/teardown-ec2.sh first, or delete $STATE_FILE to ignore."
      exit 1
    fi
  fi
fi

echo "==> Launching EC2 instance..."
INSTANCE_ID=$(aws ec2 run-instances \
  --region "$REGION" \
  --image-id "$AMI_ID" \
  --instance-type "$INSTANCE_TYPE" \
  --key-name "$KEY_NAME" \
  --security-group-ids "$SG_ID" \
  --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=$INSTANCE_TAG}]" \
  --query 'Instances[0].InstanceId' \
  --output text)

echo "    Instance ID: $INSTANCE_ID"

# ── Step 5: Wait for instance to be running ──────────────────────────────────
echo "==> Waiting for instance to be running..."
aws ec2 wait instance-running \
  --instance-ids "$INSTANCE_ID" \
  --region "$REGION"

PUBLIC_IP=$(aws ec2 describe-instances \
  --instance-ids "$INSTANCE_ID" \
  --region "$REGION" \
  --query 'Reservations[0].Instances[0].PublicIpAddress' \
  --output text)

echo "    Public IP: $PUBLIC_IP"

# Save state for teardown
cat > "$STATE_FILE" <<EOF
{
  "instance_id": "$INSTANCE_ID",
  "public_ip": "$PUBLIC_IP",
  "region": "$REGION",
  "key_file": "$KEY_FILE",
  "security_group_id": "$SG_ID",
  "key_name": "$KEY_NAME"
}
EOF
echo "    State saved to $STATE_FILE"

# ── Step 6: Wait for SSH to be ready ─────────────────────────────────────────
echo "==> Waiting for SSH to be ready (this can take 30-60s)..."
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 -o LogLevel=ERROR"
MAX_ATTEMPTS=30
for i in $(seq 1 $MAX_ATTEMPTS); do
  if ssh $SSH_OPTS -i "$KEY_FILE" ec2-user@"$PUBLIC_IP" "echo ok" &>/dev/null; then
    echo "    SSH is ready."
    break
  fi
  if [ "$i" -eq "$MAX_ATTEMPTS" ]; then
    echo "ERROR: SSH not ready after ${MAX_ATTEMPTS} attempts. Check security group and instance."
    exit 1
  fi
  sleep 5
done

# ── Step 7: SCP files and run setup ──────────────────────────────────────────
echo "==> Copying .env.prod to instance..."
scp $SSH_OPTS -i "$KEY_FILE" "$PROJECT_ROOT/.env.prod" ec2-user@"$PUBLIC_IP":~/

echo "==> Running setup script on instance..."
ssh $SSH_OPTS -i "$KEY_FILE" ec2-user@"$PUBLIC_IP" 'bash -s' <<'REMOTE_SCRIPT'
set -euo pipefail

echo "── Installing Docker ──"
sudo dnf install -y docker git
sudo systemctl enable --now docker
sudo usermod -aG docker "$USER"

echo "── Installing Docker Compose plugin ──"
sudo mkdir -p /usr/local/lib/docker/cli-plugins
sudo curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-$(uname -m)" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

echo "── Creating swap (1 GB) ──"
if [ ! -f /swapfile ]; then
  sudo fallocate -l 1G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
  sudo swapon /swapfile
  echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
fi

echo "── Cloning repository ──"
REPO_DIR="$HOME/smart-city-monitor"
if [ -d "$REPO_DIR" ]; then
  git -C "$REPO_DIR" pull
else
  git clone https://github.com/chetansierra/smart-city-monitor.git "$REPO_DIR"
fi

echo "── Moving .env.prod ──"
mv ~/\.env.prod "$REPO_DIR/.env.prod"

echo "── Pulling and starting services (pre-built images from Docker Hub) ──"
cd "$REPO_DIR"
sudo docker compose -f docker-compose.prod.yml pull
sudo docker compose -f docker-compose.prod.yml up -d

echo "── Waiting for services to start ──"
sleep 30
sudo docker compose -f docker-compose.prod.yml ps
REMOTE_SCRIPT

# ── Step 8: Health check ─────────────────────────────────────────────────────
echo ""
echo "==> Waiting for API to be ready..."
sleep 10
MAX_HEALTH=12
for i in $(seq 1 $MAX_HEALTH); do
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://$PUBLIC_IP:8080/health" 2>/dev/null || echo "000")
  if [ "$HTTP_CODE" = "200" ]; then
    echo "    API is healthy!"
    break
  fi
  if [ "$i" -eq "$MAX_HEALTH" ]; then
    echo "    WARNING: Health check not passing yet (HTTP $HTTP_CODE)."
    echo "    Services may still be starting. Check manually:"
    echo "      ssh -i $KEY_FILE ec2-user@$PUBLIC_IP"
    echo "      sudo docker compose -f docker-compose.prod.yml logs -f"
    break
  fi
  echo "    Attempt $i/$MAX_HEALTH — HTTP $HTTP_CODE, retrying in 10s..."
  sleep 10
done

# ── Done ─────────────────────────────────────────────────────────────────────
echo ""
echo "============================================================"
echo "  Deployment complete!"
echo ""
echo "  Instance:  $INSTANCE_ID"
echo "  Public IP: $PUBLIC_IP"
echo "  API:       http://$PUBLIC_IP:8080/api/v1"
echo "  Health:    http://$PUBLIC_IP:8080/health"
echo "  SSH:       ssh -i $KEY_FILE ec2-user@$PUBLIC_IP"
echo ""
echo "  Next steps:"
echo "    1. Update Vercel VITE_API_URL to http://$PUBLIC_IP:8080/api/v1"
echo "    2. Redeploy Vercel: cd frontend && vercel --prod"
echo "============================================================"
