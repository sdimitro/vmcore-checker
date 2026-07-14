// Package note defines the note.json document that vmcore-checker writes
// in place of a full vmcore when a crash matches the known-issue
// skip-list. The note is uploaded where the dump would have gone,
// typically as <serial>-<YYYYMMDDhhmm>-note.json, where the post-reboot
// triage pipeline picks it up and records the occurrence on the matched
// issue.
//
// Both the writer (vmcore-checker) and any readers import this package,
// so the wire format cannot drift between them.
package note

import (
	"encoding/json"
	"fmt"
)

// Version is the current note schema version.
const Version = 1

// Fingerprint carries the full SHA-256 hex hashes computed by
// crashfp/fingerprint for the crash the note describes.
type Fingerprint struct {
	RIP     string `json:"rip"`
	Top3    string `json:"top3"`
	Top5    string `json:"top5"`
	Full    string `json:"full"`
	TypeRIP string `json:"type_rip"`
}

// Note summarizes a crash whose vmcore capture was skipped because its
// fingerprint matched the skip-list.
type Note struct {
	Version int `json:"version"`

	// MatchedIssue is the issue key from the matching skip-list entry
	// (e.g. "KERN-1234"). It may be empty for hand-added entries;
	// readers should locate the issue by fingerprint.
	MatchedIssue string `json:"matched_issue,omitempty"`

	// MatchedBy records which hashes matched: "type-rip" or
	// "type-rip+top3".
	MatchedBy string `json:"matched_by"`

	// SkiplistGenerated is the generation timestamp from the skip-list
	// header, recording how fresh the list was at match time.
	SkiplistGenerated string `json:"skiplist_generated,omitempty"`

	// CheckedAt is when vmcore-checker made the decision (RFC 3339).
	CheckedAt string `json:"checked_at,omitempty"`

	KernelVersion string `json:"kernel_version,omitempty"`
	CrashType     string `json:"crash_type,omitempty"`
	FaultFunc     string `json:"fault_func,omitempty"`

	Fingerprint Fingerprint `json:"fingerprint"`

	// PanicExcerpt is the start of the panic message, capped by the
	// writer for size.
	PanicExcerpt string `json:"panic_excerpt,omitempty"`

	// Frames are the first few backtrace frames, fault frame first.
	Frames []string `json:"frames,omitempty"`
}

// Parse decodes and validates a note document.
func Parse(data []byte) (*Note, error) {
	var n Note
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("parse note: %w", err)
	}
	if n.Version != Version {
		return nil, fmt.Errorf("unsupported note version %d", n.Version)
	}
	if n.Fingerprint.TypeRIP == "" {
		return nil, fmt.Errorf("note has no type_rip fingerprint")
	}
	return &n, nil
}

// Encode renders the note as indented JSON with a trailing newline.
func (n *Note) Encode() ([]byte, error) {
	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode note: %w", err)
	}
	return append(data, '\n'), nil
}
