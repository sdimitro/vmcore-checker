package main

import (
	"strings"
	"time"

	"github.com/sdimitro/crashfp/dmesgcrash"
	"github.com/sdimitro/crashfp/fingerprint"
	"github.com/sdimitro/vmcore-checker/note"
)

const (
	maxNoteFrames  = 5
	maxExcerptSize = 1000
)

// buildNote assembles the note document consumed by the post-reboot
// triage pipeline.
func buildNote(fp *fingerprint.Fingerprint, crash *dmesgcrash.Crash, e *entry, matchedBy, skiplistGenerated string) *note.Note {
	frames := crash.StackTrace
	if len(frames) > maxNoteFrames {
		frames = frames[:maxNoteFrames]
	}
	excerpt := crash.PanicMessage
	if len(excerpt) > maxExcerptSize {
		excerpt = excerpt[:maxExcerptSize]
	}

	return &note.Note{
		Version:           note.Version,
		MatchedIssue:      e.issue,
		MatchedBy:         matchedBy,
		SkiplistGenerated: skiplistGenerated,
		CheckedAt:         time.Now().UTC().Format(time.RFC3339),
		KernelVersion:     kernelVersion(crash.Banner),
		CrashType:         fp.Inputs.CrashType,
		FaultFunc:         fp.Inputs.FaultFunc,
		Fingerprint: note.Fingerprint{
			RIP:     fp.RIP,
			Top3:    fp.Top3,
			Top5:    fp.Top5,
			Full:    fp.Full,
			TypeRIP: fp.TypeRIP,
		},
		PanicExcerpt: excerpt,
		Frames:       frames,
	}
}

// kernelVersion extracts the release string from the kernel banner:
// "Linux version 6.14.0-1008-nvidia-64k (buildd@...) ..." → "6.14.0-1008-nvidia-64k".
func kernelVersion(banner string) string {
	rest, ok := strings.CutPrefix(banner, "Linux version ")
	if !ok {
		return ""
	}
	if i := strings.IndexByte(rest, ' '); i > 0 {
		return rest[:i]
	}
	return rest
}
