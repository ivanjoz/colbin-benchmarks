#!/usr/bin/env python3
"""Run bench/local.sh on EC2: the same command as on the laptop, on a machine
that is not a laptop.

Nothing here measures anything. It creates the machines, copies this
repository, the colbin checkout it builds against and the pinned datasets,
runs `bench/local.sh --all` there, downloads `results/`, and destroys what it
created. Two instances by default, one per architecture:

    x86_64   c7i.xlarge   (Sapphire Rapids, 4 vCPU = 2 cores with SMT, 8 GiB)
    arm64    c7g.xlarge   (Graviton3, 4 vCPU = 4 cores, 8 GiB)

Ubuntu 24.04, because Playwright's WebKit build needs Ubuntu's libraries; an
engine that still cannot start is reported as not available in its result
file, never skipped silently (WebKit on Linux arm64 may be such a case).

    cloud/ec2.py package                      # the payload and the scripts, no AWS
    cloud/ec2.py --profile NAME up            # key pair, security group, instances
    cloud/ec2.py --profile NAME run           # copy, provision, bench/local.sh, download
    cloud/ec2.py --profile NAME status
    cloud/ec2.py --profile NAME down          # terminate, delete the group and the key

`run` passes `--all --cores auto --regime web` to bench/local.sh unless told
otherwise (`run -- --all --cores auto`): 8 GiB is not enough for the bulk
cases, which need 8 GB free; the Go bulk regime runs on Lambda (lambda/deploy.py)
or on a larger instance (`up --x86-type c7i.2xlarge --arm-type c7g.2xlarge`).

The run continues on the instance if the SSH session drops; `run --attach`
waits for a run already started and downloads its results. Results land in
results/cloud/<instance type>/<the run's date>/.

What `up` created is recorded in cloud/state/ (not committed), which `down`
reads; `down` also finds anything tagged colbin-bench-ec2 that the state file
lost. Run from anywhere; paths are relative to the repository.
"""

import argparse
import concurrent.futures
import json
import os
import pathlib
import shlex
import subprocess
import sys
import tarfile
import time
import urllib.request

REPO = pathlib.Path(__file__).resolve().parent.parent
COLBIN = REPO.parent / "colbin"
DATA = REPO / "datasets" / "data"
STATE = REPO / "cloud" / "state"
BUILD = REPO / "cloud" / "build"
RESULTS = REPO / "results" / "cloud"

TAG = "colbin-bench-ec2"
KEY_NAME = TAG
GROUP_NAME = TAG
USER = "ubuntu"
VOLUME_GB = 60
GO_VERSION = "1.27.4"  # go.mod's toolchain line
NODE_VERSION = "24.15.0"

ARCHES = {
    # arch: (default instance type, SSM parameter of the Ubuntu 24.04 AMI, Go/Node arch names)
    "x86_64": ("c7i.xlarge", "/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id", "amd64", "x64"),
    "arm64": ("c7g.xlarge", "/aws/service/canonical/ubuntu/server/24.04/stable/current/arm64/hvm/ebs-gp3/ami-id", "arm64", "arm64"),
}

# What the benchmark reads from datasets/data: every file the manifest pins,
# and the SchemaStore documents (a git checkout, pinned by its commit).
SCHEMASTORE_GLOB = "schemastore/binary-json-size-benchmark/benchmark/*/document.json"


# --- package: everything that goes to the instance, built without AWS ------------


def tracked_files(repo):
    """The files git tracks in repo, as they are in the working tree, so that
    uncommitted changes travel too (machine.txt records whether there were)."""
    out = subprocess.run(["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"], cwd=repo, check=True, capture_output=True).stdout
    return [name for name in out.decode().split("\0") if name and (repo / name).is_file()]


def write_tar(path, root, names, prefix):
    with tarfile.open(path, "w:gz") as tar:
        for name in names:
            tar.add(root / name, arcname=f"{prefix}/{name}", recursive=False)
    print(f"  {path.relative_to(REPO)}: {len(names)} files, {path.stat().st_size / 1e6:.1f} MB")


