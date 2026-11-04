#!/usr/bin/env bash
# Build (build.sh) and push the ESPHome firmware to the D200 by ESPHome OTA. Run on Linux/WSL.
#   DEVICE=<D200 IP or hostname> ./deploy.sh
# The daemon (out/ulanzi-control) is not part of the OTA: copy it to
# /data/ulanzi-control/ and restart S70esphome when it changes.
set -euo pipefail
DEVICE=${DEVICE:?set DEVICE to the D200 IP address or hostname}
SRC=$(cd "$(dirname "$0")" && pwd)
WORK=${WORK:-$HOME/d200/esphome}
bash "$SRC/build.sh"
"$WORK/venv/bin/esphome" upload "$WORK/build/examples/host/d200.yaml" --device "$DEVICE"
