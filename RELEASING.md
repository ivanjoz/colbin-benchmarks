# Releasing the paper

The paper goes to arXiv; the code and data that produce its numbers stay here.
What ties the two together is a git tag: every number in the paper must
regenerate from that tag with one command.

This is the checklist that makes that true. Steps marked **manual** need a person
with access to the arXiv or GitHub account; CI cannot do them.

## What gets published where

| what | where | identifier the paper cites |
|---|---|---|
| the paper | arXiv | `arXiv:YYMM.NNNNN` (arXiv also issues `10.48550/arXiv.YYMM.NNNNN`) |
| code, datasets, results | this repository | tag `paper-vN` and its commit hash |

arXiv hosts the paper and nothing else. It does not archive this repository, so
the paper has to name an exact tag and commit. A link to a branch does not
count: the branch will keep moving.

## Versions

Every arXiv version names the tag its numbers come from.

| change | arXiv | tag |
|---|---|---|
| first submission | v1 | `paper-v1` |
| text only (typos, wording, references) | v2, v3, … | unchanged: still `paper-v1` |
| any number changes | new version | new tag: `paper-v2` |

Never move or delete a published tag. A fix to the artifact gets a new tag.

## Checklist

### 0. Before the first submission (**manual**, once)

- An arXiv account, and an endorsement for the primary category if the account
  has never posted there. Endorsement can take days, so ask for it before the
  paper is finished.
- Choose the categories: suggested primary `cs.DB`, cross-listed to `cs.PF`.

### 1. colbin is at a published tag

`go.mod` requires colbin at a tag and has no `replace`. If the benchmarks start
using colbin API newer than the required tag:

1. In `github.com/ivanjoz/colbin`: tag and push the release (**manual**).
2. Here: `go get github.com/ivanjoz/colbin@vX.Y.Z && go mod tidy`.
3. Confirm in a clean clone, with no colbin checkout anywhere near it:

   ```sh
   tmp=$(mktemp -d) && git clone https://github.com/ivanjoz/colbin-benchmarks "$tmp/cb"
   cd "$tmp/cb" && GOFLAGS=-mod=readonly go test ./...
   ```

### 2. Freeze the numbers (**manual**)

1. The paper's machine is an OCI container instance (`cloud/README.md`):
   x86, `CI.Standard.E4.Flex`, 4 OCPU, 32 GB. The commit to be tagged must be
   pushed: the container clones it from GitHub. Nothing runs on the machine
   that starts it.

   ```sh
   cloud/ocibench.py up
   for d in tpch tpcds publicbi schemastore; do      # one container per dataset
     cloud/ocibench.py run --label $d -- --cases "$d/*"
   done
   cloud/ocibench.py status                 # until every reproduce.sh has exited
   cloud/ocibench.py fetch
   go run ./cmd/mergeruns -out results/paper-v1 results/cloud/oci/oci-x86-{tpch,tpcds,publicbi,schemastore}-<stamps>
   go run ./cmd/summary results/paper-v1
   ```

   The sweep (README, "The sweep") is a run of its own on the same machine,
   not merged into the paper's run:

   ```sh
   cloud/ocibench.py run --label sweep -- --cases 'sweep/*' --no-ablation --passes 1
   ```

   Its directory goes to `make -C paper SWEEP=...`; without it the paper
   leaves the sweep's figure out.

2. The second machine is a local one, measured at the same commit
   (`results/paper-v1-laptop`). It needs mains power, the `performance`
   governor, turbo off, nothing else running; `reproduce.sh` refuses to start
   otherwise and says which command fixes each. A run that stops continues
   with `--resume`. On a hybrid laptop the run must have the performance cores
   to itself, so the steps are, as root:
   - stop (mask, until the run ends) `tuned`, `tuned-ppd` and
     `power-profiles-daemon`: they turn the turbo back on;
   - turbo off, `performance` governor, sleep targets masked;
   - interrupts and the `system.slice`, `user.slice` and `init.scope` cgroups
     confined to the efficiency cores;
   - the run in its own slice, from a clean worktree at that commit:

   ```sh
   git worktree add ../cb-<commit> <commit>
   sudo systemd-run --unit=colbin-laptop --slice=bench.slice -p AllowedCPUs=0-11 \
     --uid="$(id -u)" --gid="$(id -g)" -p WorkingDirectory="$PWD/../cb-<commit>" \
     -E HOME="$HOME" -E PATH="$PATH" -E COLBIN_DATA="$PWD/datasets/data" \
     ../cb-<commit>/reproduce.sh --out "$PWD/results/paper-v1-laptop"
   ```

   On Fedora, SELinux does not let systemd execute a script under `$HOME`:
   `chcon -t bin_t ../cb-<commit>/reproduce.sh` first. `--cores auto` picks
   one hardware thread of each of the first two performance cores.

