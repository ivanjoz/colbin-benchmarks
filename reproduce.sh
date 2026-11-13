#!/usr/bin/env bash
# reproduce.sh: every number of the paper, in one command.
#
#   ./reproduce.sh                              the whole matrix, web and bulk
#   ./reproduce.sh --regime web                 one regime only
#   ./reproduce.sh --cases 'schemastore/*'      some cases (comma-separated globs)
#   ./reproduce.sh --out results/paper-v1       where the results go
#   ./reproduce.sh --resume --out DIR           continue a run that stopped
#
# It measures nothing itself. It fetches and verifies the datasets, runs
# cmd/colbinbench (timings), matrix.TestSizes (sizes) and matrix.TestAblation
# (the threshold and codec ablations), and has cmd/summary write SUMMARY.md and
# the paper's tables. In this order, each stage keeping its output in --out the
# moment it has it:
#
#   1. datasets/*/fetch.sh           download or generate, check MANIFEST.sha256
#                                    (only the datasets --cases names)
#   2. checks, machine.txt           power, governor, turbo, clean tree, versions
#   3. timings, web                  bench-web.txt
#   4. timings, bulk                 bench-bulk.txt, one case at a time
#   5. sizes                         sizes.csv, cases.csv
#   6. ablations                     ablation.csv
#   7. timings, web, again           bench-web-2.txt: the repeatability check
#   8. cmd/summary                   SUMMARY.md, tables/*.tex, tables/*.dat
#
# Timings come first because they are what a machine left alone overnight is
# for; sizes do not depend on the CPU. Nothing a stage wrote is deleted when a
# later one fails: the failure is recorded in FAILED and progress.log, the run
# goes on to the next stage, and the exit status says so at the end. A sizes or
# ablation case that dies (the largest need over 5 GB) is retried once under
# GOMEMLIMIT.
#
# Options
#   --regime web|bulk|all    default all
#   --cases GLOBS            default all
#   --cores LIST|auto        CPUs to pin to; default auto: one hardware thread
#                            of each of two physical performance cores
#   --out DIR                default results/<host>-<YYYYMMDD-HHMMSS>
#   --passes N               timing passes over the web cases, default 2
#   --count N --sample D     samples per benchmark and their minimum time (10, 100ms)
#   --formats LIST --ops LIST
#   --min-mem-gb N           free memory a bulk case waits for, default 8
#   --mem-limit SIZE         GOMEMLIMIT of the retry, default 9GiB
#   --resume                 skip the stages --out already holds complete
#   --skip-fetch             the datasets are already in datasets/data
#   --no-ablation            skip stage 6
#   --workspace              build with go.work (a local colbin checkout) instead
#                            of the colbin tag go.mod requires; not a paper run
#   --browser                after the Go matrix, the browser benchmark
#                            (web/README.md); not part of the paper
#   --engines LIST           with --browser: chromium,firefox,webkit
#   --vm                     a virtual machine or container, where the governor and
#                            turbo are the host's: record that the host controls the
#                            clock instead of refusing to start; every other check
#                            still holds, and colbinbench's probe (probe-*.csv)
#                            records the clock the measured cores actually ran at
#   --no-checks              turn a failed check into a warning that machine.txt
#                            records: a smoke test, not a measurement
#
# A run that is the paper's has no warning in machine.txt: mains power, the
# performance governor on the measured cores and turbo off (or --vm), a clean
# working tree, and colbin from go.mod. cmd/summary marks any other run as provisional, and
# the paper prints that on its first page.
set -euo pipefail

command_line="$0 $*"
repo=$(cd "$(dirname "$0")" && pwd)
cd "$repo"

regime=all
cases=all
cores=auto
passes=2
count=10
sample=100ms
formats=all
ops=encode,decode-fresh,decode-reuse,access
min_mem_gb=8
mem_limit=9GiB
resume=0
fetch=1
ablation=1
workspace=0
browser=0
engines=chromium,firefox,webkit
checks=1
vm=0
out=""

usage() { sed -n '2,/^set -euo/p' "$0" | sed '$d; s/^# \{0,1\}//'; exit "${1:-0}"; }

