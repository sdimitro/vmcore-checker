#!/bin/sh
# Behavioral parity gate between the stock Go build and the TinyGo build.
#
# Builds both compilers' binaries for the host platform, runs them over
# every golden corpus kernel log plus the fail-open cases, and requires
# identical exit codes and byte-identical note.json output (modulo the
# checked_at timestamp). Fingerprint hashes are deterministic, so any
# divergence between the two compilers is a hard failure — the TinyGo
# binary must never skip a different set of dumps than the stock one.
#
# Usage: scripts/verify-tiny.sh  (from the repository root)
# Env: TINYGO (default: tinygo), GO (default: go)
set -eu

TINYGO=${TINYGO:-tinygo}
GO=${GO:-go}

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

echo "== building stock and tiny binaries for the host platform"
CGO_ENABLED=0 $GO build -trimpath -buildvcs=false -ldflags "-s -w" -o "$WORK/stock" .
$TINYGO build -opt=z -no-debug -o "$WORK/tiny" .

# The corpus lives in the crashfp module; resolve its location from the
# module cache (or a local replace) via go list.
CORPUS_DIR=$($GO list -m -f '{{.Dir}}' github.com/sdimitro/crashfp)/dmesgcrash/testdata/corpus
[ -d "$CORPUS_DIR" ] || { echo "FAIL: corpus not found at $CORPUS_DIR"; exit 1; }

# Match every corpus crash: skip-list entries are derived from each
# sample's .expected fingerprints, exercising the full match path.
SKIPLIST="$WORK/skiplist"
echo "# generated 2026-01-01T00:00:00Z from PARITY" > "$SKIPLIST"
for exp in "$CORPUS_DIR"/*.expected; do
    type_rip=$(sed -n 's/^type_rip=\(.\{16\}\).*/\1/p' "$exp")
    top3=$(sed -n 's/^top3=\(.\{16\}\).*/\1/p' "$exp")
    echo "fp-type-rip:$type_rip fp-top3:$top3 PARITY-1" >> "$SKIPLIST"
done

# Fail-open inputs.
printf 'not a kernel log at all\n' > "$WORK/garbage.txt"
: > "$WORK/empty.txt"
printf 'fp-type-rip:NOTHEX\n' > "$WORK/badlist"

fail=0

# run <name> <expected-note: yes|no> [args...]
run() {
    name=$1; note_expected=$2; shift 2

    for flavor in stock tiny; do
        rm -f "$WORK/$flavor-note.json"
        set +e
        "$WORK/$flavor" --note "$WORK/$flavor-note.json" "$@" \
            > "$WORK/$flavor-stdout" 2> "$WORK/$flavor-stderr"
        echo $? > "$WORK/$flavor-exit"
        set -e
    done

    if ! cmp -s "$WORK/stock-exit" "$WORK/tiny-exit"; then
        echo "FAIL [$name]: exit codes differ: stock=$(cat "$WORK/stock-exit") tiny=$(cat "$WORK/tiny-exit")"
        fail=1
        return
    fi

    if [ "$note_expected" = yes ]; then
        for flavor in stock tiny; do
            [ -f "$WORK/$flavor-note.json" ] || { echo "FAIL [$name]: $flavor wrote no note"; fail=1; return; }
            # checked_at is wall-clock time; normalize it before diffing.
            sed 's/"checked_at": "[^"]*"/"checked_at": "X"/' "$WORK/$flavor-note.json" > "$WORK/$flavor-note-norm"
        done
        if ! cmp -s "$WORK/stock-note-norm" "$WORK/tiny-note-norm"; then
            echo "FAIL [$name]: notes differ:"
            diff "$WORK/stock-note-norm" "$WORK/tiny-note-norm" || true
            fail=1
            return
        fi
    fi

    echo "ok [$name]: exit=$(cat "$WORK/stock-exit")"
}

for log in "$CORPUS_DIR"/*.txt; do
    run "match $(basename "$log")" yes --skiplist "$SKIPLIST" "$log"
    run "no-match $(basename "$log")" no "$log"
done

run "garbage input" no "$WORK/garbage.txt"
run "empty input" no "$WORK/empty.txt"
run "missing dmesg" no "$WORK/does-not-exist.txt"
run "malformed extra skiplist" no --skiplist "$WORK/badlist" "$WORK/garbage.txt"
run "missing extra skiplist" no --skiplist "$WORK/no-such-list" "$WORK/garbage.txt"

[ $fail -eq 0 ] && echo "PASS: stock and tiny builds behave identically"
exit $fail
