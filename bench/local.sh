#!/usr/bin/env bash
# bench/local.sh: a quick run of the matrix on this machine, for development.
#
# There is one runner, ../reproduce.sh, and this is it with the settings of a
# development run: one timing pass, no fetch, no ablations, the colbin of
# go.work when there is one, and output under results/local/. Every other
# argument goes through unchanged, and one given here wins over a default:
#
#   bench/local.sh --regime web --cases 'publicbi/*'
#   bench/local.sh --all                 the Go matrix, then the browser benchmark
#   bench/local.sh --all --engines chromium
#
# A run made this way is not the paper's: cmd/summary marks it provisional.
# For the paper's numbers, run ../reproduce.sh itself.
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)

defaults=(--out "results/local/$(date +%Y%m%d-%H%M%S)" --passes 1 --skip-fetch --no-ablation)
[ -f "$repo/go.work" ] && defaults+=(--workspace)
arguments=()
for argument in "$@"; do
	case "$argument" in
	--all) arguments+=(--browser) ;; # the name it had before reproduce.sh
	*) arguments+=("$argument") ;;
	esac
done
exec "$repo/reproduce.sh" "${defaults[@]}" "${arguments[@]+"${arguments[@]}"}"
