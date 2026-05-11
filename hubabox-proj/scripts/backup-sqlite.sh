#!/usr/bin/env bash
# Online backup of hubaBox SQLite while the hub is running (uses sqlite3 .backup).
# If sqlite3 is missing, falls back to file copy (safest when hub is stopped).
set -euo pipefail

if [[ "${1:-}" == "" ]]; then
	echo "usage: $0 DATA_DIR [OUTPUT_DIR]" >&2
	echo "  DATA_DIR     directory containing hubabox.db (same as -data / HUBABOX_DATA)" >&2
	echo "  OUTPUT_DIR   default: DATA_DIR/backups" >&2
	exit 1
fi

DATA="$(cd "$1" && pwd)"
if [[ -n "${2:-}" ]]; then
	mkdir -p "$2"
	OUT="$(cd "$2" && pwd)"
else
	mkdir -p "$DATA/backups"
	OUT="$(cd "$DATA/backups" && pwd)"
fi

DB="$DATA/hubabox.db"
if [[ ! -f "$DB" ]]; then
	echo "error: database not found: $DB" >&2
	exit 1
fi

TS="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="$OUT/hubabox-$TS.db"

if command -v sqlite3 >/dev/null 2>&1; then
	sqlite3 "$DB" ".backup '$DEST'"
	echo "OK: online backup -> $DEST"
else
	cp -a "$DB" "$DEST"
	echo "warning: sqlite3 not in PATH; used plain copy. Stop hubaBox before backup for a crash-consistent file if unsure." >&2
	echo "OK: file copy -> $DEST"
fi
