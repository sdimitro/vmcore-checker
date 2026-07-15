package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sdimitro/crashfp/fingerprint"
	"github.com/sdimitro/vmcore-checker/note"
)

func TestSkiplistParse(t *testing.T) {
	src := `# generated 2026-07-14T12:00:00Z from KERN
fp-type-rip:de433837302e1e77 fp-top3:9a1b2c3d4e5f6071 KERN-1234

# a hand-added entry with no top3 or issue
fp-type-rip:76571165e62ca40f
fp-type-rip:0011223344556677 fp-rip:8899aabbccddeeff KERN-999
`
	var l skiplist
	l.parse(src, "test", io.Discard)
	if l.generated != "2026-07-14T12:00:00Z" {
		t.Errorf("generated: got %q", l.generated)
	}
	if len(l.entries) != 3 {
		t.Fatalf("entries: got %d, want 3", len(l.entries))
	}
	if e := l.entries[0]; e.typeRIP != "de433837302e1e77" || e.top3 != "9a1b2c3d4e5f6071" || e.issue != "KERN-1234" {
		t.Errorf("entry 0: %+v", e)
	}
	if e := l.entries[1]; e.typeRIP != "76571165e62ca40f" || e.top3 != "" || e.issue != "" {
		t.Errorf("entry 1: %+v", e)
	}
	// fp-rip is ignored, issue still parsed.
	if e := l.entries[2]; e.typeRIP != "0011223344556677" || e.top3 != "" || e.issue != "KERN-999" {
		t.Errorf("entry 2: %+v", e)
	}
}

// Malformed lines are skipped with a warning instead of rejecting the
// whole list: one bad hand-added entry must not disable the fast path.
func TestSkiplistParse_SkipsMalformed(t *testing.T) {
	src := strings.Join([]string{
		"no-labels-here",                           // no type-rip
		"fp-top3:9a1b2c3d4e5f6071 X-1",             // top3 without type-rip
		"fp-type-rip:NOTAHEXSTRING",                // non-hex hash
		"fp-type-rip:de433837302e1e77 fp-top3:XYZ", // non-hex top3
		"fp-type-rip:",                             // empty hash
		"fp-type-rip:de433837302e",                 // 12 hex: at the floor, valid
		"fp-type-rip:de43383730",                   // 10 hex: below the floor
		"fp-type-rip:76571165e62ca40f KERN-42",     // valid
	}, "\n")

	var warnings bytes.Buffer
	var l skiplist
	l.parse(src, "extras", &warnings)

	if len(l.entries) != 2 {
		t.Fatalf("entries: got %d (%v), want 2", len(l.entries), l.entries)
	}
	if e := l.entries[0]; e.typeRIP != "de433837302e" {
		t.Errorf("floor entry: %+v", e)
	}
	if e := l.entries[1]; e.typeRIP != "76571165e62ca40f" || e.issue != "KERN-42" {
		t.Errorf("valid entry: %+v", e)
	}
	if n := strings.Count(warnings.String(), "skipping malformed skip-list entry"); n != 6 {
		t.Errorf("warnings: got %d, want 6\n%s", n, warnings.String())
	}
	if !strings.Contains(warnings.String(), "extras line 2:") {
		t.Errorf("warning should carry source and line number:\n%s", warnings.String())
	}
}

// A malformed line must not prevent later valid entries from matching.
func TestRun_MalformedLineDoesNotDisableList(t *testing.T) {
	dir := t.TempDir()
	dmesg := writeFile(t, dir, "dmesg.txt", sampleDmesg)
	extras := writeFile(t, dir, "extras",
		"this line is garbage\nfp-type-rip:"+sampleTypeRIPPrefix+" KERN-1234\n")

	if code := run([]string{"--skiplist", extras, dmesg}, os.Stderr); code != 0 {
		t.Errorf("exit code: got %d, want 0 (valid entry should still match)", code)
	}
}

