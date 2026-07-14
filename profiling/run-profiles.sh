#!/bin/sh
# Run the memory measurement matrix on a Linux host (bare or container).
#
# Expects a work directory laid out as:
#   ./vmcore-checker-prof         stock Go binary,   -tags memprofile
#   ./vmcore-checker-tiny-prof    TinyGo binary,     -tags memprofile
#   ./inputs/*.txt                dmesg inputs
#   ./skiplist                    skip-list matching the corpus crashes
#
# Usage: run-profiles.sh <workdir> <resultsdir>
#
# For every input x binary it records GNU time -v output (peak RSS) and
# a -memstats dump; for the stock binary it also writes heap profiles
# (analyzed later with `go tool pprof` wherever Go is installed) and a
# GODEBUG=gctrace log, plus GC-tuning probes on the largest input.
set -eu

WORK=$1
RES=$2
TIME=/usr/bin/time

cd "$WORK"
mkdir -p "$RES"

run_one() { # binary label input extra_env...
    bin=$1 label=$2 input=$3; shift 3
    in_name=$(basename "$input" .txt)
    out="$RES/$label-$in_name"

    # shellcheck disable=SC2086
    env "$@" $TIME -v "./$bin" --skiplist skiplist --memstats \
        --note "$out.note.json" "$input" > "$out.stdout" 2> "$out.time" || true
    rss_kb=$(sed -n 's/.*Maximum resident set size (kbytes): //p' "$out.time")
    echo "$label $in_name rss_kb=$rss_kb"
}

for input in inputs/*.txt; do
    in_name=$(basename "$input" .txt)

    run_one vmcore-checker-tiny-prof tiny "$input"
    run_one vmcore-checker-prof stock "$input"

    # Stock-only: heap profile (cumulative allocation samples) and gctrace.
    $TIME -v ./vmcore-checker-prof --skiplist skiplist \
        --memprofile "$RES/stock-$in_name.heap.pb.gz" \
        --note /dev/null "$input" > /dev/null 2> "$RES/stock-$in_name.memprofile.time" || true
    GODEBUG=gctrace=1 ./vmcore-checker-prof --skiplist skiplist \
        --note /dev/null "$input" > /dev/null 2> "$RES/stock-$in_name.gctrace" || true
done

# GC-tuning probes on the largest input: how much of the stock peak is
# GC timing vs. live data. (TinyGo has no equivalent knobs.)
largest=$(ls -S inputs/*.txt | head -1)
for probe in "GOGC=25" "GOGC=off" "GOMEMLIMIT=8MiB"; do
    tag=$(echo "$probe" | tr '=' '-' | tr '[:upper:]' '[:lower:]')
    run_one vmcore-checker-prof "stock-$tag" "$largest" "$probe"
done

echo "results in $RES"