def package():
    """Writes cloud/build/: the three tarballs and the provisioning script."""
    if not (COLBIN / "js" / "dist" / "colbin.wasm").exists():
        sys.exit("../colbin/js/dist is not built: cd ../colbin/js && bun run build")
    manifest = (REPO / "datasets" / "MANIFEST.sha256").read_text().split("\n")
    pinned = [line.split(None, 1)[1].strip() for line in manifest if line.strip()]
    missing = [name for name in pinned if not (DATA / name).exists()]
    if missing:
        sys.exit(f"{len(missing)} pinned files are not fetched, e.g. {missing[0]}: run datasets/fetch-all.sh")
    documents = sorted(str(path.relative_to(DATA)) for path in DATA.glob(SCHEMASTORE_GLOB))
    if not documents:
        sys.exit("no SchemaStore documents: run datasets/schemastore/fetch.sh")

    BUILD.mkdir(parents=True, exist_ok=True)
    print("packaging:")
    write_tar(BUILD / "repo.tar.gz", REPO, tracked_files(REPO), "colbin-benchmarks")
    # colbin: its tracked files and the built JavaScript package, so the
    # instance needs no Rust toolchain and runs the exact module measured here.
    dist = [str(path.relative_to(COLBIN)) for path in (COLBIN / "js" / "dist").rglob("*") if path.is_file()]
    write_tar(BUILD / "colbin.tar.gz", COLBIN, sorted(set(tracked_files(COLBIN)) | set(dist)), "colbin")
    write_tar(BUILD / "data.tar.gz", DATA, pinned + documents, "colbin-benchmarks/datasets/data")
    (BUILD / "provision.sh").write_text(PROVISION)
    print(f"  {(BUILD / 'provision.sh').relative_to(REPO)}")


# Runs on the instance as the ubuntu user, from its home directory, after the
# tarballs are unpacked there: toolchains, the browsers with their system
# libraries, the manifest check. Idempotent, so a second run is quick.
PROVISION = r"""#!/usr/bin/env bash
set -euo pipefail
GOARCH_NAME=$1 NODE_ARCH=$2 GO_VERSION=$3 NODE_VERSION=$4
export DEBIAN_FRONTEND=noninteractive
sudo apt-get update -qq
sudo apt-get install -y -qq git curl ca-certificates xz-utils zstd util-linux procps python3 >/dev/null
if [ ! -x /usr/local/go/bin/go ] || ! /usr/local/go/bin/go version | grep -q "go$GO_VERSION "; then
	curl -fsSL "https://go.dev/dl/go$GO_VERSION.linux-$GOARCH_NAME.tar.gz" | sudo tar -C /usr/local -xz
fi
if [ ! -x "/opt/node/bin/node" ] || [ "$(/opt/node/bin/node --version)" != "v$NODE_VERSION" ]; then
	sudo rm -rf /opt/node && sudo mkdir -p /opt/node
	curl -fsSL "https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-$NODE_ARCH.tar.xz" | sudo tar -C /opt/node -xJ --strip-components=1
fi
export PATH=/usr/local/go/bin:/opt/node/bin:$HOME/go/bin:$PATH
grep -q /opt/node/bin ~/.profile || echo 'export PATH=/usr/local/go/bin:/opt/node/bin:$HOME/go/bin:$PATH' >>~/.profile

cd ~/colbin-benchmarks
# The build is against the colbin checkout copied beside the repository, as on the laptop.
printf 'go %s\n\nuse (\n\t.\n\t../colbin\n)\n' "$GO_VERSION" >go.work
(cd datasets/data && sha256sum --quiet -c ../MANIFEST.sha256)
go install golang.org/x/perf/cmd/benchstat@latest
cd web
npm ci --no-audit --no-fund
npx buf generate
# WebKit may not install (Linux arm64): the run reports it as not available.
sudo env PATH="$PATH" npx playwright install-deps chromium firefox webkit || true
npx playwright install chromium firefox webkit || npx playwright install chromium firefox
echo provisioned
"""


# --- AWS ---------------------------------------------------------------------------


def session_for(args):
    import boto3
    import botocore.exceptions

    session = boto3.Session(profile_name=args.profile, region_name=args.region)
    try:
        identity = session.client("sts").get_caller_identity()
    except botocore.exceptions.BotoCoreError as error:
        sys.exit(f"AWS credentials: {error}")
    print(f"account {identity['Account']}, {identity['Arn']}, region {session.region_name}")
    return session


