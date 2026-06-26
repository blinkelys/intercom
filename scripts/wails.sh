#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT_DIR/.tools/bin"
WAILS_BIN="$BIN_DIR/wails"
VERSION="v2.12.0"

mkdir -p "$BIN_DIR"

if [[ ! -x "$WAILS_BIN" ]]; then
  echo "Installing Wails CLI $VERSION into $BIN_DIR" >&2
  GOBIN="$BIN_DIR" go install github.com/wailsapp/wails/v2/cmd/wails@"$VERSION"
fi

cd "$ROOT_DIR"
exec "$WAILS_BIN" "$@"