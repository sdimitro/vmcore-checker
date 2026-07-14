#!/bin/sh
# Build a Debian package around a prebuilt vmcore-checker binary.
#
# Usage: scripts/build-deb.sh <binary> <version> <deb-arch> [outdir]
#
#   binary   prebuilt linux binary to package (e.g. vmcore-checker-linux-amd64)
#   version  package version without the leading "v" (e.g. 0.1.2)
#   deb-arch Debian architecture (amd64 or arm64)
#   outdir   where to write the .deb (default: current directory)
#
# Produces <outdir>/vmcore-checker_<version>_<deb-arch>.deb. Must run
# from the repository root; needs dpkg-deb (any platform).
set -eu

BIN=$1
VERSION=$2
ARCH=$3
OUTDIR=${4:-.}

STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT
chmod 0755 "$STAGE"

mkdir -p \
    "$STAGE/DEBIAN" \
    "$STAGE/usr/bin" \
    "$STAGE/usr/share/doc/vmcore-checker/examples"

cp "$BIN" "$STAGE/usr/bin/vmcore-checker"
chmod 0755 "$STAGE/usr/bin/vmcore-checker"

cp LICENSE "$STAGE/usr/share/doc/vmcore-checker/copyright"
cp README.md "$STAGE/usr/share/doc/vmcore-checker/README.md"
cp scripts/kdump-precapture-hook.sh "$STAGE/usr/share/doc/vmcore-checker/examples/"
chmod 0644 "$STAGE/usr/share/doc/vmcore-checker/copyright" \
    "$STAGE/usr/share/doc/vmcore-checker/README.md"
chmod 0755 "$STAGE/usr/share/doc/vmcore-checker/examples/kdump-precapture-hook.sh"

cat > "$STAGE/DEBIAN/control" <<EOF
Package: vmcore-checker
Version: $VERSION
Architecture: $ARCH
Maintainer: Serapheim Dimitropoulos <sdimitropoulos@coreweave.com>
Section: admin
Priority: optional
Homepage: https://github.com/sdimitro/vmcore-checker
Description: skip kdump vmcore capture for known kernel crashes
 vmcore-checker runs inside the kdump crash kernel. It fingerprints the
 crash from the extracted kernel log and compares it against a curated
 skip-list of known issues embedded in the binary. On a match it writes
 a small note.json and exits 0 so the kdump hook can skip the expensive
 vmcore capture and reboot immediately. It fails open: any error means
 the dump is captured normally.
EOF

mkdir -p "$OUTDIR"
dpkg-deb --build --root-owner-group "$STAGE" "$OUTDIR/vmcore-checker_${VERSION}_${ARCH}.deb"
