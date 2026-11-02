// Command subset draws a seeded simple random sample of N records (or every
// record, if the table has fewer) from a decompressed Public BI table on stdin,
// and writes them to stdout in the order they appear in the table. fetch.sh
// pipes each download through bzip2 and then through it:
//
//	curl -sL .../Arade_1.csv.bz2 | bzip2 -dc | subset Arade_1 16384 > Arade_1.csv
//
// The sample is bottom-k: record i of table T gets the key
// mix(mix(Seed ^ fnv1a(T)) + i), and the N records with the smallest keys are
// kept. Every set of N records is equally likely, the result depends only on
// Seed, the table's name and its bytes (not on math/rand, whose streams Go may
// change), and the table is read once, in constant memory beyond the sample.
//
// Why not the first N records: the tables are often sorted or grouped by their
// leading columns, so the head of a file repeats values far more than the
// table does, which flatters the formats that exploit repetition.
//
// The tables are '|'-separated with a backslash escaping the next byte, so a
// record ends at a newline that is not escaped.
package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"slices"
	"strconv"
)

// Seed is the sample's base seed, fixed before any sampled table was measured.
// Changing it is a new version of the subset.
const Seed uint64 = 20261102

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: subset TABLE ROWS < table.csv > sample.csv")
		os.Exit(2)
	}
	table := os.Args[1]
	rows, err := strconv.Atoi(os.Args[2])
	if err != nil || rows < 1 {
		fmt.Fprintln(os.Stderr, "subset: ROWS must be a positive integer")
		os.Exit(2)
	}
	sample, total, err := draw(bufio.NewReaderSize(os.Stdin, 1<<20), table, rows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "subset: %s: %v\n", table, err)
		os.Exit(1)
	}
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	for _, record := range sample {
		out.Write(record.bytes)
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "subset:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "subset: %s: %d of %d records\n", table, len(sample), total)
}

type record struct {
	key   uint64
	index int
	bytes []byte
}

// draw returns the sample in table order, and how many records the table has.
func draw(in *bufio.Reader, table string, rows int) ([]record, int, error) {
	name := fnv.New64a()
	name.Write([]byte(table))
	base := mix(Seed ^ name.Sum64())

	kept := &maxHeap{}
	total := 0
	for {
		bytes, err := readRecord(in)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, total, fmt.Errorf("record %d: %w", total, err)
		}
		key := mix(base + uint64(total))
		switch {
		case kept.Len() < rows:
			heap.Push(kept, record{key, total, bytes})
		case key < (*kept)[0].key:
			(*kept)[0] = record{key, total, bytes}
			heap.Fix(kept, 0)
		}
		total++
	}
	if total == 0 {
		return nil, 0, fmt.Errorf("no records")
	}
	sample := []record(*kept)
	slices.SortFunc(sample, func(a, b record) int { return a.index - b.index })
	return sample, total, nil
}

// readRecord returns the next record with its newline, or io.EOF at the end.
// A table whose last record has no newline gets one.
func readRecord(in *bufio.Reader) ([]byte, error) {
	var out []byte
	for {
		chunk, err := in.ReadSlice('\n')
		out = append(out, chunk...)
		switch {
		case err == bufio.ErrBufferFull:
			continue // a record longer than the buffer
		case err == io.EOF:
			if len(out) == 0 {
				return nil, io.EOF
			}
			if endsEscaped(out) {
				return nil, io.ErrUnexpectedEOF
			}
			return append(out, '\n'), nil
		case err != nil:
			return nil, err
		}
		// The newline ends the record unless the byte before it escapes it.
		if !endsEscaped(out[:len(out)-1]) {
			return out, nil
		}
	}
}

// endsEscaped reports whether b ends inside an escape: an odd run of
// backslashes at its end.
func endsEscaped(b []byte) bool {
	run := 0
	for i := len(b) - 1; i >= 0 && b[i] == '\\'; i-- {
		run++
	}
	return run%2 == 1
}

// mix is splitmix64's finalizer: a bijection on uint64 that scatters
// consecutive inputs.
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// maxHeap keeps the largest kept key at the top, the one a smaller key evicts.
type maxHeap []record

func (h maxHeap) Len() int           { return len(h) }
func (h maxHeap) Less(i, j int) bool { return h[i].key > h[j].key }
func (h maxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *maxHeap) Push(x any)        { *h = append(*h, x.(record)) }
func (h *maxHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	return last
}
