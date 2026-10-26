// Command subset copies the first N records (or all, if fewer) of a
// bzip2-compressed Public BI table from stdin to stdout, decompressed. fetch.sh
// pipes the download through
// it, so only the head of each table is ever read off the network.
//
//	curl -sL .../Arade_1.csv.bz2 | go run ./datasets/publicbi/subset 16384 > Arade_1.csv
//
// The tables are '|'-separated with a backslash escaping the next byte, so a
// record ends at a newline that is not escaped.
package main

import (
	"bufio"
	"compress/bzip2"
	"fmt"
	"io"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: subset ROWS < table.csv.bz2 > table.csv")
		os.Exit(2)
	}
	rows, err := strconv.Atoi(os.Args[1])
	if err != nil || rows < 1 {
		fmt.Fprintln(os.Stderr, "subset: ROWS must be a positive integer")
		os.Exit(2)
	}
	in := bufio.NewReaderSize(bzip2.NewReader(bufio.NewReader(os.Stdin)), 1<<20)
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	escaped := false
	for written := 0; written < rows; {
		b, err := in.ReadByte()
		if err == io.EOF && written > 0 && !escaped {
			break // a table shorter than ROWS is taken whole
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "subset: %d of %d records: %v\n", written, rows, err)
			os.Exit(1)
		}
		out.WriteByte(b)
		switch {
		case escaped:
			escaped = false
		case b == '\\':
			escaped = true
		case b == '\n':
			written++
		}
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "subset:", err)
		os.Exit(1)
	}
}
