#!/usr/bin/env python3
"""Build, deploy, run and tear down colbinbench on AWS Lambda.

One function per corpus and architecture, 4 GB each, named
colbin-bench-<corpus>-<arch>. The corpora are the four official datasets:

    tpch         TPC-H SF 0.1, all eight tables
    tpcds        TPC-DS SF 0.1, all 24 tables
    publicbi     the 46 Public BI tables
    schemastore  27 SchemaStore documents, typed and dynamic

Each function has its own layer, colbin-bench-data-<corpus>, holding the
corpus as one zstd-compressed tar that the function unpacks to /tmp/data the
first time an instance runs. The architecture is x86_64 unless --arch says
arm64 (Graviton) or both.

    lambda/deploy.py --profile NAME build      # binaries and zips, no AWS
    lambda/deploy.py --profile NAME deploy     # build, then role, layers, functions
    lambda/deploy.py --profile NAME invoke     # run the four corpora, save benchstat files
    lambda/deploy.py --profile NAME invoke --corpus publicbi
    lambda/deploy.py --profile NAME destroy    # delete everything deploy made

Run from anywhere; paths are relative to the repository. The build uses the
go.work beside go.mod when there is one, so it measures the colbin checkout in
../colbin rather than the published tag. See lambda/README.md.
"""

import argparse
import datetime
import hashlib
import itertools
import json
import os
import pathlib
import shutil
import subprocess
import sys
import time
import zipfile

import boto3
import botocore.config
import botocore.exceptions

REPO = pathlib.Path(__file__).resolve().parent.parent
BUILD = REPO / "lambda" / "build"
RESULTS = REPO / "lambda" / "results"
DATA = REPO / "datasets" / "data"

PREFIX = "colbin-bench"
ROLE = PREFIX + "-lambda"
MEMORY_MB = 4096
TIMEOUT_S = 900
# Calls that may land on another CPU than a run's first before the run starts over,
# and runs that may start over before invoke gives up.
CALL_TRIES = 3
RUN_TRIES = 3
# The unpacked corpus is up to 310 MB; /tmp defaults to 512.
EPHEMERAL_MB = 1024

# Lambda's architecture name to Go's.
ARCHES = {
    "arm64": "arm64",
    "x86_64": "amd64",
}

# colbinbench builds that differ only in which colbin kernels they compile. Only
# asm counts and is shipped: a purego build added 60 MB to a function that has
# 250 MB for itself and its layer. Add ("purego": (["-tags", "purego"], {})) back
# to work on the kernels.
VARIANTS = {
    "asm": ([], {}),
}

# What each corpus's layer holds: paths under datasets/data, as globs. Only the
# files the cases read; SchemaStore's checkout is 289 MB for 108 KB of documents.
CORPORA = {
    "tpch": ["tpch/sf0.1/*.tbl"],
    "tpcds": ["tpcds/sf0.1/*.csv"],
    "publicbi": ["publicbi/*.csv"],
    "schemastore": ["schemastore/binary-json-size-benchmark/benchmark/*/document.json"],
}


def function_name(corpus, arch):
    return f"{PREFIX}-{corpus}-{arch}"


def layer_name(corpus):
    return f"{PREFIX}-data-{corpus}"


# --- build -----------------------------------------------------------------


def go_build(output, package, goarch, flags=(), env=None):
    """Builds a static linux binary of package for goarch. Everything is pure Go
    (zstd is klauspost/compress), so CGO_ENABLED=0 cross-compiles to either
    architecture and the binary needs nothing from the Lambda image."""
    environment = dict(os.environ, GOOS="linux", GOARCH=goarch, CGO_ENABLED="0")
    environment.update(env or {})
    command = ["go", "build", "-trimpath", *flags, "-ldflags", "-s -w", "-o", str(output), package]
    print("  " + " ".join(f"{k}={environment[k]}" for k in env or {}) + " " + " ".join(command[2:]))
    subprocess.run(command, cwd=REPO, env=environment, check=True)


