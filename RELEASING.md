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

1. On the measurement machine, run `./reproduce.sh`. It writes `results/`.
2. Commit `results/`, including the machine description it records (CPU, OS,
   Go version, frequency governor, turbo).
3. Copy the tables and figures from `results/` into the paper. Do not edit
   numbers by hand. If a number looks wrong, fix the code and run again.

### 3. Tag the artifact (**manual**)

```sh
git tag -a paper-v1 -m "Artifact for arXiv submission v1"
git push origin paper-v1
```

CI runs on `paper-*` tags. Wait for it to pass before you submit: it checks that
the corpus still matches its manifest on a clean runner. Creating a GitHub
*release* from the tag is optional. It makes the tag easier to find, but arXiv
does not use it.

### 4. Point the paper at the tag

Add a code availability statement with the tag and the full commit hash
(`git rev-parse paper-v1`):

```latex
\section*{Code and data availability}
All benchmarks, datasets and scripts are available at
\url{https://github.com/ivanjoz/colbin-benchmarks/tree/paper-v1}
(commit \texttt{<full hash>}). Running \texttt{./reproduce.sh} at that tag
regenerates every table and figure in this paper.
```

### 5. Submit to arXiv (**manual**)

- Upload the LaTeX source, not only the PDF. arXiv compiles it itself.
- arXiv does not run BibTeX. Include the `.bbl` file next to the `.tex` file.
  Plain BibTeX is the safe choice: with `biblatex`, the `.bbl` only works if it
  was made with the same `biblatex` version that arXiv's TeX Live has.
- Use figures that `pdflatex` accepts (PDF, PNG, JPG). Remove unused files.
- In the *Comments* field, write the page and figure count, and
  `Code and data: https://github.com/ivanjoz/colbin-benchmarks/tree/paper-v1`.
- Check the PDF that arXiv builds before you confirm. That PDF is the one that
  gets published.

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
