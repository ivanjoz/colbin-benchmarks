#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10,<3.14"
# dependencies = ["oci>=2.140"]
# ///
"""Run reproduce.sh in OCI Container Instances, x86 by default.

Nothing here measures anything. Each container clones this repository from
GitHub at one pushed commit, runs `./reproduce.sh` there (datasets fetched and
checked against the manifest inside the container, as on any clean machine)
and uploads its run directory to an Object Storage bucket through a write-only
pre-authenticated URL. The container has no inbound access and no credentials.

    cloud/ocibench.py shapes                  # what Container Instances offers here
    cloud/ocibench.py up                      # network (egress only) and the bucket
    cloud/ocibench.py run                     # one x86 container, at HEAD
    cloud/ocibench.py run --label tpch -- --cases 'tpch/*'   # one dataset
    cloud/ocibench.py status                  # each run's state, and its progress.log
    cloud/ocibench.py fetch [--partial]       # download results to results/cloud/oci/
    cloud/ocibench.py logs --arch x86         # the container's own output, for a run that died early
    cloud/ocibench.py down [--bucket]         # delete the containers and the network (and the bucket)

Default machines, chosen so that the bulk regime fits (TPC-H nested and
normalized need over 5 GB, retried under a 9 GiB GOMEMLIMIT) and reproduce.sh
has two cores to measure on and the rest for the system and compression:

    x86   CI.Standard.E4.Flex   4 OCPU (AMD EPYC, 4 cores = 8 threads), 32 GB
    arm   CI.Standard.A1.Flex   4 OCPU (Ampere Altra, 4 cores), 32 GB

`run` refuses a dirty tree or a commit that is not on GitHub: the container
clones from there, so what runs is exactly that commit. Arguments after `--`
go to reproduce.sh (`run -- --regime web`). A container cannot set the
frequency governor or turn turbo off; reproduce.sh --vm records that the host
controls the clock, and colbinbench's probe records the clock it ran at.

Nothing depends on the machine that started a run. The container uploads its
progress and its results so far every ten minutes, and `status` and `fetch`
find runs in the bucket and among the tagged instances, not in a local file:
the laptop can be off for the whole run.

Authentication is the OCI SDK's config file (~/.oci/config, --profile). What
`up` creates is in cloud/state/oci.json (not committed) and tagged
colbin-bench-oci, which `down` also finds; `run` and `down` need that file. Needs uv (the shebang installs the
SDK in a throwaway environment), or `pip install oci` and `python3 cloud/ocibench.py`.
"""

import argparse
import datetime
import json
import pathlib
import shutil
import subprocess
import sys
import tarfile
import time

REPO = pathlib.Path(__file__).resolve().parent.parent
STATE = REPO / "cloud" / "state" / "oci.json"
RESULTS = REPO / "results" / "cloud" / "oci"
GITHUB = "https://github.com/ivanjoz/colbin-benchmarks"

TAG = "colbin-bench-oci"
BUCKET = TAG
CIDR = "10.77.0.0/16"
SUBNET_CIDR = "10.77.0.0/24"
# The Go of go.mod's toolchain line (golang:1.27.0-trixie), pinned by the
# multi-arch index digest. Digest only: Container Instances does not pull a
# reference with both a tag and a digest, and says the network is at fault.
IMAGE = "docker.io/library/golang@sha256:df98008ecd2b0ecc9f0a94d1b07e3564a9c92b555369b33d9b5f60d0765b2db7"
PAR_DAYS = 3

ARCHES = {
    "x86": {"shape": "CI.Standard.E4.Flex", "ocpus": 4, "memory": 32},
    "arm": {"shape": "CI.Standard.A1.Flex", "ocpus": 4, "memory": 32},
}

# The container's first command. Everything after the clone is in the commit.
BOOTSTRAP = (
    'set -e; git clone -q "$REPO_URL" /w; cd /w; git checkout -q "$COMMIT"; '
    "exec bash cloud/oci-entry.sh"
)


def log(message):
    print(f"[{datetime.datetime.now():%H:%M:%S}] {message}", flush=True)


# ---- OCI ----------------------------------------------------------------------


