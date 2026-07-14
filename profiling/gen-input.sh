#!/bin/sh
# Generate a synthetic kernel log for memory profiling: a banner and
# command line up top, boot/runtime filler noise with realistic
# "[ ts ] [ Tn ]" prefixes up to the requested size, and a real
# golden-corpus crash report (stack trace included) at the end — the
# worst realistic case for vmcore-checker: a huge log whose crash region
# sits at the very tail.
#
# Usage: profiling/gen-input.sh <size-mb> <outfile> [corpus-sample]
# The default sample is resolved from the crashfp module in the module
# cache, so this must run from the repository root.
set -eu

SIZE_MB=$1
OUT=$2
SAMPLE=${3:-$(go list -m -f '{{.Dir}}' github.com/sdimitro/crashfp)/dmesgcrash/testdata/corpus/arm64-nvidia-nullptr.txt}

[ -f "$SAMPLE" ] || { echo "sample not found: $SAMPLE" >&2; exit 1; }

# Reserve room for the crash sample at the tail.
sample_bytes=$(wc -c < "$SAMPLE")
filler_bytes=$((SIZE_MB * 1048576 - sample_bytes))

{
    printf '[    0.000000] [    T0] Linux version 6.14.0-1008-nvidia-64k (buildd@bos03-arm64-088) (gcc 13.3.0) #8-Ubuntu SMP PREEMPT_DYNAMIC\n'
    printf '[    0.000000] [    T0] Kernel command line: vmlinuz root=UUID=abc ro panic=10 crashkernel=512M\n'

    # Filler: rotating boot/runtime noise, none of which contains a
    # crash keyword. Each line is ~90 bytes.
    awk -v bytes="$filler_bytes" 'BEGIN {
        n = 0; total = 0;
        while (total < bytes) {
            ts = sprintf("[%5d.%06d]", 10 + n / 1000, n % 1000000);
            tid = sprintf("[T%5d]", 1000 + n % 9000);
            m = n % 4;
            if (m == 0)      msg = "systemd[1]: Started Session " n " of User acc.";
            else if (m == 1) msg = "NVRM: GPU 0008:06:00.0: RmInitAdapter succeeded for minor " n ".";
            else if (m == 2) msg = "mlx5_core 0000:01:00." (n % 8) ": firmware event queue drained, cycle " n;
            else             msg = "audit: type=1400 apparmor=ALLOWED operation=open pid=" n " comm=fio";
            line = ts " " tid " " msg;
            print line;
            total += length(line) + 1;
            n++;
        }
    }'

    cat "$SAMPLE"
} > "$OUT"

wc -c "$OUT"
