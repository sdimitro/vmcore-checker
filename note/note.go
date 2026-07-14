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
	"strconv"
	"strings"
	"unicode/utf8"
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

// Encode renders the note as indented JSON with a trailing newline. The
// output is byte-identical to encoding/json's MarshalIndent(n, "", "  ")
// (enforced by tests), but hand-rolled so that write-only consumers such
// as the vmcore-checker binary — which never calls Parse — do not link
// the reflection-based JSON machinery. That keeps the binary within its
// crash-kernel initramfs size budget.
func (n *Note) Encode() ([]byte, error) {
	var fields []string
	add := func(key, rendered string) {
		fields = append(fields, "  "+quoteJSON(key)+": "+rendered)
	}
	addString := func(key, val string, omitEmpty bool) {
		if omitEmpty && val == "" {
			return
		}
		add(key, quoteJSON(val))
	}

	add("version", strconv.Itoa(n.Version))
	addString("matched_issue", n.MatchedIssue, true)
	addString("matched_by", n.MatchedBy, false)
	addString("skiplist_generated", n.SkiplistGenerated, true)
	addString("checked_at", n.CheckedAt, true)
	addString("kernel_version", n.KernelVersion, true)
	addString("crash_type", n.CrashType, true)
	addString("fault_func", n.FaultFunc, true)

	fp := []string{
		"    " + quoteJSON("rip") + ": " + quoteJSON(n.Fingerprint.RIP),
		"    " + quoteJSON("top3") + ": " + quoteJSON(n.Fingerprint.Top3),
		"    " + quoteJSON("top5") + ": " + quoteJSON(n.Fingerprint.Top5),
		"    " + quoteJSON("full") + ": " + quoteJSON(n.Fingerprint.Full),
		"    " + quoteJSON("type_rip") + ": " + quoteJSON(n.Fingerprint.TypeRIP),
	}
	add("fingerprint", "{\n"+strings.Join(fp, ",\n")+"\n  }")

	addString("panic_excerpt", n.PanicExcerpt, true)
	if len(n.Frames) > 0 {
		lines := make([]string, len(n.Frames))
		for i, f := range n.Frames {
			lines[i] = "    " + quoteJSON(f)
		}
		add("frames", "[\n"+strings.Join(lines, ",\n")+"\n  ]")
	}

	return []byte("{\n" + strings.Join(fields, ",\n") + "\n}\n"), nil
}

// quoteJSON quotes s exactly as encoding/json does with HTML escaping
// enabled (the json.Marshal default): short escapes for quote, backslash,
// \b, \f, \n, \r, \t; \u00xx for other control characters; \u003c,
// \u003e and \u0026 for <, > and &; \ufffd for invalid UTF-8 bytes; and
// \u2028, \u2029 for the JavaScript line separators.
func quoteJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '"':
				b.WriteString(`\"`)
			case c == '\\':
				b.WriteString(`\\`)
			case c == '\n':
				b.WriteString(`\n`)
			case c == '\r':
				b.WriteString(`\r`)
			case c == '\t':
				b.WriteString(`\t`)
			case c == '\b':
				b.WriteString(`\b`)
			case c == '\f':
				b.WriteString(`\f`)
			case c == '<':
				b.WriteString(`\u003c`)
			case c == '>':
				b.WriteString(`\u003e`)
			case c == '&':
				b.WriteString(`\u0026`)
			case c < 0x20:
				b.WriteString(fmt.Sprintf(`\u%04x`, c))
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteString(`\ufffd`)
			i++
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			b.WriteString(fmt.Sprintf(`\u%04x`, r))
			i += size
			continue
		}
		b.WriteString(s[i : i+size])
		i += size
	}
	b.WriteByte('"')
	return b.String()
}
