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
