#!/usr/bin/env bash
# Sweep every local repository with dependabot-auto-configure (check mode)
# and emit one CSV row per repo: repo,exit_code,changed,findings,unsafe.
#
# Usage: scripts/sweep.sh [projects-dir] [out.csv]
#   projects-dir defaults to ~/projects; directories whose names start with
#   "archived" are skipped, as are non-git directories.
#
# Exit code 0 when every run completed (any per-repo verdict is still a
# row); 1 when the sweep itself could not run.

set -euo pipefail

PROJECTS_DIR="${1:-$HOME/projects}"
OUT="${2:-dependabot-sweep.csv}"

BIN="$(mktemp -t dependabot-auto-configure.XXXXXX)"
trap 'rm -f "$BIN"' EXIT

cd "$(dirname "$0")/.."
GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 go build -o "$BIN" ./cmd/dependabot-auto-configure

echo "repo,exit_code,wrote,planned_write,unchanged,findings,unsafe_repair" >"$OUT"

for repo in "$PROJECTS_DIR"/*/; do
	name="$(basename "$repo")"
	case "$name" in archived*) continue ;; esac
	[ -d "$repo/.git" ] || continue

	set +e
	payload="$("$BIN" --root "$repo" --check --json 2>/dev/null)"
	code=$?
	set -e

	if [ -z "$payload" ]; then
		echo "$name,$code,,,run_failed" >>"$OUT"
		continue
	fi

	jq -r --arg repo "$name" --argjson code "$code" \
		'[$repo, $code, .wrote, .planned_write, .unchanged,
		  (.findings | length), .unsafe_repair] | @csv' \
		<<<"$payload" >>"$OUT"
done

echo "wrote $OUT"
