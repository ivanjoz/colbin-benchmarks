// Package corpusdata turns the generated corpus into bytes on disk, and into a
// checksum that says those bytes are the ones the paper measured.
//
// # Why the corpus is materialised at all
//
// `corpus.Generate` is deterministic: it draws every value from
// `rand.New(rand.NewSource(seed))`, and that stream is fixed for a given seed.
// In practice it has never moved. But "in practice" is not what a reproducible
// artifact rests on:
//
//   - Go's compatibility promise does not cover the `math/rand` bit stream in
//     writing. It is stable because changing it would break the world, which is
//     a social guarantee rather than a documented one — and it is exactly why
//     `math/rand/v2` had to ship as a separate package instead of fixing the
//     old one in place.
//   - A reviewer without a Go toolchain cannot run the generator at all. They
//     can read a JSON file.
//
// So the bytes are the artifact and the generator is provenance, not the other
// way round. If a future Go release ever does move the stream, the test in this
// package fails loudly instead of silently renumbering the dataset underneath a
// published table.
//
// # Why JSON, given colbin exists
//
// Because the corpus has to be inspectable by someone who does not trust — or
// does not have — the thing under test. Encoding the benchmark input with the
// format being benchmarked makes the artifact circular.
//
// There is also a concrete reason a colbin encoding could not serve: `Event`
// carries a `map[string]string`, and a Go map has no iteration order, so the
// colbin bytes for the Events table are not stable run to run. `encoding/json`
// sorts map keys, so the JSON is. That property is what makes a checksum
// meaningful here, and it is the whole reason this file writes JSON.
//
// # Canonical form
//
// One code path writes the bytes, and everything — the committed files, the
// manifest, the test — goes through it, so there is no second encoder to drift
// from the first. The form is: compact `encoding/json`, struct field order as
// declared, map keys sorted by `encoding/json`, HTML escaping off, one trailing
// newline.
package corpusdata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/ivanjoz/colbin/corpus"
)

// Dir is where the materialised bytes and the manifest live, relative to the
// repository root.
const Dir = "corpusdata/data"

// Manifest is the checksum file, in the format `sha256sum -c` reads.
const Manifest = "MANIFEST.sha256"

// Seed is re-exported so the manifest and the paper quote one constant rather
// than two that have to be kept in agreement by hand.
const Seed = corpus.Seed

// A Named scale is one dataset the artifact is answerable for.
type Named struct {
	Name  string
	Scale corpus.Scale

	// Stored is whether the bytes themselves are committed to Dir, or whether
	// the manifest only records what they hash to.
	//
	// Only small is stored, and the reason is the measured cost of the
	// alternative rather than a rule about binaries in git. Generating all three
	// scales and hashing them takes about 1.5 seconds end to end; medium is
	// 12.7 MB of JSON and large is a few hundred. Committing those would make
	// the clone the most expensive part of reproducing the paper in order to
	// save a second and a half, and would put a file nobody reads at the top of
	// every diff.
	//
	// What the artifact actually owes a reader is that the dataset is *pinned*,
	// and the manifest does that for all three: regenerate, compare, done. small
	// is stored on top of that because 337 KB is worth the convenience of being
	// able to open the corpus in an editor and see what a row looks like without
	// running anything.
	Stored bool
}

// Scales is every dataset the benchmarks draw on, in the order the manifest
// lists them.
var Scales = []Named{
	{Name: "small", Scale: corpus.Small, Stored: true},
	{Name: "medium", Scale: corpus.Medium, Stored: false},
	{Name: "large", Scale: corpus.Large, Stored: false},
}

// Lookup finds a scale by name.
func Lookup(name string) (Named, error) {
	for _, candidate := range Scales {
		if candidate.Name == name {
			return candidate, nil
		}
	}
	return Named{}, fmt.Errorf("corpusdata: no scale named %q", name)
}

// Filename is the file a scale is stored as.
func (n Named) Filename() string { return n.Name + ".json" }

// Path is that file inside Dir.
func (n Named) Path() string { return filepath.Join(Dir, n.Filename()) }

// Write streams the canonical JSON for a scale to w.
//
// It streams rather than returning a []byte because large marshals to hundreds
// of megabytes, and the checksum for a dataset nobody stores should not require
// holding that dataset's text in memory.
func (n Named) Write(w io.Writer) error {
	encoder := json.NewEncoder(w)
	// Off, so the bytes are plain JSON rather than JSON with `<`, `>` and `&`
	// spelled as escapes. No corpus string contains those today; pinning it
	// means a future pool entry that does cannot silently change every checksum.
	encoder.SetEscapeHTML(false)
	// Encode writes the compact form plus the single trailing newline that makes
	// this a POSIX text file, which is what `sha256sum` expects to be fed.
	return encoder.Encode(corpus.Generate(Seed, n.Scale))
}

// Sum is the SHA-256 of exactly what Write produces, as lower-case hex.
func (n Named) Sum() (string, error) {
	digest := sha256.New()
	if err := n.Write(digest); err != nil {
		return "", fmt.Errorf("corpusdata: hashing %s: %w", n.Name, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// Rows is the total row count across every table, which is the number worth
// quoting next to a scale in prose.
// It counts top-level rows; a sale's detail lines are not included, since how
// many there are is itself drawn from the generator.
func Rows(scale corpus.Scale) int {
	return scale.Users + scale.Products + scale.Categories + scale.Stores +
		scale.Sales + scale.Events + scale.Metrics
}