while [ $# -gt 0 ]; do
	case "$1" in
	--regime) regime=$2; shift 2 ;;
	--cases) cases=$2; shift 2 ;;
	--cores) cores=$2; shift 2 ;;
	--out) out=$2; shift 2 ;;
	--passes) passes=$2; shift 2 ;;
	--count) count=$2; shift 2 ;;
	--sample) sample=$2; shift 2 ;;
	--formats) formats=$2; shift 2 ;;
	--ops) ops=$2; shift 2 ;;
	--min-mem-gb) min_mem_gb=$2; shift 2 ;;
	--mem-limit) mem_limit=$2; shift 2 ;;
	--resume) resume=1; shift ;;
	--skip-fetch) fetch=0; shift ;;
	--no-ablation) ablation=0; shift ;;
	--workspace) workspace=1; shift ;;
	--browser) browser=1; shift ;;
	--engines) engines=$2; shift 2 ;;
	--vm) vm=1; shift ;;
	--no-checks) checks=0; shift ;;
	-h | --help) usage ;;
	*) echo "reproduce.sh: unknown argument $1" >&2; usage 2 ;;
	esac
done
case "$regime" in all | web | bulk) ;; *) echo "reproduce.sh: --regime is web, bulk or all" >&2; exit 2 ;; esac

# colbin comes from the tag go.mod requires unless --workspace says otherwise,
# whatever go.work lies beside this checkout.
[ "$workspace" = 1 ] || export GOWORK=off

out=${out:-results/$(hostname -s)-$(date +%Y%m%d-%H%M%S)}
if [ "$resume" = 0 ] && [ -e "$out/machine.txt" ]; then
	echo "reproduce.sh: $out already holds a run; pass --resume to continue it, or another --out" >&2
	exit 2
fi
mkdir -p "$out/stages"
out=$(cd "$out" && pwd)
data=${COLBIN_DATA:-$repo/datasets/data}
failures=0

log() { echo "[$(date +%H:%M:%S)] $*" | tee -a "$out/progress.log"; }
failed() { # what
	failures=$((failures + 1))
	echo "$1" >>"$out/FAILED"
	log "FAILED: $1"
}
# A stage is done when its marker exists; --resume skips it.
done_already() { [ "$resume" = 1 ] && [ -e "$out/stages/$1.done" ]; }
mark_done() { touch "$out/stages/$1.done"; }
slug() { echo "${1//\//_}"; }
rm -f "$out/FAILED"

warnings=()
fail() {
	if [ "$checks" = 1 ]; then
		echo "reproduce.sh: $1" >&2
		echo "reproduce.sh: fix it, or pass --no-checks for a smoke test that is not a measurement" >&2
		exit 1
	fi
	echo "reproduce.sh: warning: $1" >&2
	warnings+=("$1")
}

# The governor and turbo: on a VM they are the host's, out of reach, so --vm
# records what it saw rather than refusing to start. Such a run is not
# provisional for that: the probe measures the clock it ran at instead.
host_settings=()
frequency_fail() {
	if [ "$vm" = 1 ]; then
		host_settings+=("$1")
	else
		fail "$1"
	fi
}

# What cannot be read is a warning on a machine of one's own, and expected on
# a VM, where cpufreq is the host's and is not exposed: there the probe stands
# in for it.
frequency_unknown() {
	if [ "$vm" = 1 ]; then
		host_settings+=("$1")
	else
		warnings+=("$1")
	fi
}

mem_available_gb() { awk '/^MemAvailable:/ { printf "%.1f", $2 / 1048576 }' /proc/meminfo; }

# ---- 1. datasets ----------------------------------------------------------------