def load_state():
    path = STATE / "instances.json"
    return json.loads(path.read_text()) if path.exists() else {}


def save_state(state):
    STATE.mkdir(parents=True, exist_ok=True)
    (STATE / "instances.json").write_text(json.dumps(state, indent=2) + "\n")


def key_path():
    return STATE / f"{KEY_NAME}.pem"


def my_ip():
    with urllib.request.urlopen("https://checkip.amazonaws.com", timeout=10) as response:
        return response.read().decode().strip()


def tags(name):
    return [{"ResourceType": kind, "Tags": [{"Key": "Name", "Value": name}, {"Key": TAG, "Value": "1"}]}
            for kind in ("instance", "volume")]


def up(session, arches, types, dedicated):
    ec2 = session.client("ec2")
    ssm = session.client("ssm")
    state = load_state()
    STATE.mkdir(parents=True, exist_ok=True)

    if not key_path().exists():
        key = ec2.create_key_pair(KeyName=KEY_NAME, KeyType="ed25519")
        key_path().write_text(key["KeyMaterial"])
        key_path().chmod(0o600)
        print(f"key pair {KEY_NAME} → {key_path().relative_to(REPO)}")

    if "group" not in state:
        vpc = ec2.describe_vpcs(Filters=[{"Name": "is-default", "Values": ["true"]}])["Vpcs"][0]["VpcId"]
        group = ec2.create_security_group(GroupName=GROUP_NAME, Description="colbin-bench EC2: SSH from one address",
                                          VpcId=vpc)["GroupId"]
        ip = my_ip()
        ec2.authorize_security_group_ingress(GroupId=group, IpPermissions=[{
            "IpProtocol": "tcp", "FromPort": 22, "ToPort": 22, "IpRanges": [{"CidrIp": f"{ip}/32"}]}])
        state["group"] = group
        save_state(state)
        print(f"security group {group}: SSH from {ip} only")

    for arch in arches:
        if arch in state.get("instances", {}):
            print(f"{arch}: {state['instances'][arch]['id']} already up")
            continue
        ami = ssm.get_parameter(Name=ARCHES[arch][1])["Parameter"]["Value"]
        request = dict(
            ImageId=ami, InstanceType=types[arch], MinCount=1, MaxCount=1, KeyName=KEY_NAME,
            SecurityGroupIds=[state["group"]], TagSpecifications=tags(f"{TAG}-{arch}"),
            BlockDeviceMappings=[{"DeviceName": "/dev/sda1", "Ebs": {"VolumeSize": VOLUME_GB, "VolumeType": "gp3",
                                                                    "DeleteOnTermination": True}}],
            InstanceInitiatedShutdownBehavior="terminate",
        )
        if dedicated:
            request["Placement"] = {"Tenancy": "dedicated"}
        instance = ec2.run_instances(**request)["Instances"][0]
        state.setdefault("instances", {})[arch] = {"id": instance["InstanceId"], "type": types[arch], "ami": ami,
                                                   "dedicated": dedicated}
        save_state(state)
        print(f"{arch}: {instance['InstanceId']} ({types[arch]}{', dedicated' if dedicated else ''}, {ami})")

    ids = [entry["id"] for entry in state.get("instances", {}).values()]
    if ids:
        print("waiting for the instances to pass their status checks")
        ec2.get_waiter("instance_status_ok").wait(InstanceIds=ids)
        refresh_addresses(ec2, state)
        for arch, entry in state["instances"].items():
            print(f"{arch}: ssh -i {key_path()} {USER}@{entry['address']}")


def refresh_addresses(ec2, state):
    ids = {entry["id"]: arch for arch, entry in state.get("instances", {}).items()}
    if not ids:
        return
    for reservation in ec2.describe_instances(InstanceIds=list(ids))["Reservations"]:
        for instance in reservation["Instances"]:
            state["instances"][ids[instance["InstanceId"]]]["address"] = instance.get("PublicIpAddress")
    save_state(state)


def ssh_command(address):
    return ["ssh", "-i", str(key_path()), "-o", "StrictHostKeyChecking=accept-new",
            "-o", f"UserKnownHostsFile={STATE / 'known_hosts'}", "-o", "ServerAliveInterval=30", f"{USER}@{address}"]