class Cloud:
    def __init__(self, profile, compartment):
        try:
            import oci
        except ImportError:
            sys.exit("ocibench: the OCI SDK is missing: run with uv (./cloud/ocibench.py), or pip install oci")
        self.oci = oci
        self.config = oci.config.from_file(profile_name=profile)
        oci.config.validate_config(self.config)
        self.compartment = compartment or self.config["tenancy"]
        self.region = self.config["region"]
        self.identity = oci.identity.IdentityClient(self.config)
        self.network = oci.core.VirtualNetworkClient(self.config)
        self.storage = oci.object_storage.ObjectStorageClient(self.config)
        self.containers = oci.container_instances.ContainerInstanceClient(self.config)
        self.namespace = self.storage.get_namespace().data

    def wait(self, client, response, state, seconds=600):
        return self.oci.wait_until(client, response, "lifecycle_state", state, max_wait_seconds=seconds).data

    def availability_domain(self):
        return self.identity.list_availability_domains(self.compartment).data[0].name

    def shapes(self, availability_domain):
        return self.containers.list_container_instance_shapes(
            self.compartment, availability_domain=availability_domain).data.items


def load_state():
    return json.loads(STATE.read_text()) if STATE.exists() else {}


def save_state(state):
    STATE.parent.mkdir(parents=True, exist_ok=True)
    STATE.write_text(json.dumps(state, indent=2) + "\n")


def tags(extra=None):
    return {TAG: "1", **(extra or {})}


# ---- commands -------------------------------------------------------------------


def shapes(cloud):
    ad = cloud.availability_domain()
    print(f"{cloud.region}, {ad}:")
    for shape in cloud.shapes(ad):
        ocpus = shape.ocpu_options
        memory = shape.memory_options
        print(f"  {shape.name:24} {shape.processor_description or '':40} "
              f"OCPU {ocpus.min:g}-{ocpus.max:g}, memory {memory.min_in_gbs:g}-{memory.max_in_gbs:g} GB")


def up(cloud):
    """A VCN with an internet gateway and a public subnet whose security list
    allows egress only, and a private bucket."""
    models = cloud.oci.core.models
    state = load_state()
    net = cloud.network
    if "subnet" not in state:
        log(f"network in {cloud.region}")
        vcn = net.create_vcn(models.CreateVcnDetails(
            compartment_id=cloud.compartment, cidr_blocks=[CIDR], display_name=TAG, freeform_tags=tags(),
            dns_label="colbinbench")).data
        state["vcn"] = vcn.id
        save_state(state)
        cloud.wait(net, net.get_vcn(vcn.id), "AVAILABLE")
        gateway = net.create_internet_gateway(models.CreateInternetGatewayDetails(
            compartment_id=cloud.compartment, vcn_id=vcn.id, is_enabled=True, display_name=TAG,
            freeform_tags=tags())).data
        state["gateway"] = gateway.id
        save_state(state)
        cloud.wait(net, net.get_internet_gateway(gateway.id), "AVAILABLE")
        net.update_route_table(vcn.default_route_table_id, models.UpdateRouteTableDetails(route_rules=[
            models.RouteRule(destination="0.0.0.0/0", destination_type="CIDR_BLOCK", network_entity_id=gateway.id)]))
        state["route_table"] = vcn.default_route_table_id
        security = net.create_security_list(models.CreateSecurityListDetails(
            compartment_id=cloud.compartment, vcn_id=vcn.id, display_name=TAG, freeform_tags=tags(),
            ingress_security_rules=[],
            egress_security_rules=[models.EgressSecurityRule(destination="0.0.0.0/0", protocol="all")])).data
        state["security_list"] = security.id
        save_state(state)
        cloud.wait(net, net.get_security_list(security.id), "AVAILABLE")
        subnet = net.create_subnet(models.CreateSubnetDetails(
            compartment_id=cloud.compartment, vcn_id=vcn.id, cidr_block=SUBNET_CIDR, display_name=TAG,
            route_table_id=vcn.default_route_table_id, security_list_ids=[security.id],
            prohibit_public_ip_on_vnic=False, freeform_tags=tags(), dns_label="bench")).data
        state["subnet"] = subnet.id
        save_state(state)
        cloud.wait(net, net.get_subnet(subnet.id), "AVAILABLE")
    if not state.get("bucket"):
        log(f"bucket {BUCKET}")
        storage_models = cloud.oci.object_storage.models
        try:
            cloud.storage.create_bucket(cloud.namespace, storage_models.CreateBucketDetails(
                name=BUCKET, compartment_id=cloud.compartment, public_access_type="NoPublicAccess",
                freeform_tags=tags()))
        except cloud.oci.exceptions.ServiceError as error:
            if error.status != 409:  # 409: it exists already
                raise
        state["bucket"] = BUCKET
        save_state(state)
    log("up: network and bucket ready")


