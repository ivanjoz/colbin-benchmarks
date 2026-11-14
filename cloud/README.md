# The benchmarks on EC2

`cloud/ec2.py` runs the laptop's command, `bench/local.sh --all`, on EC2
instances, and has no benchmark code of its own: it creates the machines,
copies what the run needs, runs it, downloads `results/` and destroys
everything. The Go matrix's other cloud run is `lambda/deploy.py`.

```sh
cloud/ec2.py package                       # build the payload; no AWS
cloud/ec2.py --profile NAME up             # c7i.xlarge (x86_64) and c7g.xlarge (arm64)
cloud/ec2.py --profile NAME run            # copy, provision, bench/local.sh, download
cloud/ec2.py --profile NAME down           # terminate, delete the security group and key
```

## What happens

- **up**: an ed25519 key pair (saved to `cloud/state/`, not committed), a
  security group allowing SSH from the caller's public address only, and one
  instance per architecture: Ubuntu 24.04 (Canonical's AMI from SSM), 60 GB gp3,
  on demand, tagged `colbin-bench-ec2`. `--dedicated` asks for dedicated tenancy
  (no neighbours on the host; more expensive). `--x86-type`/`--arm-type` change
  the instance types.
- **run**: `package` writes three tarballs to `cloud/build/`: this repository's
  files (tracked and untracked-but-not-ignored, as in the working tree), the
  `../colbin` checkout with its built `js/dist` (no Rust needed remotely), and the
  datasets: every file `datasets/MANIFEST.sha256` pins plus the SchemaStore
  documents (154 MB compressed). On each instance, `provision.sh` installs Go
  (the `toolchain` of go.mod), Node, writes the same `go.work` as the laptop's
  (`use (. ../colbin)`), checks the data against the manifest, and installs the
  pinned web libraries and Playwright's browsers with their system libraries.
  Then `bench/local.sh` runs under `nohup`, so a dropped SSH session does not
  stop it (`run --attach` picks it up again), and its results directory is
  downloaded to `results/cloud/<instance type>/<date>/` with `run.log`.
- **down**: terminates every instance tagged `colbin-bench-ec2` (the volumes go
  with them), then deletes the security group and the key pair.

## The arguments bench/local.sh gets

By default `--all --cores auto --regime web`:

- `--cores auto` pins to one hardware thread per physical core, at most two
  cores, leaving the core of CPU 0 to the system when there are three or more.
  On c7i.xlarge (2 cores, each with two threads) that is one thread of each
  core; on c7g.xlarge (4 cores, no SMT) two of the three cores CPU 0 is not on.
- `--regime web`: the `.xlarge` sizes have 8 GiB, and bench/local.sh refuses a
  bulk case with less than 8 GB free (TPC-H nested and normalized need more than
  5). The browser only measures the web regime anyway. For the bulk regime on
  EC2, use `.2xlarge` (16 GiB) and pass the arguments yourself:
  `run -- --all --cores auto`.

The checks for a charger and the `performance` governor do not apply on EC2:
there is no power supply, and usually no cpufreq governor to read, which
`machine.txt` records as a warning rather than refusing to run.

## Engines

Ubuntu, because Playwright's WebKit build needs Ubuntu's libraries. An engine
that does not start is written as `# <engine>: not available: <why>` in its
`bench-browser-<engine>.txt`; WebKit on Linux arm64 is the one to watch.

# The Go matrix on OCI Container Instances

`cloud/ocibench.py` runs `./reproduce.sh` (the whole Go matrix, sizes and
ablations included; no browser) in an OCI container instance: x86 by
default, arm on request. Like `ec2.py`, it has no benchmark code of its own.

```sh
cloud/ocibench.py shapes          # the container shapes this region offers
cloud/ocibench.py up              # VCN, internet gateway, egress-only subnet, private bucket
cloud/ocibench.py run             # x86, at HEAD (committed and pushed)
cloud/ocibench.py run --label tpch -- --cases 'tpch/*'   # arguments after -- go to reproduce.sh
cloud/ocibench.py status          # every run's state and the tail of its progress.log
cloud/ocibench.py fetch           # finished runs → results/cloud/oci/<name>/
cloud/ocibench.py fetch --partial # and what unfinished ones have uploaded → <name>.partial/
cloud/ocibench.py down --bucket   # delete everything, the bucket with it
```

One container per dataset takes the longest dataset's time instead of the
sum (about 2 hours instead of 5); `cmd/mergeruns` joins the parts into one run,
and refuses parts that differ in the CPU, the commit, Go, colbin or the
sampling:

```sh
for d in tpch tpcds publicbi schemastore; do cloud/ocibench.py run --label $d -- --cases "$d/*"; done
cloud/ocibench.py fetch
go run ./cmd/mergeruns -out results/paper-v1 results/cloud/oci/oci-x86-{tpch,tpcds,publicbi,schemastore}-<stamps>
go run ./cmd/summary results/paper-v1
```

It needs `uv` (the script's header installs the OCI SDK, on Python 3.13: the
SDK's `crc32c` has no wheel for 3.15) and an API key in `~/.oci/config`
(`--profile`, `--compartment`; the default compartment is the tenancy's root).

## What happens

- **up** creates a VCN with an internet gateway, a public subnet whose security
  list allows egress only (nothing can connect in), and a private bucket. All
  tagged `colbin-bench-oci`; the ids go to `cloud/state/oci.json`.
- **run** refuses a dirty tree or a commit GitHub does not have. Per
  architecture it creates a write-only pre-authenticated URL to the bucket
  (three days) and a container instance from `golang:1.27.4-trixie`, pinned by
  digest. The container clones the repository at that commit and runs
  `cloud/oci-entry.sh` from it, which runs `./reproduce.sh --vm`: it fetches and
  verifies the datasets `--cases` names itself, as on any clean machine. Every
  ten minutes it uploads `progress.log` and the run directory so far
  (`<name>.partial.tar.gz`); at the end, the run directory and the exit status,
  whether the run succeeded or not.
- **status** and **fetch** find runs in the bucket and among the tagged
  container instances, not in the state file. Nothing runs on the machine that
  started a run: it can be switched off, and another machine with the same
  `~/.oci/config` can follow and fetch the run.
- **down** deletes the container instances (and any tagged ones the state file
  lost), the URLs and the network; `--bucket` empties and deletes the bucket.

| | shape | default |
|---|---|---|
| x86 | `CI.Standard.E4.Flex` (AMD EPYC Milan) | 4 OCPU = 4 cores, 8 threads; 32 GB |
| arm | `CI.Standard.A1.Flex` (Ampere Altra) | 4 OCPU = 4 cores; 32 GB |

32 GB because TPC-H's nested and normalized cases need over 5 GB and are
retried under a 9 GiB `GOMEMLIMIT`. reproduce.sh measures on two cores and
leaves the rest to the system and to the compression of the sizes stage, which
uses every core: `--ocpus 8` shortens that stage and nothing else.

## What these runs are

The paper's timings come from such a run. An E4 OCPU is a dedicated physical
core, but a container cannot set the frequency governor or turn boost off:
they are the host's, and the host's other tenants, who share its L3 cache and
memory bandwidth, change how far it boosts. So `reproduce.sh --vm` records
`frequency control: host` instead of refusing to start, and the run carries
its own evidence instead:

- colbinbench takes each case's samples in rounds, every format and operation
  once per round, so a change of clock falls on all of them alike;
- before every round it times a chain of dependent multiplies on the measured
  core (`cmd/colbinbench/probe.go`), which gives the clock the core ran at:
  `probe-bench-*.csv`, summarised in `SUMMARY.md` and as `probe.*` numbers.
  Nothing else in the container measures the clock: no cpufreq, no
  APERF/MPERF, a TSC at the nominal 2.45 GHz whatever the core does, and
  `perf_event_open` denied by the runtime although KVM exposes the core's
  cycle counter (`perfctr_core`; checked 2026-11-13). Where the counter can be
  read (a VM or a machine of one's own), the probe also counts the chain's
  cycles: the clock as the hardware counts it, and a check that a multiply is
  3 cycles (`counted_mhz`, `cycles_per_multiply`);
- the web cases are timed twice, and the paper reports how far the ratios
  moved between the passes.