def ssh(address, command, check=True, capture=False):
    return subprocess.run([*ssh_command(address), command], check=check, text=True,
                          capture_output=capture)


def scp_to(address, paths, destination="~"):
    subprocess.run(["scp", "-q", "-i", str(key_path()), "-o", "StrictHostKeyChecking=accept-new",
                    "-o", f"UserKnownHostsFile={STATE / 'known_hosts'}", *map(str, paths),
                    f"{USER}@{address}:{destination}"], check=True)


def run_on(arch, entry, local_args, attach, poll):
    """Provisions one instance, starts bench/local.sh there in the background,
    waits for it and downloads its results directory."""
    address = entry["address"]
    log = f"[{arch}]"
    if not attach:
        print(f"{log} copying the repository, colbin and the data")
        scp_to(address, [BUILD / "repo.tar.gz", BUILD / "colbin.tar.gz", BUILD / "data.tar.gz", BUILD / "provision.sh"])
        ssh(address, "rm -rf colbin-benchmarks colbin && tar -xzf repo.tar.gz && tar -xzf colbin.tar.gz"
                     " && tar -xzf data.tar.gz && rm -f repo.tar.gz colbin.tar.gz data.tar.gz")
        print(f"{log} provisioning (Go {GO_VERSION}, Node {NODE_VERSION}, Playwright browsers)")
        _, _, goarch, nodearch = ARCHES[arch]
        ssh(address, f"bash provision.sh {goarch} {nodearch} {GO_VERSION} {NODE_VERSION} >provision.log 2>&1"
                     " || { tail -30 provision.log; exit 1; }")
        out = f"results/local/{time.strftime('%Y%m%d-%H%M%S')}"
        command = (f"cd colbin-benchmarks && . ~/.profile && rm -f ~/run.done ~/run.out"
                   f" && echo {shlex.quote(out)} >~/run.out"
                   f" && nohup bash -c {shlex.quote(f'bench/local.sh {shlex.join(local_args)} --out {out} >~/run.log 2>&1; echo $? >~/run.done')}"
                   " >/dev/null 2>&1 &")
        print(f"{log} bench/local.sh {' '.join(local_args)}")
        ssh(address, command)
    out = ssh(address, "cat ~/run.out", capture=True).stdout.strip()
    while True:
        done = ssh(address, "cat ~/run.done 2>/dev/null || true", capture=True, check=False).stdout.strip()
        if done:
            break
        last = ssh(address, "tail -1 ~/run.log 2>/dev/null || true", capture=True, check=False).stdout.strip()
        print(f"{log} {last[:150]}")
        time.sleep(poll)
    if done != "0":
        print(f"{log} bench/local.sh exited {done}; its log:")
        print(ssh(address, "tail -40 ~/run.log", capture=True, check=False).stdout)
    target = RESULTS / entry["type"] / pathlib.Path(out).name
    target.mkdir(parents=True, exist_ok=True)
    tar = subprocess.run([*ssh_command(address), f"cd colbin-benchmarks/{out} && tar -cz ."],
                         check=True, capture_output=True).stdout
    subprocess.run(["tar", "-xz", "-C", str(target)], input=tar, check=True)
    (target / "run.log").write_text(ssh(address, "cat ~/run.log", capture=True, check=False).stdout)
    print(f"{log} results in {target.relative_to(REPO)}")
    return done == "0"


def run(session, arches, local_args, attach, poll):
    state = load_state()
    refresh_addresses(session.client("ec2"), state)
    chosen = {arch: entry for arch, entry in state.get("instances", {}).items() if arch in arches}
    if not chosen:
        sys.exit("no instance up: cloud/ec2.py up first")
    if not attach:
        package()
    with concurrent.futures.ThreadPoolExecutor(len(chosen)) as pool:
        futures = {pool.submit(run_on, arch, entry, local_args, attach, poll): arch for arch, entry in chosen.items()}
        ok = all(future.result() for future in futures)
    print("done" if ok else "a run failed; the instances are still up, for a look (ssh) or `down`")


