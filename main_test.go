package main

import (
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
	if err := l.parse(src); err != nil {
		t.Fatalf("parse: %v", err)
	}
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

func TestSkiplistParse_Malformed(t *testing.T) {
	cases := []string{
		"no-labels-here",               // no type-rip
		"fp-top3:9a1b2c3d4e5f6071 X-1", // top3 without type-rip
		"fp-type-rip:NOTHEX",           // non-hex hash
		"fp-type-rip:de43 fp-top3:XYZ", // non-hex top3
		"fp-type-rip:",                 // empty hash
	}
	for _, src := range cases {
		var l skiplist
		if err := l.parse(src); err == nil {
			t.Errorf("parse(%q): expected error", src)
		}
	}
}

func TestSkiplistMatch(t *testing.T) {
	var l skiplist
	err := l.parse(strings.Join([]string{
		"fp-type-rip:aaaa000000000000 fp-top3:bbbb000000000000 X-1",
		"fp-type-rip:cccc000000000000 X-2",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
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
	if _, err := loadSkiplist("", "/nonexistent/skiplist"); err == nil {
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
		{"malformed extras file", []string{"--skiplist", writeFile(t, dir, "bad-list", "this is not a skiplist entry\n"), dmesg}},
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
