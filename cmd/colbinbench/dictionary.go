package main

import (
	"os"

	"github.com/ivanjoz/colbin"
)

// COLBIN_DICTIONARY=1 runs colbin with string dictionaries on, so the same
// build can be measured against itself without them: a script that sets it
// and execs the binary is a -baseline like any other.
func init() {
	if os.Getenv("COLBIN_DICTIONARY") == "1" {
		colbin.SetDictionary(true)
	}
}