if [ "$fetch" = 1 ] && ! done_already fetch; then
	# Only the datasets --cases names, when each of its globs names one.
	fetchers=()
	if [ "$cases" != all ]; then
		IFS=, read -r -a globs <<<"$cases"
		for glob in "${globs[@]}"; do
			dataset=${glob%%/*}
			if [[ $dataset == *[*?[]* ]] || [ ! -x "datasets/$dataset/fetch.sh" ]; then
				fetchers=()
				break
			fi
			[[ " ${fetchers[*]-} " == *" datasets/$dataset/fetch.sh "* ]] || fetchers+=("datasets/$dataset/fetch.sh")
		done
	fi
	[ "${#fetchers[@]}" -gt 0 ] || fetchers=(datasets/fetch-all.sh)
	log "datasets: fetch and verify (${fetchers[*]})"
	(for fetcher in "${fetchers[@]}"; do COLBIN_DATA="$data" "$fetcher" || exit 1; done) >"$out/stages/fetch.log" 2>&1 || {
		tail -20 "$out/stages/fetch.log" >&2
		echo "reproduce.sh: a dataset did not fetch or does not match datasets/MANIFEST.sha256; see $out/stages/fetch.log" >&2
		exit 1
	}
	mark_done fetch
fi

# ---- 2. checks before anything is measured, and machine.txt ----------------------

# --cores auto: one hardware thread per physical core, the last two of them, and
# not the core CPU 0 is on when there are three or more (the system's). On a
# hybrid CPU only the performance cores count (/sys/devices/cpu_core/cpus).
if [ "$cores" = auto ]; then
	topology=$(lscpu -p=CPU,CORE | grep -v '^#')
	if [ -r /sys/devices/cpu_core/cpus ]; then
		performance=$(tr , '\n' </sys/devices/cpu_core/cpus | awk -F- '{ for (c = $1; c <= ($2 == "" ? $1 : $2); c++) print c }')
		topology=$(echo "$topology" | awk -F, 'NR == FNR { keep[$1]; next } $1 in keep' <(echo "$performance") -)
	fi
	firsts=$(echo "$topology" | sort -t, -k2,2n -k1,1n | awk -F, '!seen[$2]++ { print $1 }')
	if [ "$(echo "$firsts" | wc -l)" -ge 3 ]; then
		cpu0_core=$(echo "$topology" | awk -F, '$1 == 0 { print $2 }')
		firsts=$(echo "$topology" | sort -t, -k2,2n -k1,1n | awk -F, -v skip="$cpu0_core" '$2 != skip && !seen[$2]++ { print $1 }')
	fi
	cores=$(echo "$firsts" | tail -2 | paste -sd,)
	echo "cores: auto → $cores"
fi
IFS=, read -r -a core_list <<<"$cores"

supplies=(/sys/class/power_supply/*/online)
if [ -e "${supplies[0]}" ]; then
	on_ac=0
	for supply in "${supplies[@]}"; do [ "$(cat "$supply")" = 1 ] && on_ac=1; done
	[ "$on_ac" = 1 ] || fail "on battery: plug the charger in"
fi
governors=""
for core in "${core_list[@]}"; do
	governor=/sys/devices/system/cpu/cpu$core/cpufreq/scaling_governor
	if [ -r "$governor" ]; then
		governors="$governors $(cat "$governor")"
		[ "$(cat "$governor")" = performance ] || frequency_fail "cpu$core governor is $(cat "$governor"), not performance (sudo cpupower frequency-set -g performance)"
	else
		governors="$governors unknown"
		frequency_unknown "cpu$core has no cpufreq governor to check"
	fi
done
# Turbo (frequency boost): on, a core's frequency depends on what the other
# cores are doing and on the package's temperature, which is drift between two
# passes. intel_pstate says no_turbo; acpi-cpufreq and amd-pstate say boost.
turbo=unknown
if [ -r /sys/devices/system/cpu/intel_pstate/no_turbo ]; then
	[ "$(cat /sys/devices/system/cpu/intel_pstate/no_turbo)" = 1 ] && turbo=off || turbo=on
	turbo_hint="echo 1 | sudo tee /sys/devices/system/cpu/intel_pstate/no_turbo"
elif [ -r /sys/devices/system/cpu/cpufreq/boost ]; then
	[ "$(cat /sys/devices/system/cpu/cpufreq/boost)" = 0 ] && turbo=off || turbo=on
	turbo_hint="echo 0 | sudo tee /sys/devices/system/cpu/cpufreq/boost"
fi
case "$turbo" in
on) frequency_fail "turbo is on ($turbo_hint)" ;;
unknown) frequency_unknown "turbo: no intel_pstate/no_turbo or cpufreq/boost to read" ;;
esac

