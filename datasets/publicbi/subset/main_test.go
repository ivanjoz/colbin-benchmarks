package main

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
)

func sample(t *testing.T, input, table string, rows int) ([]string, int) {
	t.Helper()
	got, total, err := draw(bufio.NewReaderSize(strings.NewReader(input), 16), table, rows)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range got {
		out = append(out, string(r.bytes))
	}
	return out, total
}

func TestEscapedNewlinesStayInTheirRecord(t *testing.T) {
	input := "a|b\\\nstill a|x\nb|\\\\\nc|this record is longer than the reader's buffer\nd"
	got, total := sample(t, input, "T", 10)
	want := []string{"a|b\\\nstill a|x\n", "b|\\\\\n", "c|this record is longer than the reader's buffer\n", "d\n"}
	if total != 4 || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %d %q, want 4 %q", total, got, want)
	}
}

func TestSampleIsSeededOrderedAndSized(t *testing.T) {
	var input strings.Builder
	for i := range 1000 {
		fmt.Fprintf(&input, "%04d\n", i)
	}
	first, total := sample(t, input.String(), "T", 100)
	again, _ := sample(t, input.String(), "T", 100)
	other, _ := sample(t, input.String(), "U", 100)
	if total != 1000 || len(first) != 100 {
		t.Fatalf("%d of %d records, want 100 of 1000", len(first), total)
	}
	if fmt.Sprint(first) != fmt.Sprint(again) {
		t.Fatal("the same table and seed drew different samples")
	}
	if fmt.Sprint(first) == fmt.Sprint(other) {
		t.Fatal("two tables drew the same sample")
	}
	for i := 1; i < len(first); i++ {
		if first[i-1] >= first[i] {
			t.Fatalf("not in table order: %q before %q", first[i-1], first[i])
		}
	}
	if first[len(first)-1] < "0500" {
		t.Fatalf("sample never reaches the second half of the table: last is %q", first[len(first)-1])
	}
}