3. Read `results/paper-v1/SUMMARY.md` and `machine.txt`, and those of
   `results/paper-v1-laptop/`. Their `sizes.csv` must match line for line
   except `gob` on messages of tens of bytes. Each run is the
   paper's only if `machine.txt` has no `WARNING` line and there is no `FAILED`
   file; otherwise `tables/numbers.tex` marks it provisional and the paper
   prints that on its first page. On a cloud run, also read the probe
   (`SUMMARY.md`, "The measured cores' clock"): no case's clock should have
   moved by much more than the differences the paper reports.
4. Build the paper on those numbers, and read every blue `[check: …]` mark
   against them: each is a sentence whose wording depends on which way a number
   fell. Fix the sentence, delete the mark, and when none is left set
   `\finaltrue` in `paper/main.tex`.

   ```sh
   make -C paper RUN=../results/paper-v1 LAPTOP=../results/paper-v1-laptop
   ```

5. Commit `results/paper-v1/` and `results/paper-v1-laptop/` as `reproduce.sh` left them (`.gitignore` lets
   `results/paper-v*` through and nothing else under `results/`). Do not edit
   a number by hand, in the results or in the paper. If a number looks wrong,
   fix the code, commit, and run again: the commit `machine.txt` names has to
   be the one that produced the numbers, so this commit holds results and
   text only.

### 3. Tag the artifact (**manual**)

```sh
git tag -a paper-v1 -m "Artifact for arXiv submission v1"
git push origin paper-v1
```

CI runs on `paper-*` tags. Wait for it to pass before you submit: it checks that
the corpus still matches its manifest on a clean runner.

A git tag is not an archive: a repository can be deleted or rewritten. For a
permanent copy with a DOI, enable the repository in Zenodo's GitHub integration
(zenodo.org → GitHub, once), then create a GitHub *release* from the tag;
Zenodo archives it and issues the DOI.

### 4. Point the paper at the tag

The availability statement is the last section of the paper
(`paper/sections/07-availability.tex`). It already takes the measured commit
and the colbin version from the run. Fill in the two placeholders marked
`TODO(release)`: the full hash of the tag (`git rev-parse paper-v1`) and the
Zenodo DOI. Put the DOI in `CITATION.cff` too.

The tag names the commit that holds the results; the statement's hash and DOI
are known only after tagging, so the PDF submitted to arXiv is built from the
tag plus those two strings. That is text only, and by the rule under
[Versions](#versions) it needs no new tag.

### 5. Submit to arXiv (**manual**)

- `make -C paper arxiv` packs what arXiv needs into `paper/build/arxiv.tar.gz`:
  `main.tex`, `sections/`, `tables/` (the generated tables, numbers and figure
  data) and `main.bbl`. Upload that, not the PDF: arXiv declines a PDF made
  from TeX and compiles the source itself.
- **What arXiv announces is archived for good, source included, and anyone can
  download it.** So the archive is not a copy of `paper/`: the target refuses to
  pack while a `[check]` mark, a `TODO`, a placeholder or `\finalfalse` is left,
  or while `tables/` is a provisional run's; it drops whole-line TeX comments;
  and it takes from `tables/` only the files the text reads (a run writes about
  140, the paper reads about 25, and arXiv flags unused files). Where there is a
  `pdflatex`, it then compiles the archive on its own, the way arXiv will.
- On arXiv's *Check Files* page, the compiler should be detected as pdfLaTeX
  and the top-level file as `main.tex`. Do not let it delete `main.bbl`.
- The title and abstract are typed again into arXiv's metadata form, as plain
  text: copy them from the built PDF, where the numbers are already filled in,
  not from `main.tex`, where they are `\result{…}`. The abstract may be at most
  1 920 characters (it is about 1 650).
- arXiv does not run BibTeX, which is why the archive carries `main.bbl`. The
  paper uses plain BibTeX: with `biblatex`, the `.bbl` only works if it was made
  with the same `biblatex` version that arXiv's TeX Live has.
- The figures are drawn by `pgfplots` from the `.dat` files in `tables/`, so
  there is no image to upload.
- In the *Comments* field, write the page and figure count, and
  `Code and data: https://github.com/ivanjoz/colbin-benchmarks/tree/paper-v1`.
- Check the PDF that arXiv builds before you confirm. That PDF is the one that
  gets published.
- A submission completed before 14:00 US Eastern, Monday to Friday, is
  announced that evening. An error found before the announcement is fixed with
  *Unsubmit*, which costs no version but moves the submission to the back of
  the queue. After the announcement, *replace* the submission (see
  [Revising](#revising)); never submit a correction as a new paper.

### 6. After it is announced

1. Fill in the arXiv ID in `CITATION.cff` (`preferred-citation`) and in the
   README.
2. Commit that on `main`. The commit lands after `paper-v1`, and that is fine:
   the tag does not have to contain its own arXiv ID.

## Revising

- **Text only:** submit a replacement on arXiv. The availability statement
  still points at `paper-v1`.
- **Numbers changed:** repeat steps 2–6 with `paper-v2`, update the availability
  statement, then submit the replacement. In the arXiv *Comments* field, say that
  this version uses new results.