def pushed_commit():
    """HEAD, if the tree is clean and GitHub has it."""
    def git(*args):
        return subprocess.run(["git", "-C", str(REPO), *args], check=True, capture_output=True, text=True).stdout.strip()
    if git("status", "--porcelain"):
        sys.exit("ocibench: uncommitted changes; the container clones from GitHub, so commit and push first")
    commit = git("rev-parse", "HEAD")
    git("fetch", "-q", "origin")
    if not git("branch", "-r", "--contains", commit):
        sys.exit(f"ocibench: {commit[:7]} is not on origin; push it first")
    return commit


def run(cloud, arches, ocpus, memory, shapes_override, repro_args, label):
    state = load_state()
    if "subnet" not in state:
        sys.exit("ocibench: nothing is up; run `up` first")
    commit = pushed_commit()
    ad = cloud.availability_domain()
    offered = {shape.name for shape in cloud.shapes(ad)}
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    models = cloud.oci.container_instances.models
    storage_models = cloud.oci.object_storage.models
    state.setdefault("runs", {})
    for arch in arches:
        spec = dict(ARCHES[arch])
        spec["shape"] = shapes_override.get(arch, spec["shape"])
        spec["ocpus"] = ocpus or spec["ocpus"]
        spec["memory"] = memory or spec["memory"]
        if spec["shape"] not in offered:
            sys.exit(f"ocibench: {spec['shape']} is not offered in {ad}; `shapes` lists what is")
        name = f"oci-{arch}-{label + '-' if label else ''}{stamp}"
        par = cloud.storage.create_preauthenticated_request(cloud.namespace, BUCKET,
            storage_models.CreatePreauthenticatedRequestDetails(
                name=name, access_type="AnyObjectWrite",
                time_expires=datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=PAR_DAYS))).data
        par_url = f"https://objectstorage.{cloud.region}.oraclecloud.com{par.access_uri}"
        log(f"{arch}: {spec['shape']}, {spec['ocpus']} OCPU, {spec['memory']} GB, commit {commit[:7]} → {name}")
        instance = cloud.containers.create_container_instance(models.CreateContainerInstanceDetails(
            compartment_id=cloud.compartment, availability_domain=ad, display_name=name,
            shape=spec["shape"],
            shape_config=models.CreateContainerInstanceShapeConfigDetails(
                ocpus=spec["ocpus"], memory_in_gbs=spec["memory"]),
            vnics=[models.CreateContainerVnicDetails(subnet_id=state["subnet"], is_public_ip_assigned=True)],
            container_restart_policy="NEVER",
            containers=[models.CreateContainerDetails(
                display_name="reproduce", image_url=IMAGE,
                command=["bash", "-c", BOOTSTRAP],
                environment_variables={
                    "REPO_URL": GITHUB, "COMMIT": commit, "NAME": name, "PAR": par_url,
                    "REPRO_ARGS": " ".join(repro_args)},
            )],
            freeform_tags=tags({"run": name}))).data
        state["runs"][name] = {"arch": arch, "instance": instance.id, "par": par.id, "commit": commit,
                               "started": stamp, **spec, "args": repro_args}
        save_state(state)
    log("started; `status` shows progress, `fetch` downloads what has finished")


def object_text(cloud, name):
    try:
        return cloud.storage.get_object(cloud.namespace, BUCKET, name).data.content.decode()
    except cloud.oci.exceptions.ServiceError as error:
        if error.status == 404:
            return None
        raise


SUFFIXES = (".progress.log", ".partial.tar.gz", ".results.tar.gz", ".exit")


def runs(cloud):
    """Every run, from the bucket and the tagged container instances, not from
    the state file: what one machine started, any other can follow."""
    found = {}
    for item in cloud.containers.list_container_instances(cloud.compartment).data.items:
        name = (item.freeform_tags or {}).get("run")
        if TAG in (item.freeform_tags or {}) and name:
            found[name] = item.lifecycle_state.lower()
            if found[name] == "failed":
                found[name] += failure(cloud, item.id)
    start = None
    while True:
        page = cloud.storage.list_objects(cloud.namespace, BUCKET, start=start).data
        for item in page.objects:
            for suffix in SUFFIXES:
                if item.name.endswith(suffix):
                    found.setdefault(item.name[: -len(suffix)], "gone")
        if not page.next_start_with:
            break
        start = page.next_start_with
    return dict(sorted(found.items()))


