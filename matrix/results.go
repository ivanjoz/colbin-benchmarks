package matrix

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

// table is one CSV under $COLBIN_RESULTS, or nothing when it is unset.
type table struct {
	file   *os.File
	writer *csv.Writer
}

func openTable(t testing.TB, name string, header ...string) *table {
	dir := os.Getenv("COLBIN_RESULTS")
	if dir == "" {
		return &table{}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	writer := csv.NewWriter(file)
	writer.Write(header)
	t.Cleanup(func() {
		writer.Flush()
		file.Close()
	})
	return &table{file, writer}
}

func (t *table) row(fields ...string) {
	if t.writer != nil {
		t.writer.Write(fields)
		t.writer.Flush()
	}
}
