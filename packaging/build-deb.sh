#!/bin/sh
set -eu

version=${1:?usage: packaging/build-deb.sh VERSION}
case "$version" in
    '' | [!0-9]* | *[!0-9A-Za-z.+:~_-]*)
        echo "invalid Debian version: $version" >&2
        exit 2
        ;;
esac

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
arch=$(dpkg --print-architecture)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

make -C "$root" build
make -C "$root" install DESTDIR="$stage"
mkdir -p "$stage/DEBIAN" "$root/dist"
sed -e "s/@VERSION@/$version/g" -e "s/@ARCH@/$arch/g" \
    "$root/packaging/debian/control.in" > "$stage/DEBIAN/control"
find "$stage" -type d -exec chmod 755 {} +
dpkg-deb --root-owner-group --build "$stage" \
    "$root/dist/network-manager-defguard_${version}_${arch}.deb"
