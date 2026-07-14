#!/bin/sh
# Behavioral parity gate between the stock Go build and the TinyGo
# build, and between the two input modes (default whole-file load vs
# --stream line-by-line).
#
# Builds both compilers' binaries for the host platform, runs each in
# both modes over every golden corpus kernel log plus the fail-open
# cases, and requires identical exit codes and byte-identical note.json
# output (modulo the checked_at timestamp) across all four combinations.
# Fingerprint hashes are deterministic, so any divergence is a hard
# failure — no build or mode may skip a different set of dumps.
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
# Executes all four combinations (stock/tiny x default/--stream) and
# compares every one against stock-default.
run() {
    name=$1; note_expected=$2; shift 2

    for flavor in stock tiny; do
        for mode in default stream; do
            id="$flavor-$mode"
            modeflag=""
            [ "$mode" = stream ] && modeflag="--stream"
            rm -f "$WORK/$id-note.json"
            set +e
            # shellcheck disable=SC2086
            "$WORK/$flavor" $modeflag --note "$WORK/$id-note.json" "$@" \
                > "$WORK/$id-stdout" 2> "$WORK/$id-stderr"
            echo $? > "$WORK/$id-exit"
            set -e
            sed 's/"checked_at": "[^"]*"/"checked_at": "X"/' \
                "$WORK/$id-note.json" > "$WORK/$id-note-norm" 2>/dev/null || : > "$WORK/$id-note-norm"
        done
    done

    for id in stock-stream tiny-default tiny-stream; do
        if ! cmp -s "$WORK/stock-default-exit" "$WORK/$id-exit"; then
            echo "FAIL [$name]: exit codes differ: stock-default=$(cat "$WORK/stock-default-exit") $id=$(cat "$WORK/$id-exit")"
            fail=1
            return
        fi
        if ! cmp -s "$WORK/stock-default-note-norm" "$WORK/$id-note-norm"; then
            echo "FAIL [$name]: notes differ (stock-default vs $id):"
            diff "$WORK/stock-default-note-norm" "$WORK/$id-note-norm" || true
            fail=1
            return
        fi
    done

    if [ "$note_expected" = yes ] && [ ! -s "$WORK/stock-default-note-norm" ]; then
        echo "FAIL [$name]: expected a note but none was written"
        fail=1
        return
    fi

    echo "ok [$name]: exit=$(cat "$WORK/stock-default-exit") (4 variants agree)"
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