def failure(cloud, instance_id):
    """Why OCI gave up on an instance, from its work requests: ': <message>'."""
    for request in cloud.containers.list_work_requests(cloud.compartment, resource_id=instance_id).data.items:
        for error in cloud.containers.list_work_request_errors(request.id).data.items:
            return f": {error.message}"
    return ""


def status(cloud):
    for name, lifecycle in runs(cloud).items():
        exit_code = object_text(cloud, f"{name}.exit")
        fetched = (RESULTS / name).exists()
        print(f"{name}: container {lifecycle}"
              + (f", reproduce.sh exited {exit_code.strip()}" if exit_code else ", running or not started")
              + (", fetched" if fetched else ""))
        progress = object_text(cloud, f"{name}.progress.log")
        for line in (progress or "  (no progress.log yet: it is uploaded every ten minutes)").splitlines()[-4:]:
            print(f"    {line}")


def fetch(cloud, partial):
    """Finished runs into results/cloud/oci/<name>/; with partial, also what
    an unfinished one has uploaded so far, into <name>.partial/."""
    for name in runs(cloud):
        finished = object_text(cloud, f"{name}.exit") is not None
        if finished:
            target, archive_name = RESULTS / name, f"{name}.results.tar.gz"
        elif partial:
            target, archive_name = RESULTS / f"{name}.partial", f"{name}.partial.tar.gz"
        else:
            log(f"{name}: not finished (`fetch --partial` takes what it has uploaded)")
            continue
        if finished and target.exists():
            continue
        try:
            data = cloud.storage.get_object(cloud.namespace, BUCKET, archive_name).data.content
        except cloud.oci.exceptions.ServiceError as error:
            if error.status != 404:
                raise
            log(f"{name}: no {archive_name} yet")
            continue
        scratch = RESULTS / f".{name}.unpack"
        shutil.rmtree(scratch, ignore_errors=True)
        scratch.mkdir(parents=True)
        archive = scratch / "archive.tar.gz"
        archive.write_bytes(data)
        with tarfile.open(archive) as tar:
            tar.extractall(scratch, filter="data")
        shutil.rmtree(target, ignore_errors=True)
        (scratch / name).rename(target)
        shutil.rmtree(scratch)
        log(f"{name}: {'results' if finished else 'partial results'} in {target.relative_to(REPO)}")


def logs(cloud, arch):
    state = load_state()
    runs = [info for info in state.get("runs", {}).values() if info["arch"] == arch]
    if not runs:
        sys.exit(f"ocibench: no {arch} run")
    instance = cloud.containers.get_container_instance(runs[-1]["instance"]).data
    for container in instance.containers:
        try:
            output = cloud.containers.retrieve_logs(container.container_id).data
        except cloud.oci.exceptions.ServiceError as error:
            sys.exit(f"ocibench: no logs ({error.message}){failure(cloud, instance.id)}")
        sys.stdout.write(output.content.decode(errors="replace") if hasattr(output, "content") else str(output))


