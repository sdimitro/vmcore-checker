package note

import (
	"encoding/json"
	"reflect"
	"testing"
)

func sample() *Note {
	return &Note{
		Version:           Version,
		MatchedIssue:      "KERN-1234",
		MatchedBy:         "type-rip+top3",
		SkiplistGenerated: "2026-07-14T12:00:00Z",
		CheckedAt:         "2026-07-14T13:00:00Z",
		KernelVersion:     "6.14.0-1008-nvidia-64k",
		CrashType:         "null_pointer",
		FaultFunc:         "kfifoGetChannelIterator_IMPL",
		Fingerprint: Fingerprint{
			RIP:     "aaaa",
			Top3:    "bbbb",
			Top5:    "cccc",
			Full:    "dddd",
			TypeRIP: "eeee",
		},
		PanicExcerpt: "Unable to handle kernel NULL pointer dereference\nwith \"quotes\" and\ttabs",
		Frames:       []string{"kfifoGetChannelIterator_IMPL+0x3c/0x90 [nvidia] (P)", "kthread+0x100/0x120"},
	}
}

func TestRoundTrip(t *testing.T) {
	n := sample()
	data, err := n.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(n, got) {
		t.Errorf("round trip mismatch:\nwant %+v\ngot  %+v", n, got)
	}
}

func TestParse_NoIssueKeyIsValid(t *testing.T) {
	n := sample()
	n.MatchedIssue = ""
	data, err := n.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err != nil {
		t.Errorf("note without issue key should parse: %v", err)
	}
}

// TestEncodeMatchesEncodingJSON pins the hand-rolled encoder to
// encoding/json byte for byte, so it can never drift from what
// MarshalIndent would produce. This is the compatibility contract that
// lets write-only consumers skip linking the reflection-based encoder.
func TestEncodeMatchesEncodingJSON(t *testing.T) {
	notes := map[string]*Note{
		"full": sample(),
		"minimal": {
			Version:   Version,
			MatchedBy: "type-rip",
			Fingerprint: Fingerprint{
				RIP:     "aaaa",
				Top3:    "bbbb",
				Top5:    "cccc",
				Full:    "dddd",
				TypeRIP: "eeee",
			},
		},
		"zero value": {},
		"nasty strings": {
			Version:      Version,
			MatchedIssue: `quotes " backslash \ slash /`,
			MatchedBy:    "html <>&, unicode \u2028\u2029 \u00e9 \U0001f600",
			PanicExcerpt: "controls \x00\x01\x08\x0c\x1f tab\t nl\n cr\r invalid utf8 \xff\xfe done",
			FaultFunc:    "kfifoGetChannelIterator_IMPL",
			Frames:       []string{"a+0x1/0x2 [mod] (P)", "<TASK>", "b&c<d>e"},
		},
		"empty frames slice": {
			Version:   Version,
			MatchedBy: "type-rip",
			Frames:    []string{},
		},
	}

	for name, n := range notes {
		t.Run(name, func(t *testing.T) {
			want, err := json.MarshalIndent(n, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			want = append(want, '\n')

			got, err := n.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("Encode diverges from encoding/json\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	cases := map[string]string{
		"garbage":       "not json",
		"wrong version": `{"version": 2, "fingerprint": {"type_rip": "ee"}}`,
		"no type_rip":   `{"version": 1, "fingerprint": {}}`,
	}
	for name, in := range cases {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
