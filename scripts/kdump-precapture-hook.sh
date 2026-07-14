#!/bin/sh
# kdump pre-capture hook: skip vmcore capture for known crashes.
#
# Runs inside the crash kernel before makedumpfile. Extracts the crashed
# kernel's log from /proc/vmcore, fingerprints the crash with
# vmcore-checker, and — when it matches the skip-list baked into the
# binary — saves only the dmesg + a small note.json and reboots, instead
# of capturing the multi-GB vmcore.
#
# Fail-open: any failure in this script must end in normal dump capture.
# Wire this into your kdump tooling (dracut kdump module hook or custom
# initramfs script) so that exiting 0 skips capture and exiting non-zero
# proceeds with it, or adapt the reboot/capture commands below.
set -u

DUMP_DIR="${DUMP_DIR:-/kdumproot}"
DMESG_OUT="$DUMP_DIR/dmesg.txt"
NOTE_OUT="$DUMP_DIR/note.json"
EXTRA_SKIPLIST="${EXTRA_SKIPLIST:-/etc/vmcore-checker/skiplist.d/extra}"

# Extract the crashed kernel's log. Always keep it, match or not — it is
# cheap and useful either way. vmcore-dmesg ships with kexec-tools;
# makedumpfile --dump-dmesg is the fallback.
if command -v vmcore-dmesg >/dev/null 2>&1; then
    vmcore-dmesg /proc/vmcore > "$DMESG_OUT" 2>/dev/null || exit 1
else
    makedumpfile --dump-dmesg /proc/vmcore "$DMESG_OUT" >/dev/null 2>&1 || exit 1
fi

# Only pass --skiplist when the extras file exists: a present-but-broken
# file is a hard error inside vmcore-checker (fail-open), while an absent
# one simply means "embedded list only".
set -- --note "$NOTE_OUT"
[ -f "$EXTRA_SKIPLIST" ] && set -- --skiplist "$EXTRA_SKIPLIST" "$@"

if vmcore-checker "$@" "$DMESG_OUT"; then
    # Known crash: dmesg + note are saved; caller should upload
    # $DUMP_DIR contents (note lands as <serial>-<ts>-note.json in the
    # kdump bucket) and reboot without capturing the vmcore.
    exit 0
fi

# Unknown crash or any error: proceed with normal capture.
exit 1
