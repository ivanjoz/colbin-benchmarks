// Command materialize writes the benchmark corpus to disk and records what it
// hashes to.
//
// Run from the repository root:
//
//	go run ./cmd/materialize            # write the stored scales, rewrite the manifest
//	go run ./cmd/materialize -scale large   # also produce large.json, which is not committed
//	go run ./cmd/materialize -check     # verify what is on disk, write nothing
//
// The manifest is ordinary `sha256sum` output, so verifying it needs no Go:
//
//	cd corpusdata/data && sha256sum -c --ignore-missing MANIFEST.sha256
//
// `--ignore-missing` is there because large.json is deliberately absent. Drop
// the flag after generating it to check all three.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ivanjoz/colbin-benchmarks/corpusdata"
)

func main() {
	scale := flag.String("scale", "", "materialise only this scale, including one that is not normally stored")
	check := flag.Bool("check", false, "verify the files on disk against a fresh generation, write nothing")
	flag.Parse()

	if err := run(*scale, *check); err != nil {
		fmt.Fprintln(os.Stderr, "materialize:", err)
		os.Exit(1)
	}
}

func run(only string, check bool) error {
	wanted := corpusdata.Scales
	if only != "" {
		found, err := corpusdata.Lookup(only)
		if err != nil {
			return err
		}
		// An explicit -scale overrides Stored: asking for large by name is how a
		// reviewer produces the file the manifest already has a line for.
		found.Stored = true
		wanted = []corpusdata.Named{found}
	}

	if err := os.MkdirAll(corpusdata.Dir, 0o755); err != nil {
		return err
	}

	sums := make(map[string]string, len(corpusdata.Scales))
	for _, named := range wanted {
		sum, err := named.Sum()
		if err != nil {
			return err
		}
		sums[named.Name] = sum

		if !named.Stored {
			fmt.Printf("%-7s %9d rows  %s  (not stored)\n", named.Name, corpusdata.Rows(named.Scale), sum[:16])
			continue
		}
		if check {
			if err := verify(named, sum); err != nil {
				return err
			}
			fmt.Printf("%-7s %9d rows  %s  ok\n", named.Name, corpusdata.Rows(named.Scale), sum[:16])
			continue
		}
		size, err := write(named)
		if err != nil {
			return err
		}
		fmt.Printf("%-7s %9d rows  %s  %s  %d B\n",
			named.Name, corpusdata.Rows(named.Scale), sum[:16], named.Filename(), size)
	}

	// A partial run would otherwise write a manifest missing the scales it did
	// not visit, which is worse than not writing one.
	if check || only != "" {
		return nil
	}
	return writeManifest(sums)
}

// write streams a scale to its file, through a buffer because the encoder emits
// many small writes and large is hundreds of megabytes of them.
func write(named corpusdata.Named) (int64, error) {
	file, err := os.Create(named.Path())
	if err != nil {
		return 0, err
	}
	defer file.Close()

	buffered := bufio.NewWriterSize(file, 1<<20)
	if err := named.Write(buffered); err != nil {
		return 0, err
	}
	if err := buffered.Flush(); err != nil {
		return 0, err
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// sumFile hashes a file on disk, streaming it.
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

// verify compares the bytes on disk against a fresh generation. It hashes the
// file rather than reading both into memory and diffing, so -check costs the
// same whatever the scale.
func verify(named corpusdata.Named, want string) error {
	got, err := sumFile(named.Path())
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%s: on disk %s, freshly generated %s — "+
			"either the file was edited or the generator moved; see corpusdata's package comment",
			named.Path(), got, want)
	}
	return nil
}

func writeManifest(sums map[string]string) error {
	path := filepath.Join(corpusdata.Dir, corpusdata.Manifest)
	var text []byte
	for _, named := range corpusdata.Scales {
		sum, ok := sums[named.Name]
		if !ok {
			return fmt.Errorf("no checksum computed for %s", named.Name)
		}
		// `sha256sum` format: hash, two spaces, filename. Nothing else on the
		// line, and no comments — GNU sha256sum accepts comment lines, but BSD
		// and busybox do not, and the point of this file is that anyone can
		// check it. The prose belongs in the README.
		text = append(text, fmt.Sprintf("%s  %s\n", sum, named.Filename())...)
	}
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}
