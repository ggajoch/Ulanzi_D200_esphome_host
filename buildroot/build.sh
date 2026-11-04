#!/bin/bash
# Builds the firmware: upstream d200-buildroot at the commit in UPSTREAM, with
# patches/ on top, in Docker (debug flavour). Linux, or WSL2 with the work dir
# on the Linux file system (not /mnt/c).
#   ./build.sh [work dir]      default: ~/d200-buildroot-esphome
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=UPSTREAM
. "$here/UPSTREAM"
work=${1:-$HOME/d200-buildroot-esphome}

[ -d "$work/.git" ] || git clone "$UPSTREAM_URL" "$work"
cd "$work"
git am --abort >/dev/null 2>&1 || true
git cat-file -e "$UPSTREAM_COMMIT^{commit}" 2>/dev/null || git fetch origin
git checkout -q -B esphome "$UPSTREAM_COMMIT"
# A fixed committer and dates: the version and the U-Boot and kernel build
# dates come from the commits, so a rebuild gives the same images and an
# update skips an unchanged bootloader
git -c user.name="d200-esphome build" -c user.email=build@localhost \
	am -q --committer-date-is-author-date "$here"/patches/*.patch

make build FLAVOR=debug
echo "Images: $work/$(ls -td output/*-debug | head -n 1)"
