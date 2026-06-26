#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILE_DIR="$ROOT_DIR/scripts/profiles"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/run-profile.sh <profile> [wails-args...]

Profiles:
  laptop-balanced
  laptop-hyper
  talkback-hyper

Examples:
  ./scripts/run-profile.sh laptop-hyper dev
  ./scripts/run-profile.sh talkback-hyper dev
  ./scripts/run-profile.sh laptop-balanced doctor
EOF
}

if [[ ${1:-} == "" || ${1:-} == "-h" || ${1:-} == "--help" ]]; then
  usage
  exit 0
fi

PROFILE_NAME="$1"
shift || true

PROFILE_FILE="$PROFILE_DIR/${PROFILE_NAME}.env"
if [[ ! -f "$PROFILE_FILE" ]]; then
  echo "Unknown profile: $PROFILE_NAME" >&2
  echo "Expected file: $PROFILE_FILE" >&2
  usage
  exit 1
fi

# shellcheck disable=SC1090
source "$PROFILE_FILE"

if [[ $# -eq 0 ]]; then
  set -- dev
fi

echo "Loaded profile: $PROFILE_NAME" >&2
echo "Running: ./scripts/wails.sh $*" >&2

exec "$ROOT_DIR/scripts/wails.sh" "$@"
