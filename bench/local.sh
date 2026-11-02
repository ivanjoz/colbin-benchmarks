#!/usr/bin/env bash
# bench/local.sh: the whole Go matrix on this machine, in one command.
#
#   bench/local.sh                       sizes + timings, web and bulk
#   bench/local.sh --regime web          one regime only
#   bench/local.sh --cases 'publicbi/*'  some cases (comma-separated globs, or "all")
#   bench/local.sh --all                 the Go matrix, then the browser benchmark
#   bench/local.sh --all --engines chromium
#
# Everything measured is pinned to one hardware thread per physical
# performance core (--cores, default 0,2 for the i7-1355U this was written on:
# CPUs 0-3 are its two P cores with hyperthreading, 4-11 its E cores). Web
# cases run two at a time, one per core; bulk cases one at a time, each after a
# check that enough memory is free (tpch nested and normalized need over 5 GB).
#
# The run refuses to start on battery, or with a measured core not on the
# performance governor; --no-checks downgrades that to a warning, which
# machine.txt records, for a smoke test only.
#
# Output goes to results/local/<YYYYMMDD-HHMMSS>/:
#   sizes.csv cases.csv   TestSizes, both regimes (sizes do not depend on the CPU)
#   bench-web.txt         colbinbench, web cases
#   bench-bulk.txt        colbinbench, bulk cases
#   bench-browser-*.txt   with --all: web/run.mjs, one file per engine
#   browser-paths-*.csv   with --all: which path colbin's decode took per case
#   browser-verify.txt    with --all: web/verify.mjs, every format against the Go JSON
#   freq.csv              the measured cores' frequency, once a second, during timings
#   machine.txt           CPU, kernel, versions, commits, busiest processes, frequency summary
set -euo pipefail

command_line="$0 $*"
repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"

regime=all
cases=all
cores=0,2
count=10
sample=100ms
formats=all
ops=encode,decode-fresh,decode-reuse,access
browser=0
engines=chromium,firefox,webkit
checks=1
min_mem_gb=8
out=""

usage() { sed -n '2,/^set -euo/p' "$0" | sed '$d; s/^# \{0,1\}//'; exit "${1:-0}"; }

while [ $# -gt 0 ]; do
	case "$1" in
	--regime) regime=$2; shift 2 ;;
	--cases) cases=$2; shift 2 ;;
	--cores) cores=$2; shift 2 ;;
	--count) count=$2; shift 2 ;;
	--sample) sample=$2; shift 2 ;;
	--formats) formats=$2; shift 2 ;;
	--ops) ops=$2; shift 2 ;;
	--out) out=$2; shift 2 ;;
	--min-mem-gb) min_mem_gb=$2; shift 2 ;;
	--all) browser=1; shift ;;
	--engines) engines=$2; shift 2 ;;
	--no-checks) checks=0; shift ;;
	-h | --help) usage ;;
	*) echo "local.sh: unknown argument $1" >&2; usage 2 ;;
	esac
done
case "$regime" in all | web | bulk) ;; *) echo "local.sh: --regime is web, bulk or all" >&2; exit 2 ;; esac

out=${out:-results/local/$(date +%Y%m%d-%H%M%S)}
mkdir -p "$out"
out=$(cd "$out" && pwd)
data="$repo/datasets/data"
IFS=, read -r -a core_list <<<"$cores"
warnings=()

fail() {
	if [ "$checks" = 1 ]; then
		echo "local.sh: $1" >&2
		echo "local.sh: fix it, or pass --no-checks for a smoke test that is not a measurement" >&2
		exit 1
	fi
	echo "local.sh: warning: $1" >&2
	warnings+=("$1")
}

mem_available_gb() { awk '/^MemAvailable:/ { printf "%.1f", $2 / 1048576 }' /proc/meminfo; }

# ---- 1. checks before anything is measured --------------------------------

