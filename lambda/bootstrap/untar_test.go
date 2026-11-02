package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUntarZstd unpacks a corpus tarball lambda/deploy.py build wrote, when
// there is one, and checks it is non-empty.
func TestUntarZstd(t *testing.T) {
	archive := os.Getenv("COLBIN_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set COLBIN_TEST_ARCHIVE to a lambda/build/*.tar.zst")
	}
	dest := t.TempDir()
	if err := untarZstd(archive, dest); err != nil {
		t.Fatal(err)
	}
	var files int
	filepath.Walk(dest, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && info.Mode().IsRegular() && info.Size() > 0 {
			files++
		}
		return nil
	})
	if files == 0 {
		t.Fatal("nothing unpacked")
	}
}
