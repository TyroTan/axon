#!/usr/bin/env bash
# dev.sh — build UI + start Axon server in one command
# Usage: ./dev.sh [port]
#
# Environment overrides (all optional):
#   AXON_LEVEL_OVERRIDE   Difficulty level for question generation.
#                         Values (easiest→hardest):
#                           recall | easy | default | medium | challenge | intense | extreme
#                         Unset = "default" (pure adaptive, no override).
#                         Combined with per-track learner signal via simple average on the
#                         spine (-3..+5). Example: AXON_LEVEL_OVERRIDE=medium ./dev.sh
#
#   AXON_CONTEXT_LIMIT    Max inherited context tokens per session (default: 50000)
#   AXON_SOFT_LIMIT       Token count that triggers a split plan (default: 250000)
#   AXON_HARD_LIMIT       Token count that aborts generation entirely (default: 300000)
#   AXON_QUESTION_COUNT   Target questions per session (default: 8). Generator over-produces
#                         by ~25% and deduplicates down to this count. Job-post questions
#                         are generated in a separate call and merged before dedup.
#   ANTHROPIC_API_KEY     If set, uses Anthropic HTTP API instead of claude CLI
AXON_LEVEL_OVERRIDE="medium"
AXON_ELABORATION_RATE="medium"
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PORT="${1:-3456}"

echo "→ building UI…"
cd "$SCRIPT_DIR/ui"
npm run build

echo "→ starting server on :$PORT"
cd "$SCRIPT_DIR"
AXON_LEVEL_OVERRIDE="$AXON_LEVEL_OVERRIDE" AXON_DIR="$SCRIPT_DIR" PORT="$PORT" go run .