func TestSkiplistMatch(t *testing.T) {
	var l skiplist
	l.parse(strings.Join([]string{
		"fp-type-rip:aaaa000000000000 fp-top3:bbbb000000000000 X-1",
		"fp-type-rip:cccc000000000000 X-2",
	}, "\n"), "test", io.Discard)
	if len(l.entries) != 2 {
		t.Fatalf("entries: got %d, want 2", len(l.entries))
	}

	fp := func(typeRIP, top3 string) *fingerprint.Fingerprint {
		return &fingerprint.Fingerprint{TypeRIP: typeRIP, Top3: top3}
	}

	// type-rip and top3 both match.
	if e, by := l.match(fp("aaaa000000000000ffff", "bbbb000000000000ffff")); e == nil || e.issue != "X-1" || by != "type-rip+top3" {
		t.Errorf("full match failed: %+v %q", e, by)
	}
	// type-rip matches but entry requires top3 which doesn't.
	if e, _ := l.match(fp("aaaa000000000000ffff", "dddd000000000000ffff")); e != nil {
		t.Errorf("expected no match when top3 differs, got %+v", e)
	}
	// bare type-rip entry matches regardless of top3.
	if e, by := l.match(fp("cccc000000000000ffff", "whatever")); e == nil || e.issue != "X-2" || by != "type-rip" {
		t.Errorf("type-rip match failed: %+v %q", e, by)
	}
	// unknown crash.
	if e, _ := l.match(fp("eeee000000000000ffff", "")); e != nil {
		t.Errorf("expected no match, got %+v", e)
	}
}

func TestLoadSkiplist_MissingExtrasFails(t *testing.T) {
	if _, err := loadSkiplist("", "/nonexistent/skiplist", io.Discard); err == nil {
		t.Error("expected error for missing extras file")
	}
}

// sampleDmesg is an arm64 NULL-pointer crash whose type-rip hash is
// sha256("null_pointer:kfifoGetChannelIterator_IMPL").
const sampleDmesg = `[    0.000000] [    T0] Linux version 6.14.0-1008-nvidia-64k (buildd@bos03-arm64-088) (gcc 13.3.0) #8-Ubuntu SMP
[ 2855.144566] [ T131119] Unable to handle kernel NULL pointer dereference at virtual address 0000000000000360
[ 2855.211771] [ T131119] Internal error: Oops: 0000000096000005 [#1] SMP
[ 2855.384846] [ T131119] pc : kfifoGetChannelIterator_IMPL+0x3c/0x90 [nvidia]
[ 2855.474073] [ T131119] Call trace:
[ 2855.476568] [ T131119]  kfifoGetChannelIterator_IMPL+0x3c/0x90 [nvidia] (P)
[ 2855.482996] [ T131119]  kgspRcAndNotifyAllChannels_IMPL+0x8c/0x220 [nvidia]
[ 2855.489403] [ T131119]  osHandleGpuLost+0x138/0x140 [nvidia]
[ 2855.551585] [ T131119]  kthread+0x100/0x120
[ 2855.554887] [ T131119]  ret_from_fork+0x10/0x20
[ 2855.558544] [ T131119] Code: 94066b7d 3100069f 54000101 b9000e7f (b94362a0)
`

const sampleTypeRIPPrefix = "76571165e62ca40f"

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun_KnownCrashWritesNote(t *testing.T) {
	dir := t.TempDir()
	dmesg := writeFile(t, dir, "dmesg.txt", sampleDmesg)
	extras := writeFile(t, dir, "extras",
		"# generated 2026-07-14T12:00:00Z from KERN\nfp-type-rip:"+sampleTypeRIPPrefix+" KERN-1234\n")
	notePath := filepath.Join(dir, "note.json")

	code := run([]string{"--skiplist", extras, "--note", notePath, dmesg}, os.Stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d, want 0", code)
	}

	data, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("note not written: %v", err)
	}

	n, err := note.Parse(data)
	if err != nil {
		t.Fatalf("note does not parse with the shared package: %v\n%s", err, data)
	}
	if n.MatchedIssue != "KERN-1234" || n.MatchedBy != "type-rip" {
		t.Errorf("note header: %+v", n)
	}
	if n.KernelVersion != "6.14.0-1008-nvidia-64k" {
		t.Errorf("kernel: got %q", n.KernelVersion)
	}
	if n.CrashType != "null_pointer" || n.FaultFunc != "kfifoGetChannelIterator_IMPL" {
		t.Errorf("classification: %+v", n)
	}
	if !strings.HasPrefix(n.Fingerprint.TypeRIP, sampleTypeRIPPrefix) {
		t.Errorf("type_rip: got %q", n.Fingerprint.TypeRIP)
	}
	if len(n.Frames) == 0 || len(n.Frames) > maxNoteFrames {
		t.Errorf("frames: got %d", len(n.Frames))
	}
	if !strings.Contains(n.PanicExcerpt, "NULL pointer dereference") {
		t.Errorf("panic excerpt: got %q", n.PanicExcerpt)
	}
	if n.SkiplistGenerated != "2026-07-14T12:00:00Z" {
		t.Errorf("skiplist_generated: got %q", n.SkiplistGenerated)
	}
}