commit_of() { # dir → "commit" or "commit (uncommitted changes)"
	local dirty=""
	[ -n "$(git -C "$1" status --porcelain 2>/dev/null)" ] && dirty=" (uncommitted changes)"
	echo "$(git -C "$1" rev-parse HEAD)$dirty"
}
if [ -n "$(git status --porcelain)" ]; then
	fail "the working tree has uncommitted changes: the commit machine.txt names would not be what ran"
fi
colbin_version() {
	# What the build resolves, not what was asked for: `go env GOWORK` is the
	# workspace file in use, empty or "off" when there is none.
	local work
	work=$(go env GOWORK)
	if [ -n "$work" ] && [ "$work" != off ] && grep -q '\.\./colbin' "$work"; then
		echo "../colbin via go.work at $(commit_of ../colbin), $(git -C ../colbin describe --tags --always 2>/dev/null)"
	else
		go list -m github.com/ivanjoz/colbin
	fi
}
work=$(go env GOWORK)
if [ -n "$work" ] && [ "$work" != off ]; then
	warnings+=("--workspace: colbin from go.work, not from the tag go.mod requires")
fi

busiest=$(ps -eo pcpu,pid,comm --sort=-pcpu | head -6)
echo "busiest processes before the run:"
echo "$busiest"
echo "close browsers, IDEs and sync clients before measuring"

if [ "$resume" = 1 ] && [ -e "$out/machine.txt" ]; then
	# The description is the first start's; a resumed run says when it resumed
	# and from which commit, above the lscpu block.
	sed -i "/^== lscpu/i resumed: $(date -Iseconds), colbin-benchmarks $(commit_of .)" "$out/machine.txt"
