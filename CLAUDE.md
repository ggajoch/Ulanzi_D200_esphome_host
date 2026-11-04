# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ulanzi_D200_esphome_host: a fork of mapero/esphome-ulanzi-d200 that runs ESPHome on the **Ulanzi D200** stream deck's own RK3308 (ESPHome `host` platform, aarch64 Linux) instead of on an external ESP32. The D200 has a 14-button grid (5x3 layout, button 13 is double-wide).

Two variants share the component and the daemon:
- **Host** (this fork): open firmware from `buildroot/` (d200-buildroot, mainline kernel, arm64 userspace). ESPHome (`examples/host/d200.yaml`) and the daemon run on the D200, joined by a socat pty pair (`/run/d200-esp` ⇄ `/run/d200-daemon`) that `S70esphome` starts. Home Assistant connects over WiFi.
- **ESP32** (upstream): stock firmware (ARMv7 userspace) with the daemon; an ESP32 on the debug UART (`/dev/ttyFIQ0`) runs ESPHome. `examples/*.yaml`, `docs/d200-preparation.md`, `firmware/`, `install.*`.

Subsystems:
- **ESPHome component** (`components/ulanzi_d200/`) — Python schema + C++ bridge logic; the same code in both variants
- **Go daemon** (`daemon/`) — runs on the D200, renders the UI on `/dev/fb0` and reads the keys
- **Firmware** (`buildroot/`) — patches and build script for d200-buildroot. GPL-2.0-or-later, maintained separately; the rest of the repo is MIT

Communication: NDJSON over a UART (ESP32 variant) or a pty (host variant) at 1.5 Mbps.

## Build Commands

### Host variant

```bash
examples/host/build.sh                    # Linux/WSL: ESPHome + daemon -> ~/d200/esphome/out/{d200,ulanzi-control}
DEVICE=<D200 IP> examples/host/deploy.sh  # build, then ESPHome OTA of the d200 binary
```

`build.sh` needs the Buildroot host toolchain of the D200 image (`BR_HOST`): the binary must match the device's glibc. Keep its flags: `-O2`, static libstdc++/libgcc, a static OpenSSL `libcrypto` for OTA, stripped (`esphome.build_flags` in `d200.yaml` plus `CC`/`CXX` in `build.sh`). It compiles a copy on ext4 (`~/d200/esphome/build/`), never on `/mnt/c`. The OTA carries only ESPHome: a daemon change is copied to `/data/ulanzi-control/` by hand.

`examples/host/secrets.yaml` holds the real API/OTA key and is git-ignored; `backup/` holds stock firmware dumps and is git-ignored. Commit neither.

### ESP32 Component (ESPHome)

```bash
# Compile firmware (from repo root or examples dir)
esphome compile examples/basic.yaml

# Flash to device
esphome run examples/basic.yaml

# View logs
esphome logs examples/basic.yaml
```

Requires `secrets.yaml` in `examples/` — copy from `examples/secrets.yaml.example`.

### Go Daemon

```bash
# Build for the stock D200 firmware (ARMv7 Linux, ESP32 variant)
cd daemon && GOOS=linux GOARCH=arm GOARM=7 go build -o ulanzi-control ./cmd/ulanzi-control/

# Build for d200-buildroot (arm64, host variant; examples/host/build.sh does this)
cd daemon && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o ulanzi-control ./cmd/ulanzi-control/

# Build for local development
cd daemon && go build -o ulanzi-control ./cmd/ulanzi-control/

# Deploy to a stock D200 via ADB (ESP32 variant)
adb push daemon/ulanzi-control /userdata/ulanzi-control/
adb push daemon/deploy/S95ulanzi /userdata/ulanzi-control/
adb push daemon/deploy/ulanzi-run.sh /userdata/ulanzi-control/

# /etc/init.d is read-only; create a symlink instead
adb shell ln -sf /userdata/ulanzi-control/S95ulanzi /etc/init.d/S95ulanzi
```

Go 1.22+ required. Dependencies: `golang.org/x/image`, `go.bug.st/serial`.

## Architecture

### Concept Model

```
ulanzi_d200 (1 per device)
  └── sections[] (physical button positions 0–13)
        ├── position, page (-1 = all pages, 0+ = specific page)
        ├── on_press / on_release (ESPHome automations)
        └── layouts[] (conditional display states, first match wins)
              ├── when (lambda condition, evaluated every 1s)
              ├── widget type (entity_view | gauge | line_graph | status | chips | notification)
              └── properties (text, color, icon_id, etc. — static or lambda)
```

### ESPHome Component (C++/Python)

- `__init__.py` — ESPHome YAML schema validation and C++ code generation. Defines all widget types and their properties. Processes lambda vs static string properties.
- `ulanzi_d200.h` — `UlanziBridge` class (extends `Component`, `UARTDevice`). Also declares `UlanziBacklightLight` (brightness-only light entity), trigger classes, and data structures for sections/layouts/widgets.
- `ulanzi_d200.cpp` — Core implementation:
  - **Layout evaluation**: runs every 1s, evaluates layout conditions top-to-bottom, sends first match
  - **Template caching**: properties only sent over UART when changed (dirty flag)
  - **Batch command queue**: max 4 commands/batch, 3s response timeout, flow control via in_flight flag
  - **UART parsing**: handles NDJSON responses, robust against kernel printk noise
  - **Page navigation**: `navigate_page_next()`, `navigate_page_prev()`, `navigate_page_jump(n)`
  - **HA integration**: exposes brightness as light entity, auto-registers notification services

### Go Daemon

- `cmd/ulanzi-control/main.go` — Entry point. Framebuffer rendering (720x1280 @ 32bpp, geometry read from the fb ioctls), button input from `/dev/input/event0` (`struct input_event` size follows the architecture: 24 bytes on arm64). Default key map: the mainline kernel's KEY_F13–F24, KEY_PROG1/2; the stock firmware's codes need a `config/keymap.json` (calibration). Clock time zone: `TZ`, else `config/timezone` (Go tzdata is embedded; the image has none).
- `pkg/state/store.go` — Global state singleton. ButtonConfig/ButtonState management, RingBuffer for time-series data, thread-safe (RWMutex), state persistence to `config/state.json`.
- `pkg/transport/commands.go` — NDJSON protocol implementation. Modules: `display`, `sensor`, `page`, `system`. Error codes: OK=0, ERROR=1, INVALID=2, TIMEOUT=3, INTERNAL=5.
- `pkg/transport/serial.go` — Serial communication (1.5 Mbps); the port is `ULANZI_SERIAL`, default `/dev/ttyFIQ0`.
- `pkg/ui/draw.go` + `font.go` — Display rendering pipeline with per-button dirty flags.

### NDJSON Protocol

ESPHome sends commands: `{"type":"command", "module":"display", "action":"set_button_config", "params":{...}}`
D200 responds: `{"type":"response", "status":"ok", "code":0, ...}`
D200 sends events: `{"type":"event", "event":"page_change", ...}` for button presses, page changes, mode changes.

### Key Patterns

- **Conditional rendering**: layouts evaluated top-to-bottom, first matching `when` wins; layout without `when` is the fallback
- **All string properties** accept static strings or `!lambda` returning `optional<std::string>`
- **HA state sync**: template switch (optimistic, calls HA service) + binary_sensor (syncs real state back) — see `docs/component.md` for the pattern
- **Page -1**: sections visible on all pages (navigation bars, status bars)
