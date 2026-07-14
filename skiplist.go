package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sdimitro/crashfp/fingerprint"
)

// entry is one known crash on the skip-list, exported from the
// crash-tracking system. Hashes are hex prefixes (conventionally 16
// chars) of the full SHA-256 fingerprint hashes.
type entry struct {
	typeRIP string // required
	top3    string // optional; when present it must match too
	issue   string // optional issue key, recorded in the note
}

func (e entry) String() string {
	s := "fp-type-rip:" + e.typeRIP
	if e.top3 != "" {
		s += " fp-top3:" + e.top3
	}
	if e.issue != "" {
		s += " " + e.issue
	}
	return s
}

// skiplist is the merged set of known crashes to skip capture for.
type skiplist struct {
	entries   []entry
	generated string // timestamp from the embedded list's header, if any
}

// loadSkiplist parses the embedded list and, when extraPath is set,
// merges entries from that file on top. A missing or unreadable extras
// file is an error so the caller fails open.
func loadSkiplist(embedded, extraPath string) (*skiplist, error) {
	list := &skiplist{}
	if err := list.parse(embedded); err != nil {
		return nil, fmt.Errorf("embedded skip-list: %w", err)
	}

	if extraPath != "" {
		data, err := os.ReadFile(extraPath)
		if err != nil {
			return nil, fmt.Errorf("extra skip-list: %w", err)
		}
		if err := list.parse(string(data)); err != nil {
			return nil, fmt.Errorf("extra skip-list %s: %w", extraPath, err)
		}
	}

	return list, nil
}

// parse appends entries from one skip-list document. Blank lines are
// skipped; "# generated ..." headers record the generation timestamp and
// other comments are ignored. A malformed entry line is an error: a
// truncated or corrupted list must not silently skip fewer (or match
// wrong) crashes.
func (l *skiplist) parse(src string) error {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if rest, ok := strings.CutPrefix(line, "# generated "); ok && l.generated == "" {
				l.generated = strings.Fields(rest)[0]
			}
			continue
		}

		var e entry
		for _, field := range strings.Fields(line) {
			switch {
			case strings.HasPrefix(field, "fp-type-rip:"):
				e.typeRIP = strings.TrimPrefix(field, "fp-type-rip:")
			case strings.HasPrefix(field, "fp-top3:"):
				e.top3 = strings.TrimPrefix(field, "fp-top3:")
			case strings.HasPrefix(field, "fp-"):
				// Other strategies (fp-rip, fp-top5, ...) are not used
				// for skip decisions; ignore them.
			default:
				e.issue = field
			}
		}
		if e.typeRIP == "" || !isHex(e.typeRIP) || (e.top3 != "" && !isHex(e.top3)) {
			return fmt.Errorf("malformed entry %q", line)
		}
		l.entries = append(l.entries, e)
	}
	return nil
}

// match returns the first entry whose type-rip hash prefix matches the
// fingerprint — and whose top3 prefix matches too, when the entry carries
// one — along with a description of what matched.
func (l *skiplist) match(fp *fingerprint.Fingerprint) (*entry, string) {
	for i := range l.entries {
		e := &l.entries[i]
		if !strings.HasPrefix(fp.TypeRIP, e.typeRIP) {
			continue
		}
		if e.top3 != "" {
			if !strings.HasPrefix(fp.Top3, e.top3) {
				continue
			}
			return e, "type-rip+top3"
		}
		return e, "type-rip"
	}
	return nil, ""
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return len(s) > 0
}
