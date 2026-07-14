//go:build memprofile && tinygo

package main

import (
	"fmt"
	"os"
)

// TinyGo has no runtime/pprof; heap profiles are only available from the
// stock Go build. Allocation sites for TinyGo are inspected statically
// with `tinygo build -print-allocs=.` instead.
func writeHeapProfile(string) {
	fmt.Fprintln(os.Stderr, "memprofile: not supported under tinygo; use -print-allocs at build time")
}
