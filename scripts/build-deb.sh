#!/bin/sh
# Build a Debian package around a prebuilt vmcore-checker binary.
#
# Usage: scripts/build-deb.sh <binary> <version> <deb-arch> [outdir] [package]
#
#   binary   prebuilt linux binary to package (e.g. vmcore-checker-linux-amd64)
#   version  package version without the leading "v" (e.g. 0.1.2)
#   deb-arch Debian architecture (amd64 or arm64)
#   outdir   where to write the .deb (default: current directory)
#   package  vmcore-checker (default) or vmcore-checker-tiny
#
# Both packages install /usr/bin/vmcore-checker; the -tiny one wraps the
# TinyGo build and Conflicts/Replaces/Provides the stock package, so
# exactly one can be installed and hooks always find the same path.
#
# Produces <outdir>/<package>_<version>_<deb-arch>.deb. Must run from
# the repository root; needs dpkg-deb (any platform).
set -eu

BIN=$1
VERSION=$2
ARCH=$3
OUTDIR=${4:-.}
PACKAGE=${5:-vmcore-checker}

case $PACKAGE in
vmcore-checker) EXTRA_CONTROL="" ;;
vmcore-checker-tiny)
    EXTRA_CONTROL="Conflicts: vmcore-checker
Replaces: vmcore-checker
Provides: vmcore-checker" ;;
*)
    echo "unknown package name: $PACKAGE" >&2
    exit 1 ;;
esac

STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT
chmod 0755 "$STAGE"

mkdir -p \
    "$STAGE/DEBIAN" \
    "$STAGE/usr/bin" \
    "$STAGE/usr/share/doc/$PACKAGE/examples"

cp "$BIN" "$STAGE/usr/bin/vmcore-checker"
chmod 0755 "$STAGE/usr/bin/vmcore-checker"

cp LICENSE "$STAGE/usr/share/doc/$PACKAGE/copyright"
cp README.md "$STAGE/usr/share/doc/$PACKAGE/README.md"
cp scripts/kdump-precapture-hook.sh "$STAGE/usr/share/doc/$PACKAGE/examples/"
chmod 0644 "$STAGE/usr/share/doc/$PACKAGE/copyright" \
    "$STAGE/usr/share/doc/$PACKAGE/README.md"
chmod 0755 "$STAGE/usr/share/doc/$PACKAGE/examples/kdump-precapture-hook.sh"

cat > "$STAGE/DEBIAN/control" <<EOF
Package: $PACKAGE
Version: $VERSION
Architecture: $ARCH
Maintainer: Serapheim Dimitropoulos <sdimitro@users.noreply.github.com>
Section: admin
Priority: optional
Homepage: https://github.com/sdimitro/vmcore-checker
${EXTRA_CONTROL:+$EXTRA_CONTROL
}Description: skip kdump vmcore capture for known kernel crashes
 vmcore-checker runs inside the kdump crash kernel. It fingerprints the
 crash from the extracted kernel log and compares it against a curated
 skip-list of known issues embedded in the binary. On a match it writes
 a small note.json and exits 0 so the kdump hook can skip the expensive
 vmcore capture and reboot immediately. It fails open: any error means
 the dump is captured normally.
EOF

mkdir -p "$OUTDIR"
dpkg-deb --build --root-owner-group "$STAGE" "$OUTDIR/${PACKAGE}_${VERSION}_${ARCH}.deb"