// TestStreamMatchesDefault verifies the two input modes are
// interchangeable: same exit code and byte-identical note.json (modulo
// the checked_at timestamp) whether the log is loaded whole (default)
// or streamed line by line (--stream).
func TestStreamMatchesDefault(t *testing.T) {
	dir := t.TempDir()
	extras := writeFile(t, dir, "extras", "fp-type-rip:"+sampleTypeRIPPrefix+" KERN-1234\n")

	normalize := func(b []byte) string {
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			if strings.Contains(l, `"checked_at"`) {
				lines[i] = `  "checked_at": "X",`
			}
		}
		return strings.Join(lines, "\n")
	}

	inputs := map[string]string{
		"matching crash":   sampleDmesg,
		"no crash":         "[ 0.0] [T0] Linux version 6.1.0 (a@b) (gcc) #1\n[ 1.0] [T1] systemd booted\n",
		"garbage":          "not a kernel log\n",
		"empty":            "",
		"trailing noise":   sampleDmesg + "[ 9999.0] [T1] systemd[1]: Started Session 1 of User root.\n",
		"no final newline": strings.TrimSuffix(sampleDmesg, "\n"),
	}

	for name, content := range inputs {
		t.Run(name, func(t *testing.T) {
			input := writeFile(t, dir, "in.txt", content)
			defNote := filepath.Join(dir, "def-note.json")
			strNote := filepath.Join(dir, "str-note.json")
			os.Remove(defNote)
			os.Remove(strNote)

			defCode := run([]string{"--skiplist", extras, "--note", defNote, input}, os.Stderr)
			strCode := run([]string{"--stream", "--skiplist", extras, "--note", strNote, input}, os.Stderr)
			if defCode != strCode {
				t.Fatalf("exit codes differ: default=%d stream=%d", defCode, strCode)
			}

			defData, defErr := os.ReadFile(defNote)
			strData, strErr := os.ReadFile(strNote)
			if os.IsNotExist(defErr) != os.IsNotExist(strErr) {
				t.Fatalf("note presence differs: default=%v stream=%v", defErr, strErr)
			}
			if defErr == nil && normalize(defData) != normalize(strData) {
				t.Errorf("notes differ:\ndefault:\n%s\nstream:\n%s", defData, strData)
			}
		})
	}
}

func TestRun_FailOpen(t *testing.T) {
	dir := t.TempDir()
	dmesg := writeFile(t, dir, "dmesg.txt", sampleDmesg)

	cases := []struct {
		name string
		args []string
	}{
		{"unknown crash", []string{dmesg}},
		{"missing dmesg", []string{filepath.Join(dir, "nope.txt")}},
		{"no crash in dmesg", []string{writeFile(t, dir, "boring.txt", "[ 0.0] [T0] Linux version 6.1.0 (a@b) (gcc) #1\n[ 1.0] [T1] systemd booted\n")}},
		{"empty dmesg", []string{writeFile(t, dir, "empty.txt", "")}},
		{"missing extras file", []string{"--skiplist", filepath.Join(dir, "nope-list"), dmesg}},
		// Malformed lines are skipped (not fatal), so an extras file with
		// only bad lines contributes nothing and the crash stays unmatched.
		{"extras with only malformed lines", []string{"--skiplist", writeFile(t, dir, "bad-list", "this is not a skiplist entry\n"), dmesg}},
		{"no args", nil},
		{"too many args", []string{dmesg, dmesg}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := run(tc.args, os.Stderr); code != 1 {
				t.Errorf("exit code: got %d, want 1", code)
			}
		})
	}
}

func TestKernelVersion(t *testing.T) {
	if got := kernelVersion("Linux version 6.14.0-1008-nvidia-64k (buildd@x) (gcc) #8"); got != "6.14.0-1008-nvidia-64k" {
		t.Errorf("got %q", got)
	}
	if got := kernelVersion(""); got != "" {
		t.Errorf("empty banner: got %q", got)
	}
}
