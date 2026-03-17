#!/usr/bin/env bash
# teardown-ec2.sh — Terminate EC2 instance and clean up AWS resources.
#
# Reads state from ec2-instance.json (created by launch-ec2.sh).
#
# Usage:
#   ./scripts/teardown-ec2.sh              # terminate instance only
#   ./scripts/teardown-ec2.sh --full       # also delete key pair + security group

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STATE_FILE="$PROJECT_ROOT/ec2-instance.json"
FULL_CLEANUP=false

if [ "${1:-}" = "--full" ]; then
  FULL_CLEANUP=true
fi

if [ ! -f "$STATE_FILE" ]; then
  echo "No state file found at $STATE_FILE. Nothing to tear down."
  exit 0
fi

INSTANCE_ID=$(jq -r '.instance_id' "$STATE_FILE")
REGION=$(jq -r '.region' "$STATE_FILE")
SG_ID=$(jq -r '.security_group_id' "$STATE_FILE")
KEY_NAME=$(jq -r '.key_name' "$STATE_FILE")
KEY_FILE=$(jq -r '.key_file' "$STATE_FILE")

echo "==> Terminating instance: $INSTANCE_ID"
aws ec2 terminate-instances \
  --instance-ids "$INSTANCE_ID" \
  --region "$REGION" \
  --query 'TerminatingInstances[0].CurrentState.Name' \
  --output text

echo "==> Waiting for termination..."
aws ec2 wait instance-terminated \
  --instance-ids "$INSTANCE_ID" \
  --region "$REGION"
echo "    Instance terminated."

if [ "$FULL_CLEANUP" = true ]; then
  echo "==> Deleting security group: $SG_ID"
  aws ec2 delete-security-group --group-id "$SG_ID" --region "$REGION" 2>/dev/null || \
    echo "    (already deleted or in use)"

  echo "==> Deleting key pair: $KEY_NAME"
  aws ec2 delete-key-pair --key-name "$KEY_NAME" --region "$REGION" 2>/dev/null || true

  if [ -f "$KEY_FILE" ]; then
    rm -f "$KEY_FILE"
    echo "    Deleted local key file: $KEY_FILE"
  fi
fi

rm -f "$STATE_FILE"
echo ""
echo "==> Teardown complete."
