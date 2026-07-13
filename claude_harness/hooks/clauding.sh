#!/usr/bin/env bash
# clauding.sh — wrapper for the axon-clauding hook CLI (the §74 dynamic directive injector).
# Builds the binary on first use if missing (.devbin is gitignored), then execs it, passing the
# hook's stdin JSON through. FAIL-SILENT: any failure (no repo root / build error) degrades to a
# clean exit so a hook NEVER blocks the session. Subcommand (inject|tick|compact) is $1.
set -uo pipefail
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
[ -n "$ROOT" ] || exit 0
BIN="$ROOT/.devbin/axon-clauding"
if [ ! -x "$BIN" ]; then
	mkdir -p "$ROOT/.devbin"
	go build -o "$BIN" "$ROOT/cmd/axon-clauding" 2>>"$ROOT/.devbin/clauding_build.log" || exit 0
fi
exec "$BIN" "$@"