def down(cloud, bucket):
    state = load_state()
    net = cloud.network
    for name, info in list(state.get("runs", {}).items()):
        try:
            cloud.containers.delete_container_instance(info["instance"])
            log(f"{name}: container instance deleted")
        except cloud.oci.exceptions.ServiceError as error:
            if error.status != 404:
                raise
        try:
            cloud.storage.delete_preauthenticated_request(cloud.namespace, BUCKET, info["par"])
        except cloud.oci.exceptions.ServiceError as error:
            if error.status != 404:
                raise
    # Anything tagged that the state file lost.
    for item in cloud.containers.list_container_instances(cloud.compartment).data.items:
        if TAG in (item.freeform_tags or {}) and item.lifecycle_state not in ("DELETED", "DELETING"):
            cloud.containers.delete_container_instance(item.id)
            log(f"{item.display_name}: container instance deleted (tagged, not in the state file)")
    # The subnet cannot go while a container's VNIC is still in it.
    for info in state.get("runs", {}).values():
        try:
            cloud.oci.wait_until(cloud.containers, cloud.containers.get_container_instance(info["instance"]),
                                 "lifecycle_state", "DELETED", max_wait_seconds=900, succeed_on_not_found=True)
        except cloud.oci.exceptions.ServiceError as error:
            if error.status != 404:
                raise
    state["runs"] = {}
    save_state(state)
    steps = [
        ("subnet", lambda id: net.delete_subnet(id)),
        ("security_list", lambda id: net.delete_security_list(id)),
        ("route_table", lambda id: net.update_route_table(
            id, cloud.oci.core.models.UpdateRouteTableDetails(route_rules=[]))),
        ("gateway", lambda id: net.delete_internet_gateway(id)),
        ("vcn", lambda id: net.delete_vcn(id)),
    ]
    for key, delete in steps:
        if key not in state:
            continue
        for attempt in range(20):
            try:
                delete(state[key])
                break
            except cloud.oci.exceptions.ServiceError as error:
                if error.status == 404:
                    break
                if error.status != 409 or attempt == 19:  # 409: still in use, wait
                    raise
                time.sleep(15)
        log(f"{key} deleted")
        del state[key]
        save_state(state)
    if bucket and state.get("bucket"):
        for item in cloud.storage.list_objects(cloud.namespace, BUCKET).data.objects:
            cloud.storage.delete_object(cloud.namespace, BUCKET, item.name)
        cloud.storage.delete_bucket(cloud.namespace, BUCKET)
        del state["bucket"]
        save_state(state)
        log("bucket deleted")
    elif state.get("bucket"):
        log(f"bucket {BUCKET} kept, with the results in it; `down --bucket` deletes it")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--profile", default="DEFAULT", help="the ~/.oci/config profile")
    parser.add_argument("--compartment", help="compartment OCID (default: the tenancy's root)")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("shapes", help="the container shapes offered in the first availability domain")
    commands.add_parser("up", help="the network (egress only) and the bucket")
    run_parser = commands.add_parser("run", help="one container instance per architecture, at the pushed HEAD")
    run_parser.add_argument("--arch", default="x86", help="x86 (default), arm, or x86,arm")
    run_parser.add_argument("--ocpus", type=float, help="OCPUs per instance (default 4)")
    run_parser.add_argument("--memory", type=float, help="GB per instance (default 32)")
    run_parser.add_argument("--x86-shape", help=f"default {ARCHES['x86']['shape']}")
    run_parser.add_argument("--arm-shape", help=f"default {ARCHES['arm']['shape']}")
    run_parser.add_argument("--label", help="a word in the run's name, e.g. tpch")
    run_parser.add_argument("repro_args", nargs=argparse.REMAINDER, help="after --: arguments for reproduce.sh")
    commands.add_parser("status", help="each run's container state and the tail of its progress.log")
    fetch_parser = commands.add_parser("fetch", help="download finished runs to results/cloud/oci/")
    fetch_parser.add_argument("--partial", action="store_true",
                              help="also what unfinished runs uploaded so far, into <name>.partial/")
    logs_parser = commands.add_parser("logs", help="the container's own output")
    logs_parser.add_argument("--arch", default="x86")
    down_parser = commands.add_parser("down", help="delete the containers and the network")
    down_parser.add_argument("--bucket", action="store_true", help="also empty and delete the bucket")
    args = parser.parse_args()

    cloud = Cloud(args.profile, args.compartment)
    if args.command == "shapes":
        shapes(cloud)
    elif args.command == "up":
        up(cloud)
    elif args.command == "run":
        arches = [arch.strip() for arch in args.arch.split(",")]
        for arch in arches:
            if arch not in ARCHES:
                sys.exit(f"ocibench: --arch is x86, arm or x86,arm, not {arch}")
        overrides = {arch: shape for arch, shape in (("x86", args.x86_shape), ("arm", args.arm_shape)) if shape}
        repro_args = [arg for arg in args.repro_args if arg != "--"]
        run(cloud, arches, args.ocpus, args.memory, overrides, repro_args, args.label)
    elif args.command == "status":
        status(cloud)
    elif args.command == "fetch":
        fetch(cloud, args.partial)
    elif args.command == "logs":
        logs(cloud, args.arch)
    elif args.command == "down":
        down(cloud, args.bucket)


if __name__ == "__main__":
    main()
