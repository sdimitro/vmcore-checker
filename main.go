// vmcore-checker decides, inside the kdump crash kernel, whether the
// crash that just happened is an already-known issue. It parses the
// crashed kernel's log (extracted with vmcore-dmesg or makedumpfile
// --dump-dmesg), computes deterministic fingerprint hashes with
// crashfp, and compares them against a curated skip-list of known
// crashes. On a match it writes a small note.json and exits 0 so the
// kdump hook can skip the expensive vmcore capture and reboot
// immediately.
//
// The tool fails open: any error, missing input, or unrecognized crash
// exits non-zero, and the caller proceeds with normal dump capture.
//
// Usage:
//
//	vmcore-checker [--skiplist extra-entries] [--note out.json] dmesg.txt
//
// Exit codes:
//
//	0 — known issue matched (note written if --note was given)
//	1 — no match, no crash found, or any error: capture the dump
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"github.com/sdimitro/crashfp/dmesgcrash"
	"github.com/sdimitro/crashfp/fingerprint"
)

// embeddedSkiplist is baked in at build time; the initramfs build
// regenerates skiplist.txt from the crash-tracking system before
// compiling.
//
//go:embed skiplist.txt
var embeddedSkiplist string

var version = "dev" // overridden via -ldflags "-X main.version=..."

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, errw *os.File) int {
	fs := flag.NewFlagSet("vmcore-checker", flag.ContinueOnError)
	fs.SetOutput(errw)
	skiplistPath := fs.String("skiplist", "", "file with extra skip-list entries, merged with the embedded list")
	notePath := fs.String("note", "", "write a note.json describing the matched crash to this path")
	printList := fs.Bool("print-skiplist", false, "print the effective skip-list and exit")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *showVersion {
		fmt.Fprintf(errw, "vmcore-checker %s\n", version)
		return 0
	}

	list, err := loadSkiplist(embeddedSkiplist, *skiplistPath)
	if err != nil {
		fmt.Fprintf(errw, "vmcore-checker: %v\n", err)
		return 1
	}

	if *printList {
		fmt.Fprintf(errw, "# %d entries (embedded generation: %s)\n", len(list.entries), orUnknown(list.generated))
		for _, e := range list.entries {
			fmt.Println(e)
		}
		return 0
	}

	if fs.NArg() != 1 {
		fmt.Fprintf(errw, "usage: vmcore-checker [flags] <dmesg.txt>\n")
		return 1
	}

	raw, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(errw, "vmcore-checker: read dmesg: %v\n", err)
		return 1
	}

	crash := dmesgcrash.Parse(string(raw))
	if crash == nil || len(crash.StackTrace) == 0 {
		fmt.Fprintf(errw, "vmcore-checker: no crash backtrace found in dmesg; capturing dump\n")
		return 1
	}

	fp := fingerprint.ComputeFromKernelBacktrace(crash.StackTrace, crash.PanicMessage)
	if fp == nil {
		fmt.Fprintf(errw, "vmcore-checker: backtrace did not fingerprint; capturing dump\n")
		return 1
	}

	entry, matchedBy := list.match(fp)
	if entry == nil {
		fmt.Fprintf(errw, "vmcore-checker: unknown crash %s in %s (type-rip %.16s); capturing dump\n",
			fp.Inputs.CrashType, fp.Inputs.FaultFunc, fp.TypeRIP)
		return 1
	}

	fmt.Fprintf(errw, "vmcore-checker: known crash %s in %s matches %s (by %s); skipping dump\n",
		fp.Inputs.CrashType, fp.Inputs.FaultFunc, orUnknown(entry.issue), matchedBy)

	if *notePath != "" {
		data, err := buildNote(fp, crash, entry, matchedBy, list.generated).Encode()
		if err == nil {
			err = os.WriteFile(*notePath, data, 0o644)
		}
		if err != nil {
			// Without the note the occurrence would be lost entirely;
			// fail open and capture the dump instead.
			fmt.Fprintf(errw, "vmcore-checker: write note: %v; capturing dump\n", err)
			return 1
		}
	}

	return 0
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
