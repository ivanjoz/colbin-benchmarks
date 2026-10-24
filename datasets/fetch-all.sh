#!/usr/bin/env bash
# Fetches and verifies every dataset. Each fetch.sh is idempotent: a file
# already present is only re-verified.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
for script in "$here"/*/fetch.sh; do
	echo "== ${script#"$here"/}" >&2
	"$script"
done