supplies=(/sys/class/power_supply/*/online)
if [ -e "${supplies[0]}" ]; then
	on_ac=0
	for supply in "${supplies[@]}"; do [ "$(cat "$supply")" = 1 ] && on_ac=1; done
	[ "$on_ac" = 1 ] || fail "on battery: plug the charger in"
fi
for core in "${core_list[@]}"; do
	governor=/sys/devices/system/cpu/cpu$core/cpufreq/scaling_governor
	if [ -r "$governor" ]; then
		[ "$(cat "$governor")" = performance ] || fail "cpu$core governor is $(cat "$governor"), not performance (sudo cpupower frequency-set -g performance)"
	else
		warnings+=("cpu$core has no cpufreq governor to check")
	fi
done
busiest=$(ps -eo pcpu,pid,comm --sort=-pcpu | head -6)
echo "busiest processes before the run:"
echo "$busiest"
echo "close browsers, IDEs and sync clients before measuring"

# ---- 2. machine.txt -----------------------------------------------------------

commit_of() { # dir → "commit (dirty)" or "commit"
	local dirty=""
	[ -n "$(git -C "$1" status --porcelain 2>/dev/null)" ] && dirty=" (uncommitted changes)"
	echo "$(git -C "$1" rev-parse HEAD)$dirty"
}
colbin_version() {
	if [ -f go.work ] && grep -q '\.\./colbin' go.work; then
		echo "../colbin via go.work at $(commit_of ../colbin), $(git -C ../colbin describe --tags --always 2>/dev/null)"
	else
		go list -m github.com/ivanjoz/colbin
	fi
}
{
	echo "date: $(date -Iseconds)"
	echo "command: $command_line"
	echo "regime: $regime"
	echo "cases: $cases"
	echo "measured cores: $cores"
	echo "count: $count, sample: $sample, formats: $formats, ops: $ops"
	echo "kernel: $(uname -sr)"
	echo "go: $(go version)"
	echo "colbin: $(colbin_version)"
	echo "colbin-benchmarks: $(commit_of .)"
	echo "MemAvailable at start: $(mem_available_gb) GB"
	for warning in "${warnings[@]+"${warnings[@]}"}"; do echo "WARNING: $warning"; done
	[ "$checks" = 1 ] || echo "WARNING: --no-checks: a smoke test, not a measurement"
	echo
	echo "== lscpu"
	lscpu
	echo
	echo "== busiest processes at start"
	echo "$busiest"
} >"$out/machine.txt"

# ---- builds ---------------------------------------------------------------

bin=$(mktemp -d)
sampler=""
cleanup() {
	[ -n "$sampler" ] && kill "$sampler" 2>/dev/null || true
	rm -rf "$bin"
}
trap cleanup EXIT
echo "building colbinbench and the matrix test binary"
go build -o "$bin/colbinbench" ./cmd/colbinbench
go test -c -o "$bin/matrix.test" ./matrix

# Which cases run, and in which regime. Listing loads each case once.
"$bin/colbinbench" -data "$data" -list -cases "$cases" >"$bin/cases.txt"
web_cases=$(awk '$2 == "web" { print $1 }' "$bin/cases.txt" | paste -sd,)
bulk_cases=$(awk '$2 == "bulk" { print $1 }' "$bin/cases.txt")
[ "$regime" = bulk ] && web_cases=""
[ "$regime" = web ] && bulk_cases=""
echo "web cases: $(echo "$web_cases" | tr , '\n' | grep -c . || true), bulk cases: $(echo "$bulk_cases" | grep -c . || true)"

need_memory() { # case
	local free
	free=$(mem_available_gb)
	if awk -v f="$free" -v m="$min_mem_gb" 'BEGIN { exit !(f < m) }'; then
		echo "local.sh: $1: only $free GB available, a bulk case needs $min_mem_gb GB; aborting" >&2
		echo "local.sh: close what holds memory, or run --regime web" >&2
		exit 1
	fi
}

# ---- 3. sizes -----------------------------------------------------------------

# One TestSizes process for the web cases, and one per bulk case after a memory
# check; their CSVs are joined under one header.
sizes_run() { # dir, cases
	mkdir -p "$1"
	(cd matrix && COLBIN_DATA="$data" COLBIN_RESULTS="$1" COLBIN_CASES="$2" \
		"$bin/matrix.test" -test.run '^TestSizes$' -test.timeout 0 >"$1/test.log" 2>&1) || {
		echo "local.sh: TestSizes failed for $2; see $1/test.log" >&2
		tail -20 "$1/test.log" >&2
		return 1
	}
}
parts=()
if [ -n "$web_cases" ]; then
	echo "sizes: web"
	sizes_run "$bin/sizes-web" "$web_cases"
	parts+=("$bin/sizes-web")
fi
index=0
for name in $bulk_cases; do
	need_memory "$name"
	echo "sizes: $name"
	sizes_run "$bin/sizes-bulk-$index" "$name"
	parts+=("$bin/sizes-bulk-$index")
	index=$((index + 1))
done
for table in sizes.csv cases.csv; do
	first=1
	for part in "${parts[@]+"${parts[@]}"}"; do
		[ -f "$part/$table" ] || continue
		if [ "$first" = 1 ]; then cat "$part/$table"; first=0; else tail -n +2 "$part/$table"; fi
	done >"$out/$table"
done
for part in "${parts[@]+"${parts[@]}"}"; do cat "$part/test.log"; done >"$out/sizes-test.log"

# ---- 4-5. timings, with the frequency sampled ----------------------------------

{
	printf 'time'
	for core in "${core_list[@]}"; do printf ',cpu%s_khz' "$core"; done
	echo
} >"$out/freq.csv"
(
	while :; do
		line=$(date +%s)
		for core in "${core_list[@]}"; do
			line="$line,$(cat "/sys/devices/system/cpu/cpu$core/cpufreq/scaling_cur_freq" 2>/dev/null || echo)"
		done
		echo "$line" >>"$out/freq.csv"
		sleep 1
	done
) &
sampler=$!

header() { echo "cpu: $(lscpu | sed -n 's/^Model name: *//p' | head -1)"; }
bench_flags=(-data "$data" -formats "$formats" -ops "$ops" -count "$count" -sample "$sample")
if [ -n "$web_cases" ]; then
	echo "timings: web, ${#core_list[@]} at a time on cores $cores"
	{ header; "$bin/colbinbench" "${bench_flags[@]}" -cases "$web_cases" -cores "$cores"; } >"$out/bench-web.txt"
fi
if [ -n "$bulk_cases" ]; then
	: >"$out/bench-bulk.txt"
	header >>"$out/bench-bulk.txt"
	for name in $bulk_cases; do
		need_memory "$name"
		echo "timings: $name, alone on core ${core_list[0]}"
		"$bin/colbinbench" "${bench_flags[@]}" -cases "$name" -cores "${core_list[0]}" >>"$out/bench-bulk.txt"
	done
fi
kill "$sampler" 2>/dev/null || true
sampler=""

# ---- the browser ------------------------------------------------------------

if [ "$browser" = 1 ] && [ "$regime" != bulk ]; then
	# The browser decodes the bytes the Go encoders write for the web cases,
	# after every format has been checked against the Go JSON (web/README.md).
	echo "browser: fixtures, verification, then $engines"
	go run ./cmd/webfixtures -data "$data" -cases "$cases"
	(
		cd web
		[ -d node_modules ] || npm ci
		[ -d gen ] || npx buf generate
		node verify.mjs --cases "$cases" >"$out/browser-verify.txt" || {
			echo "local.sh: a format does not decode a fixture to the Go JSON value; see $out/browser-verify.txt" >&2
			exit 1
		}
		node run.mjs --out "$out" --cores "$cores" --cases "$cases" --engines "$engines" --rounds "$count"
	)
	echo "colbin: $(node -e 'console.log(require("./web/node_modules/colbin/package.json").version)') from $(readlink -f web/node_modules/colbin)" >>"$out/machine.txt"
fi

# ---- frequency summary --------------------------------------------------------

{
	echo
	echo "== frequency of the measured cores during timings (MHz)"
	column=2
	for core in "${core_list[@]}"; do
		tail -n +2 "$out/freq.csv" | cut -d, -f"$column" | grep . | sort -n | awk -v core="$core" '
			{ v[NR] = $1 }
			END {
				if (NR == 0) { print "cpu" core ": no samples"; exit }
				printf "cpu%s: min %d, median %d, max %d, %d samples\n", core, v[1] / 1000, v[int((NR + 1) / 2)] / 1000, v[NR] / 1000, NR
			}'
		column=$((column + 1))
	done
} >>"$out/machine.txt"

echo
echo "results in $out"
echo "summarise with:"
[ -f "$out/bench-web.txt" ] && echo "  benchstat -filter 'regime:web' $out/bench-web.txt"
[ -f "$out/bench-bulk.txt" ] && echo "  benchstat -filter 'regime:bulk' $out/bench-bulk.txt"
echo "  two runs:  benchstat old/bench-web.txt new/bench-web.txt"
echo "(benchstat: go install golang.org/x/perf/cmd/benchstat@latest)"