else
	{
		echo "date: $(date -Iseconds)"
		echo "driver: reproduce.sh"
		echo "command: $command_line"
		echo "regime: $regime"
		echo "cases: $cases"
		echo "measured cores: $cores"
		echo "passes: $passes"
		echo "count: $count"
		echo "sample: $sample"
		echo "formats: $formats"
		echo "ops: $ops"
		echo "kernel: $(uname -sr)"
		echo "go: $(go version)"
		echo "colbin: $(colbin_version)"
		echo "colbin-benchmarks: $(commit_of .)"
		echo "governor:$governors"
		echo "turbo: $turbo"
		if [ "$vm" = 1 ]; then
			echo "frequency control: host"
			for setting in "${host_settings[@]+"${host_settings[@]}"}"; do echo "host setting: $setting"; done
		else
			echo "frequency control: local"
		fi
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
fi

# ---- builds ---------------------------------------------------------------------

# Only the binaries live outside --out, and only they are removed on exit.
bin=$(mktemp -d)
sampler=""
cleanup() {
	[ -n "$sampler" ] && kill "$sampler" 2>/dev/null || true
	rm -rf "$bin"
}
trap cleanup EXIT
log "building colbinbench and the matrix test binaries"
go build -o "$bin/colbinbench" ./cmd/colbinbench
go test -c -o "$bin/matrix.test" ./matrix
[ "$ablation" = 0 ] || go test -c -tags colbin_ablation -o "$bin/ablation.test" ./matrix

# Which cases run, and in which regime. Listing loads each case once.
"$bin/colbinbench" -data "$data" -list -cases "$cases" >"$out/stages/cases.txt"
web_cases=$(awk '$2 == "web" { print $1 }' "$out/stages/cases.txt" | paste -sd,)
bulk_cases=$(awk '$2 == "bulk" { print $1 }' "$out/stages/cases.txt")
[ "$regime" = bulk ] && web_cases=""
[ "$regime" = web ] && bulk_cases=""
log "web cases: $(echo "$web_cases" | tr , '\n' | grep -c . || true), bulk cases: $(echo "$bulk_cases" | grep -c . || true)"
if [ -z "$web_cases" ] && [ -z "$bulk_cases" ]; then
	echo "reproduce.sh: no fetched case matches --cases $cases --regime $regime" >&2
	exit 1
fi

# A bulk case waits for memory instead of dying for the lack of it: what holds
# it is usually the previous case's pages on their way back to the system.
wait_memory() { # case
	local waited=0 free
	while :; do
		free=$(mem_available_gb)
		awk -v f="$free" -v m="$min_mem_gb" 'BEGIN { exit !(f < m) }' || return 0
		if [ "$waited" -ge 120 ]; then
			log "$1: only $free GB available after two minutes, a bulk case wants $min_mem_gb GB; trying anyway"
			return 0
		fi
		sleep 10
		waited=$((waited + 10))
	done
}

# ---- the frequency of the measured cores, once a second, during timings ----------

[ -e "$out/freq.csv" ] || {
	printf 'time'
	for core in "${core_list[@]}"; do printf ',cpu%s_khz' "$core"; done
	echo
} >"$out/freq.csv"
sampler_start() {
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
}
sampler_stop() {
	[ -n "$sampler" ] && kill "$sampler" 2>/dev/null || true
	sampler=""
}

header() { echo "cpu: $(lscpu | sed -n 's/^Model name: *//p' | head -1)"; }
# colbinbench appends each case's core clock, before every round of samples,
# to probe-<file>.csv (cmd/colbinbench/probe.go).
probe_header="case,round,ns_per_multiply,mhz"
bench_flags=(-data "$data" -formats "$formats" -ops "$ops" -count "$count" -sample "$sample")

timings_web() { # pass
	local stage=bench-web file=bench-web.txt
	[ "$1" = 1 ] || { stage=bench-web-$1; file=bench-web-$1.txt; }
	[ -n "$web_cases" ] || return 0
	done_already "$stage" && return 0
	log "timings: web, pass $1, ${#core_list[@]} at a time on cores $cores"
	echo "$probe_header" >"$out/probe-${file%.txt}.csv"
	sampler_start
	if { header; "$bin/colbinbench" "${bench_flags[@]}" -probe "$out/probe-${file%.txt}.csv" -cases "$web_cases" -cores "$cores"; } >"$out/$file.part" 2>"$out/${file%.txt}.err"; then
		mv "$out/$file.part" "$out/$file"
		mark_done "$stage"
		log "timings: web, pass $1 done, $(grep -c '^Benchmark' "$out/$file") lines"
	else
		failed "timings web pass $1 (partial output in $file.part, errors in ${file%.txt}.err)"
	fi
	sampler_stop
}

timings_bulk() {
	[ -n "$bulk_cases" ] || return 0
	mkdir -p "$out/stages/bench-bulk"
	local name part index=0 complete=1
	[ -e "$out/probe-bench-bulk.csv" ] || echo "$probe_header" >"$out/probe-bench-bulk.csv"
	sampler_start
	for name in $bulk_cases; do
		part=$out/stages/bench-bulk/$(printf '%02d' "$index")-$(slug "$name")
		index=$((index + 1))
		[ "$resume" = 1 ] && [ -e "$part.txt" ] && continue
		wait_memory "$name"
		log "timings: $name, alone on core ${core_list[0]}"
		if "$bin/colbinbench" "${bench_flags[@]}" -probe "$out/probe-bench-bulk.csv" -cases "$name" -cores "${core_list[0]}" >"$part.part" 2>"$part.err"; then
			mv "$part.part" "$part.txt"
		else
			complete=0
			failed "timings $name (see $part.err)"
		fi
	done
	sampler_stop
	{ header; cat "$out"/stages/bench-bulk/*.txt 2>/dev/null; } >"$out/bench-bulk.txt"
	cat "$out"/stages/bench-bulk/*.err >"$out/bench-bulk.err" 2>/dev/null || true
	[ "$complete" = 0 ] || log "timings: bulk done"
}

# One test process for the web cases, and one per bulk case; each writes its
# CSVs into its own directory under stages/, and they are joined under one
# header. A process that fails is run once more under GOMEMLIMIT: the largest
# cases die of memory, not of a wrong answer, and a wrong answer fails again.
test_run() { # binary, test, dir, cases
	mkdir -p "$3"
	(cd matrix && COLBIN_DATA="$data" COLBIN_RESULTS="$3" COLBIN_CASES="$4" \
		"$1" -test.run "^$2\$" -test.v -test.timeout 0 >"$3/test.log" 2>&1) && return 0
	log "$2 failed for ${4:0:60}; once more with GOMEMLIMIT=$mem_limit"
	mv "$3/test.log" "$3/test-first.log"
	(cd matrix && GOMEMLIMIT="$mem_limit" COLBIN_DATA="$data" COLBIN_RESULTS="$3" COLBIN_CASES="$4" \
		"$1" -test.run "^$2\$" -test.v -test.timeout 0 >"$3/test.log" 2>&1)
}
csv_stage() { # stage, binary, test, tables...
	local stage=$1 binary=$2 test=$3
	shift 3
	local name dir table part first
	if [ -n "$web_cases" ] && ! { [ "$resume" = 1 ] && [ -e "$out/stages/$stage/web/ok" ]; }; then
		log "$stage: web"
		dir=$out/stages/$stage/web
		if test_run "$binary" "$test" "$dir" "$web_cases"; then touch "$dir/ok"; else failed "$stage web (see $dir/test.log)"; fi
	fi
	for name in $bulk_cases; do
		dir=$out/stages/$stage/$(slug "$name")
		[ "$resume" = 1 ] && [ -e "$dir/ok" ] && continue
		wait_memory "$name"
		log "$stage: $name"
		if test_run "$binary" "$test" "$dir" "$name"; then touch "$dir/ok"; else failed "$stage $name (see $dir/test.log)"; fi
	done
	# Joined in the matrix's order: web first, then the bulk cases as listed.
	for table in "$@"; do
		first=1
		for name in web $bulk_cases; do
			part=$out/stages/$stage/$(slug "$name")/$table
			[ -f "$part" ] || continue
			if [ "$first" = 1 ]; then cat "$part"; first=0; else tail -n +2 "$part"; fi
		done >"$out/$table"
	done
}

# ---- 3-7. the stages ---------------------------------------------------------------

timings_web 1
timings_bulk
csv_stage sizes "$bin/matrix.test" TestSizes sizes.csv cases.csv
[ "$ablation" = 0 ] || csv_stage ablation "$bin/ablation.test" TestAblation ablation.csv
pass=2
while [ "$pass" -le "$passes" ]; do
	timings_web "$pass"
	pass=$((pass + 1))
done

# ---- the browser: not part of the paper --------------------------------------------

if [ "$browser" = 1 ] && [ "$regime" != bulk ]; then
	# The browser decodes the bytes the Go encoders write for the web cases,
	# after every format has been checked against the Go JSON (web/README.md).
	log "browser: fixtures, verification, then $engines"
	sampler_start
	if go run ./cmd/webfixtures -data "$data" -cases "$cases" && (
		cd web
		[ -d node_modules ] || npm ci
		[ -d gen ] || npx buf generate
		node verify.mjs --cases "$cases" >"$out/browser-verify.txt" || {
			echo "reproduce.sh: a format does not decode a fixture to the Go JSON value; see $out/browser-verify.txt" >&2
			exit 1
		}
		node run.mjs --out "$out" --cores "$cores" --cases "$cases" --engines "$engines" --rounds "$count"
	); then
		echo "colbin js: $(node -e 'console.log(require("./web/node_modules/colbin/package.json").version)') from $(readlink -f web/node_modules/colbin)" >>"$out/machine.txt"
	else
		failed "browser benchmark"
	fi
	sampler_stop
fi

# ---- frequency summary ---------------------------------------------------------------

sed -i '/^== frequency of the measured cores/,$d' "$out/machine.txt"
{
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

# ---- 8. SUMMARY.md and the paper's tables ----------------------------------------------

log "summary"
go run ./cmd/summary -wasm web/node_modules/colbin/dist/colbin.wasm "$out" || failed "cmd/summary"

echo
if [ "$failures" -gt 0 ]; then
	log "done, with $failures failed stage(s): see $out/FAILED; fix the cause and run again with --resume --out $out"
	exit 1
fi
touch "$out/DONE"
log "done: results in $out (start with SUMMARY.md; the paper's tables are in tables/)"
echo "per-case statistics:  benchstat -filter 'regime:web' $out/bench-web.txt"
echo "the paper:            make -C paper RUN=$out"
