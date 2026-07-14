//go:build memprofile

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

// Profiling instrumentation, enabled with `-tags memprofile` (see the
// profile-linux-% and tiny-profile-linux-% Makefile targets). Kept out
// of release builds so the size budgets are unaffected.
var (
	memStatsFlag   *bool
	memProfilePath *string
)

func profileFlags(fs *flag.FlagSet) {
	memStatsFlag = fs.Bool("memstats", false, "print runtime.ReadMemStats summary to stderr at exit")
	memProfilePath = fs.String("memprofile", "", "write a Go heap profile to this path at exit (stock Go builds only)")
}

func profileAtExit(_ *int) {
	if memStatsFlag != nil && *memStatsFlag {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Fprintf(os.Stderr,
			"memstats: Sys=%d HeapSys=%d HeapInuse=%d HeapAlloc=%d TotalAlloc=%d Mallocs=%d Frees=%d\n",
			m.Sys, m.HeapSys, m.HeapInuse, m.HeapAlloc, m.TotalAlloc, m.Mallocs, m.Frees)
	}
	if memProfilePath != nil && *memProfilePath != "" {
		writeHeapProfile(*memProfilePath)
	}
}
