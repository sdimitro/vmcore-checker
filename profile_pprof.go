//go:build memprofile && !tinygo

package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

// writeHeapProfile dumps a heap profile (with cumulative allocation
// samples usable via -sample_index=alloc_space) at exit.
func writeHeapProfile(path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memprofile: %v\n", err)
		return
	}
	defer f.Close()
	runtime.GC() // flush recent allocations into the profile
	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
		fmt.Fprintf(os.Stderr, "memprofile: %v\n", err)
	}
}
