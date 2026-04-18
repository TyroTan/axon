#!/usr/bin/env bash
# dev.sh — build UI + start Axon server in one command
# Usage: ./dev.sh [port]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PORT="${1:-3456}"

echo "→ building UI…"
cd "$SCRIPT_DIR/ui"
npm run build

echo "→ starting server on :$PORT"
cd "$SCRIPT_DIR"
AXON_DIR="$SCRIPT_DIR" PORT="$PORT" go run .