def build(arches, corpora):
    """Builds the zips of the functions for arches and of the layers for
    corpora, and returns their paths: arch, and (corpus, "layer"), to a path."""
    for tool in ("go", "zstd"):
        if not shutil.which(tool):
            sys.exit(f"{tool} is not on PATH")
    files = {corpus: sorted(path for pattern in patterns for path in DATA.glob(pattern))
             for corpus, patterns in CORPORA.items() if corpus in corpora}
    for corpus, found in files.items():
        if not found:
            sys.exit(f"{corpus}: nothing under datasets/data; run datasets/{corpus}/fetch.sh")
    if (REPO / "go.work").exists():
        print("go.work is present: colbin comes from the checkout it points at, not the tag in go.mod")

    shutil.rmtree(BUILD, ignore_errors=True)
    BUILD.mkdir(parents=True)
    zips = {}
    for arch in arches:
        goarch = ARCHES[arch]
        print(f"{arch}:")
        staging = BUILD / arch
        staging.mkdir()
        go_build(staging / "bootstrap", "./lambda/bootstrap", goarch)
        for variant, (flags, env) in VARIANTS.items():
            go_build(staging / f"colbinbench-{variant}", "./cmd/colbinbench", goarch, flags, env)
        zips[arch] = write_zip(BUILD / f"function-{arch}.zip",
                               [(path, path.name) for path in sorted(staging.iterdir())])

    # A layer unpacks under /opt: data.tar.zst lands at /opt/data.tar.zst, and
    # the function unpacks it, keeping the paths relative to datasets/data.
    for corpus, found in files.items():
        tarball = BUILD / f"{corpus}.tar.zst"
        archive(tarball, found)
        zips[corpus, "layer"] = write_zip(BUILD / f"layer-{corpus}.zip", [(tarball, "data.tar.zst")])
    for name, path in zips.items():
        print(f"{name}: {path.relative_to(REPO)} {path.stat().st_size / 1e6:.1f} MB")
    return zips


def archive(path, files):
    """Writes files, relative to datasets/data, as a zstd-compressed tar."""
    names = [str(file.relative_to(DATA)) for file in files]
    tar = subprocess.Popen(["tar", "-C", str(DATA), "-cf", "-", *names], stdout=subprocess.PIPE)
    subprocess.run(["zstd", "-19", "-T0", "-q", "-f", "-o", str(path)], stdin=tar.stdout, check=True)
    tar.stdout.close()
    if tar.wait():
        sys.exit("tar failed")