def status(session):
    ec2 = session.client("ec2")
    state = load_state()
    found = ec2.describe_instances(Filters=[{"Name": f"tag:{TAG}", "Values": ["1"]},
                                            {"Name": "instance-state-name", "Values": ["pending", "running", "stopping", "stopped"]}])
    instances = [i for r in found["Reservations"] for i in r["Instances"]]
    for instance in instances:
        name = next((t["Value"] for t in instance.get("Tags", []) if t["Key"] == "Name"), "")
        print(f"{instance['InstanceId']}  {name}  {instance['InstanceType']}  {instance['State']['Name']}  {instance.get('PublicIpAddress', '')}")
    if not instances:
        print("no instance tagged", TAG)
    print(f"security group: {state.get('group', 'none recorded')}, key: {KEY_NAME if key_path().exists() else 'none recorded'}")


def down(session):
    ec2 = session.client("ec2")
    state = load_state()
    found = ec2.describe_instances(Filters=[{"Name": f"tag:{TAG}", "Values": ["1"]},
                                            {"Name": "instance-state-name", "Values": ["pending", "running", "stopping", "stopped"]}])
    ids = sorted({i["InstanceId"] for r in found["Reservations"] for i in r["Instances"]}
                 | {entry["id"] for entry in state.get("instances", {}).values()})
    if ids:
        ec2.terminate_instances(InstanceIds=ids)
        print(f"terminating {', '.join(ids)}")
        ec2.get_waiter("instance_terminated").wait(InstanceIds=ids)
    groups = ec2.describe_security_groups(Filters=[{"Name": "group-name", "Values": [GROUP_NAME]}])["SecurityGroups"]
    for group in groups:
        ec2.delete_security_group(GroupId=group["GroupId"])
        print(f"deleted security group {group['GroupId']}")
    try:
        ec2.delete_key_pair(KeyName=KEY_NAME)
        print(f"deleted key pair {KEY_NAME}")
    except Exception as error:  # noqa: BLE001 - nothing to delete is fine
        print(f"key pair: {error}")
    for path in (key_path(), STATE / "instances.json", STATE / "known_hosts"):
        path.unlink(missing_ok=True)
    print("nothing left: instances terminated (their volumes with them), group and key deleted")


# --- main ----------------------------------------------------------------------------


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--profile", help="AWS profile (default: AWS_PROFILE)")
    parser.add_argument("--region", help="AWS region (default: the profile's)")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("package", help="build the payload and the provisioning script, touching no AWS")
    up_parser = commands.add_parser("up", help="create the key pair, the security group and the instances")
    up_parser.add_argument("--arch", choices=[*ARCHES, "both"], default="both")
    up_parser.add_argument("--x86-type", default=ARCHES["x86_64"][0])
    up_parser.add_argument("--arm-type", default=ARCHES["arm64"][0])
    up_parser.add_argument("--dedicated", action="store_true", help="dedicated tenancy: no neighbours, a higher price")
    run_parser = commands.add_parser("run", help="copy, provision, run bench/local.sh and download the results")
    run_parser.add_argument("--arch", choices=[*ARCHES, "both"], default="both")
    run_parser.add_argument("--attach", action="store_true", help="wait for a run already started, then download")
    run_parser.add_argument("--poll", type=int, default=120, help="seconds between progress checks")
    run_parser.add_argument("local_args", nargs=argparse.REMAINDER,
                            help="arguments for bench/local.sh after --, default --all --cores auto --regime web")
    commands.add_parser("status", help="list what is up")
    commands.add_parser("down", help="terminate the instances, delete the group and the key")
    args = parser.parse_args()

    if args.command == "package":
        package()
        return
    session = session_for(args)
    if args.command == "up":
        arches = list(ARCHES) if args.arch == "both" else [args.arch]
        up(session, arches, {"x86_64": args.x86_type, "arm64": args.arm_type}, args.dedicated)
    elif args.command == "run":
        arches = list(ARCHES) if args.arch == "both" else [args.arch]
        local_args = [a for a in args.local_args if a != "--"] or ["--all", "--cores", "auto", "--regime", "web"]
        run(session, arches, local_args, args.attach, args.poll)
    elif args.command == "status":
        status(session)
    elif args.command == "down":
        down(session)


if __name__ == "__main__":
    main()
