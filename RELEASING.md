# Cutting the paper release

Every number in the paper has to regenerate from one tag with one command. This
is the checklist that makes that true. Steps marked **manual** need a human with
access to the GitHub and Zenodo accounts; nothing in CI can do them.

## 1. colbin is at a published tag

`go.mod` requires colbin at a tag and has no `replace`. If the benchmarks start
using colbin API newer than the required tag:

1. In `github.com/ivanjoz/colbin`: tag and push the release (**manual**).
2. Here: `go get github.com/ivanjoz/colbin@vX.Y.Z && go mod tidy`.
3. Confirm in a clean clone, with no colbin checkout anywhere near it:

   ```sh
   tmp=$(mktemp -d) && git clone https://github.com/ivanjoz/colbin-benchmarks "$tmp/cb"
   cd "$tmp/cb" && GOFLAGS=-mod=readonly go test ./...
   ```

## 2. Zenodo is watching the repository (**manual**, once)

1. Sign in to <https://zenodo.org> with the GitHub account that owns the
   repository.
2. Account → GitHub → flip the switch for `ivanjoz/colbin-benchmarks`.
3. Zenodo now archives every GitHub *release* (not every tag) and mints a DOI
   for it. The metadata comes from `CITATION.cff`, so check that file first.

## 3. Cut `paper-v1` (**manual**)

1. Run `./reproduce.sh` on the measurement machine and commit `results/`.
2. `git tag -a paper-v1 -m "Artifact for the colbin paper"` and push the tag.
3. On GitHub, create a release from `paper-v1`. This is the step Zenodo reacts
   to; a bare tag does not trigger it.
4. Copy the DOI Zenodo shows for the release into `CITATION.cff`, the README
   and the paper. Use the *version* DOI for the paper, since it pins this exact
   artifact; the *concept* DOI always resolves to the newest version.

A later fix to the artifact is `paper-v1.1`, with its own release and DOI. Do not
move `paper-v1`.