def write_zip(path, entries):
    """Writes entries, (file, name in the zip), executables kept executable."""
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for source, name in entries:
            info = zipfile.ZipInfo(name, date_time=(2026, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = (0o755 if os.access(source, os.X_OK) else 0o644) << 16
            archive.writestr(info, source.read_bytes())
    return path


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as file:
        for chunk in iter(lambda: file.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


# --- deploy ----------------------------------------------------------------


def ensure_role(iam):
    """Returns the functions' role ARN, creating the role the first time."""
    try:
        return iam.get_role(RoleName=ROLE)["Role"]["Arn"]
    except iam.exceptions.NoSuchEntityException:
        pass
    print(f"creating role {ROLE}")
    trust = {
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Principal": {"Service": "lambda.amazonaws.com"},
            "Action": "sts:AssumeRole",
        }],
    }
    arn = iam.create_role(RoleName=ROLE, AssumeRolePolicyDocument=json.dumps(trust),
                          Description="colbinbench on Lambda: logs only")["Role"]["Arn"]
    # Logs and nothing else: the functions read their layer and /var/task.
    iam.attach_role_policy(RoleName=ROLE,
                           PolicyArn="arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole")
    return arn


def ensure_layer(lam, corpus, path):
    """Returns the layer version ARN for the corpus's path, publishing only on a change."""
    digest = sha256(path)
    versions = lam.list_layer_versions(LayerName=layer_name(corpus)).get("LayerVersions", [])
    for version in versions:
        if digest in version.get("Description", ""):
            print(f"layer {layer_name(corpus)} v{version['Version']} is current")
            return version["LayerVersionArn"]
    print(f"publishing layer {layer_name(corpus)} ({path.stat().st_size / 1e6:.1f} MB)")
    published = lam.publish_layer_version(
        LayerName=layer_name(corpus),
        Description=f"{corpus}, zstd tar; sha256 {digest}",
        Content={"ZipFile": path.read_bytes()},
        CompatibleArchitectures=list(ARCHES),
        CompatibleRuntimes=["provided.al2023"],
    )
    return published["LayerVersionArn"]


def ensure_function(lam, corpus, arch, role, layer, path):
    """Creates the corpus and arch's function, or brings its code and settings up to date."""
    name = function_name(corpus, arch)
    settings = dict(
        Role=role,
        Handler="bootstrap",
        Runtime="provided.al2023",
        MemorySize=MEMORY_MB,
        Timeout=TIMEOUT_S,
        Layers=[layer],
        EphemeralStorage={"Size": EPHEMERAL_MB},
        Environment={"Variables": {"CORPUS": corpus}},
        Description=f"colbinbench, {corpus} on {arch}: colbin against msgpack and vtprotobuf",
    )
    code = path.read_bytes()
    try:
        lam.get_function(FunctionName=name)
    except lam.exceptions.ResourceNotFoundException:
        print(f"creating function {name}")
        # A role just created takes a few seconds before Lambda may assume it.
        for attempt in range(10):
            try:
                lam.create_function(FunctionName=name, Architectures=[arch],
                                    Code={"ZipFile": code}, **settings)
                break
            except lam.exceptions.InvalidParameterValueException as error:
                if "assume" not in str(error) or attempt == 9:
                    raise
                time.sleep(5)
        lam.get_waiter("function_active_v2").wait(FunctionName=name)
        return
    print(f"updating function {name}")
    lam.update_function_configuration(FunctionName=name, **settings)
    lam.get_waiter("function_updated_v2").wait(FunctionName=name)
    lam.update_function_code(FunctionName=name, ZipFile=code, Architectures=[arch])
    lam.get_waiter("function_updated_v2").wait(FunctionName=name)


def deploy(session, arches, corpora):
    zips = build(arches, corpora)
    iam, lam = session.client("iam"), session.client("lambda")
    role = ensure_role(iam)
    names = []
    for corpus in corpora:
        layer = ensure_layer(lam, corpus, zips[corpus, "layer"])
        for arch in arches:
            ensure_function(lam, corpus, arch, role, layer, zips[arch])
            names.append(function_name(corpus, arch))
    print("deployed: " + ", ".join(names))


# --- invoke ----------------------------------------------------------------


class CPUChanged(Exception):
    """A call landed on a different CPU than the run's first."""


def invoke(session, event, arches, corpora):
    """Runs the corpora one after another and writes one benchstat file per
    corpus, architecture and variant under lambda/results/<time>/.

    All of an architecture's calls, across its corpora's functions, have to land
    on the same CPU, since Lambda does not say which one a function gets and the
    corpora are only comparable on one. The first call sets the CPU. A call that
    reports another is thrown away and made again, up to CALL_TRIES times in all;
    when none lands on the first CPU the whole run starts over from its first
    call, up to RUN_TRIES times, and whatever the failed run wrote is deleted.
    Nothing is run in parallel: the calls are the only load on the instances.

    One invocation per case rather than one for all of them. A synchronous
    invoke holds an HTTP connection open with nothing on it until the function
    returns: long enough, a NAT or firewall between here and AWS drops the idle
    connection, after which the function finishes and its answer goes nowhere.
    A case is seconds to a couple of minutes. The case list is the function's
    own: the corpus's timed cases in the matrix."""
    config = botocore.config.Config(read_timeout=TIMEOUT_S + 60, retries={"max_attempts": 0})
    lam = session.client("lambda", config=config)
    out = RESULTS / datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    out.mkdir(parents=True)
    explicit = event.pop("cases", None)

    def call(name, payload):
        answer = lam.invoke(FunctionName=name, Payload=json.dumps(payload).encode())
        body = json.loads(answer["Payload"].read())
        if "FunctionError" in answer:
            raise RuntimeError(f"{name}: {body}")
        return body

    def recycle(corpus, arch):
        """Discards the function's warm instances, so that its next call starts
        a new one, on whatever CPU Lambda picks: a retry on the same warm
        instance could only report the same CPU again. Changing a setting is
        what makes Lambda do that."""
        name = function_name(corpus, arch)
        lam.update_function_configuration(FunctionName=name, Environment={"Variables": {
            "CORPUS": corpus, "RECYCLED": str(time.time_ns())}})
        lam.get_waiter("function_updated_v2").wait(FunctionName=name)

    def run(arch, attempt):
        """One run of every corpus on arch; returns its CPU, or raises CPUChanged."""
        staging = out / f".{arch}-run{attempt}"
        staging.mkdir()
        if attempt > 1:
            # A new run needs a new first CPU, not the one the last run kept.
            for corpus in corpora:
                recycle(corpus, arch)
        cpu = None

        def checked(label, corpus, payload):
            name = function_name(corpus, arch)
            nonlocal cpu
            for tries in range(1, CALL_TRIES + 1):
                body = call(name, payload)
                if cpu is None:
                    cpu = body["cpu"]
                if body["cpu"] == cpu:
                    return body
                print(f"  {label}: landed on {body['cpu']}, not {cpu} (try {tries}/{CALL_TRIES})", flush=True)
                if tries < CALL_TRIES:
                    recycle(corpus, arch)
            raise CPUChanged(label)

        for corpus in corpora:
            cases = explicit or checked(f"{corpus}/{arch} list", corpus, {"list": True})["cases"]
            for index, case in enumerate(cases):
                label = f"{corpus}/{arch}"
                print(f"{label}: {case} ({index + 1}/{len(cases)})", flush=True)
                body = checked(f"{label} {case}", corpus, {**event, "cases": [case]})
                for variant, text in body["variants"].items():
                    path = staging / f"{corpus}-{arch}-{variant}.txt"
                    if path.exists():
                        # The goos/goarch/pkg/cpu header once, at the top; each
                        # case's regime line is kept, as it applies to the case's
                        # benchmark lines below it.
                        text = "".join(line for line in text.splitlines(keepends=True)
                                       if line.startswith(("Benchmark", "regime:")))
                    with open(path, "a") as file:
                        file.write(text)
        for path in staging.iterdir():
            path.rename(out / path.name)
        staging.rmdir()
        return cpu

    summary = []
    for arch in arches:
        started = time.monotonic()
        for attempt in range(1, RUN_TRIES + 1):
            try:
                cpu = run(arch, attempt)
                break
            except CPUChanged as error:
                shutil.rmtree(out / f".{arch}-run{attempt}")
                print(f"{arch}: run {attempt}/{RUN_TRIES} discarded, {error} never landed on the first CPU", flush=True)
        else:
            sys.exit(f"{arch}: no run kept one CPU in {RUN_TRIES} tries; nothing kept for it")
        summary.append(f"{arch}: {cpu}, {time.monotonic() - started:.0f} s")
    print("\n".join(summary))
    print(f"results in {out.relative_to(REPO)}")


# --- destroy ---------------------------------------------------------------


def destroy(session):
    iam, lam, logs = session.client("iam"), session.client("lambda"), session.client("logs")
    for corpus, arch in itertools.product(CORPORA, ARCHES):
        name = function_name(corpus, arch)
        try:
            lam.delete_function(FunctionName=name)
            print(f"deleted function {name}")
        except lam.exceptions.ResourceNotFoundException:
            pass
        try:
            logs.delete_log_group(logGroupName=f"/aws/lambda/{name}")
            print(f"deleted log group /aws/lambda/{name}")
        except logs.exceptions.ResourceNotFoundException:
            pass
    for corpus in CORPORA:
        for version in lam.list_layer_versions(LayerName=layer_name(corpus)).get("LayerVersions", []):
            lam.delete_layer_version(LayerName=layer_name(corpus), VersionNumber=version["Version"])
            print(f"deleted layer {layer_name(corpus)} v{version['Version']}")
    try:
        for policy in iam.list_attached_role_policies(RoleName=ROLE)["AttachedPolicies"]:
            iam.detach_role_policy(RoleName=ROLE, PolicyArn=policy["PolicyArn"])
        iam.delete_role(RoleName=ROLE)
        print(f"deleted role {ROLE}")
    except iam.exceptions.NoSuchEntityException:
        pass


# --- main ------------------------------------------------------------------


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--profile", help="AWS profile (default: AWS_PROFILE)")
    parser.add_argument("--region", help="AWS region (default: the profile's)")
    commands = parser.add_subparsers(dest="command", required=True)
    selecting = []
    for name, text in (("build", "build the binaries and zips, touching no AWS"),
                       ("deploy", "build, then create or update the role, layers and functions"),
                       ("invoke", "run the benchmark on the corpora's functions")):
        command = commands.add_parser(name, help=text)
        command.add_argument("--arch", choices=[*ARCHES, "both"], default="x86_64")
        command.add_argument("--corpus", help=f"comma-separated of {','.join(CORPORA)} (default all)")
        selecting.append(command)
    run = commands.choices["invoke"]
    run.add_argument("--cases", help="comma-separated, default the corpus's timed cases")
    run.add_argument("--variants", help="comma-separated, default asm, the only variant built")
    run.add_argument("--formats", help="default colbin,msgpack,vtprotobuf")
    run.add_argument("--ops", help="default encode,decode-fresh,decode-reuse")
    run.add_argument("--count", type=int, help="samples per benchmark, default 5")
    run.add_argument("--sample", help="minimum time per sample, default 100ms")
    commands.add_parser("destroy", help="delete the functions, layer, role and logs")
    args = parser.parse_args()

    if args.command == "destroy":
        arches = corpora = None
    else:
        arches = list(ARCHES) if args.arch == "both" else [args.arch]
        corpora = args.corpus.split(",") if args.corpus else list(CORPORA)
        unknown = [name for name in corpora if name not in CORPORA]
        if unknown:
            sys.exit(f"unknown corpus {', '.join(unknown)}")
    if args.command == "build":
        build(arches, corpora)
        return
    session = boto3.Session(profile_name=args.profile, region_name=args.region)
    try:
        identity = session.client("sts").get_caller_identity()
    except botocore.exceptions.BotoCoreError as error:
        sys.exit(f"AWS credentials: {error}")
    print(f"account {identity['Account']}, {identity['Arn']}, region {session.region_name}")

    if args.command == "deploy":
        deploy(session, arches, corpora)
    elif args.command == "invoke":
        event = {}
        for key in ("cases", "variants"):
            if getattr(args, key):
                event[key] = getattr(args, key).split(",")
        for key in ("formats", "ops", "count", "sample"):
            if getattr(args, key):
                event[key] = getattr(args, key)
        invoke(session, event, arches, corpora)
    elif args.command == "destroy":
        destroy(session)


if __name__ == "__main__":
    main()
