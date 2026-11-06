#!/usr/bin/env bash
# Build the D200 stack on Linux (WSL works), for the device's aarch64 glibc userspace,
# from this repo's components/ and daemon/:
#   $WORK/out/d200            ESPHome host firmware (d200.yaml + ulanzi_d200)
#   $WORK/out/ulanzi-control  display/keys daemon (static Go binary)
#
#   ./build.sh
#   WORK=... BR_HOST=... GOROOT_DIR=... ./build.sh
set -euo pipefail

SRC=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$SRC/../.." && pwd)
WORK=${WORK:-$HOME/d200/esphome}
# Buildroot host dir of the D200 image (../../buildroot/README.md): same glibc as the device. Used read-only.
BR_HOST=${BR_HOST:-$HOME/d200/d200-buildroot/BuildEnv/output-debug/host}
ESPHOME_VERSION=${ESPHOME_VERSION:-2026.10.2}
GOROOT_DIR=${GOROOT_DIR:-$HOME/d200/go}
GO_VERSION=1.27.3
GO_SHA256=65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40
# Same OpenSSL as Buildroot's libopenssl package
OPENSSL_VERSION=3.6.5
OPENSSL_SHA256=a2157c2830efdec3788939b00c9b0638306d3f0bbb76dc4832ee503bb397df98

T=$BR_HOST/bin
[ -x "$T/aarch64-linux-g++" ] || { echo "no Buildroot toolchain at $T" >&2; exit 1; }
mkdir -p "$WORK/build" "$WORK/out"

if [ ! -x "$WORK/venv/bin/esphome" ]; then
  python3 -m venv "$WORK/venv"
  "$WORK/venv/bin/pip" install -q "esphome==$ESPHOME_VERSION"
fi

if [ "$("$GOROOT_DIR/bin/go" env GOVERSION 2>/dev/null)" != "go$GO_VERSION" ]; then
  curl -fsSLo "$WORK/go.tgz" "https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz"
  echo "$GO_SHA256  $WORK/go.tgz" | sha256sum -c -
  rm -rf "$GOROOT_DIR" && mkdir -p "$GOROOT_DIR"
  tar xzf "$WORK/go.tgz" -C "$GOROOT_DIR" --strip-components=1 && rm "$WORK/go.tgz"
fi

# OTA pulls in ESPHome's md5/sha256, which on host use OpenSSL libcrypto (-lcrypto).
# The image has no OpenSSL, so link a static libcrypto.a.
OSSL=$WORK/openssl/aarch64
if [ ! -f "$OSSL/lib/libcrypto.a" ]; then
  mkdir -p "$WORK/openssl" && cd "$WORK/openssl"
  tarball=openssl-$OPENSSL_VERSION.tar.gz
  [ -f "$tarball" ] || curl -fsSLo "$tarball" \
    "https://github.com/openssl/openssl/releases/download/openssl-$OPENSSL_VERSION/$tarball"
  echo "$OPENSSL_SHA256  $tarball" | sha256sum -c -
  rm -rf src && mkdir src && tar xzf "$tarball" -C src --strip-components=1 && cd src
  ./Configure linux-aarch64 no-shared no-module no-tests no-docs no-apps no-engine no-dso no-async \
    --prefix="$OSSL" --libdir=lib \
    CC="$T/aarch64-linux-gcc" AR="$T/aarch64-linux-ar" RANLIB="$T/aarch64-linux-ranlib"
  make -j"$(nproc)" build_libs
  make install_dev
fi

# Daemon: pure Go, static. No VCS stamp: git may refuse a checkout on /mnt/c (dubious ownership)
(cd "$REPO/daemon" && PATH=$GOROOT_DIR/bin:$PATH GOTOOLCHAIN=local GOPATH=$WORK/gopath GOCACHE=$WORK/gocache \
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags='-s -w' \
  -o "$WORK/out/ulanzi-control" ./cmd/ulanzi-control)

# The API/OTA key lives outside git; make one on first build
if [ ! -f "$SRC/secrets.yaml" ]; then
  echo "api_encryption_key: \"$(head -c32 /dev/urandom | base64)\"" > "$SRC/secrets.yaml"
  echo "generated $SRC/secrets.yaml"
fi

# Build on ext4, not on /mnt/c: the component and the config, in the repo's layout
# (d200.yaml finds the component at ../../components)
B=$WORK/build
rm -rf "$B/components" && cp -r "$REPO/components" "$B/"
mkdir -p "$B/examples/host" && cp "$SRC"/*.yaml "$SRC"/*.h "$B/examples/host/"   # d200.yaml, its packages and header, secrets.yaml

# The host platform compiles with whatever CC/CXX/AR/... say (ninja, no PlatformIO);
# extra words in CC/CXX reach every compile and the link. Optimisation and static
# libstdc++ come from esphome.build_flags in d200.yaml.
export CC="$T/aarch64-linux-gcc -I$OSSL/include -L$OSSL/lib" \
       CXX="$T/aarch64-linux-g++ -I$OSSL/include -L$OSSL/lib" \
       AR=$T/aarch64-linux-ar OBJDUMP=$T/aarch64-linux-objdump READELF=$T/aarch64-linux-readelf
"$WORK/venv/bin/esphome" compile "$B/examples/host/d200.yaml"

cp "$B/examples/host/.esphome/build/d200/.pioenvs/d200/program" "$WORK/out/d200"
"$READELF" -d "$WORK/out/d200" | grep NEEDED
ls -l "$WORK/out/d200" "$WORK/out/ulanzi-control"
