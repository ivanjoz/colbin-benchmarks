package corpusdata

// The tests that make the artifact an artifact.
//
// Between them they assert the three things a reader of the paper is entitled
// to assume: that the committed bytes are what the generator produces, that the
// manifest describes those bytes, and that "what the generator produces" is a
// single answer rather than a different one per run.
//
// A failure here is not a flaky test. It means the dataset behind a published
// table has moved, and the first question is which of the three broke.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanjoz/colbin/corpus"
)

// root resolves a repository-root-relative path from inside the package.
//
// `Dir` is written from the root because that is where `go run ./cmd/...`
// starts, but `go test` always runs with the working directory set to the
// package under test. One `..` reconciles the two.
func root(path string) string { return filepath.Join("..", path) }

// TestStoredFilesMatchTheGenerator is the load-bearing one. If Go's math/rand
// stream ever moves, or somebody hand-edits a corpus file, this is what says so
// — rather than the benchmarks quietly reporting numbers for a dataset that is
// no longer the published one.
func TestStoredFilesMatchTheGenerator(t *testing.T) {
	for _, named := range Scales {
		if !named.Stored {
			continue
		}
		t.Run(named.Name, func(t *testing.T) {
			want, err := named.Sum()
			if err != nil {
				t.Fatal(err)
			}
			got, err := sumFile(root(named.Path()))
			if err != nil {
				t.Fatalf("%s: %v (run `go run ./cmd/materialize` from the repository root)", named.Path(), err)
			}
			if got != want {
				t.Fatalf("%s has drifted\n  on disk:    %s\n  generated:  %s\n"+
					"Either the file was edited, or the generator no longer produces the published dataset.",
					named.Path(), got, want)
			}
		})
	}
}

// TestManifestMatchesTheGenerator covers every scale, including the one whose
// bytes are not committed. large is where the columnar layout actually matters,
// so its checksum being unverifiable would leave the most interesting table in
// the paper resting on nothing.
func TestManifestMatchesTheGenerator(t *testing.T) {
	listed, err := readManifest(root(filepath.Join(Dir, Manifest)))
	if err != nil {
		t.Fatal(err)
	}
	for _, named := range Scales {
		t.Run(named.Name, func(t *testing.T) {
			// Gate on size, not on Stored: medium is not committed either, but
			// regenerating it costs milliseconds and there is no reason to skip
			// it. Only large is big enough to notice.
			if testing.Short() && Rows(named.Scale) > 1_000_000 {
				t.Skipf("%s is %d rows; checking it against the manifest takes real time", named.Name, Rows(named.Scale))
			}
			want, ok := listed[named.Filename()]
			if !ok {
				t.Fatalf("%s has no line for %s", Manifest, named.Filename())
			}
			got, err := named.Sum()
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("%s disagrees with the generator for %s\n  manifest:   %s\n  generated:  %s",
					Manifest, named.Filename(), want, got)
			}
		})
	}
	if len(listed) != len(Scales) {
		t.Errorf("%s lists %d files, but there are %d scales", Manifest, len(listed), len(Scales))
	}
}

// TestCanonicalFormIsDeterministic generates the same scale twice in one process
// and requires the bytes to be identical.
//
// This is not redundant with the checks above, and it is the test that earns
// JSON its place: `Event.Fields` is a `map[string]string`, and a Go map's
// iteration order is deliberately randomised per range. A colbin encoding of the
// Events table therefore differs run to run. `encoding/json` sorts map keys, so
// this form does not — and that is precisely what makes a checksum of it mean
// anything. If the canonical form ever changes to one that walks a map directly,
// this fails.
func TestCanonicalFormIsDeterministic(t *testing.T) {
	named, err := Lookup("small")
	if err != nil {
		t.Fatal(err)
	}
	first, err := named.Sum()
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		again, err := named.Sum()
		if err != nil {
			t.Fatal(err)
		}
		if again != first {
			t.Fatalf("the canonical form is not stable within one process: %s then %s", first, again)
		}
	}
}

// TestEventsCarryTheMapThisAllRestsOn guards the premise rather than the result.
// If the corpus ever loses its map-valued field the determinism test above still
// passes, but it stops testing anything, and the reasoning in the package
// comment quietly becomes false.
func TestEventsCarryTheMapThisAllRestsOn(t *testing.T) {
	named, err := Lookup("small")
	if err != nil {
		t.Fatal(err)
	}
	built := corpus.Generate(Seed, named.Scale)
	populated := 0
	for _, event := range built.Events {
		if len(event.Fields) > 1 {
			populated++
		}
	}
	if populated == 0 {
		t.Fatal("no Event carries a multi-entry map; the determinism test no longer proves map ordering is handled")
	}
}

func sumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// readManifest parses `sha256sum` output: hash, two spaces, filename.
func readManifest(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		hash, name, found := strings.Cut(line, "  ")
		if !found {
			return nil, &parseError{path: path, line: line}
		}
		out[name] = hash
	}
	return out, scanner.Err()
}

type parseError struct {
	path string
	line string
}

func (e *parseError) Error() string {
	return e.path + ": not sha256sum format: " + e.line
}
