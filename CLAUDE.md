# CLAUDE.md

Ulanzi_D200_esphome_host: a fork of mapero/esphome-ulanzi-d200 that runs ESPHome on the **Ulanzi D200**'s own
RK3308 (ESPHome `host` platform, aarch64 Linux) instead of on an external ESP32. Home Assistant talks to it over
WiFi (USB WiFi dongle on a powered USB-C hub). 14 keys, 5x3 grid, key 13 is double-wide.

```
HA ⇄ WiFi ⇄ ESPHome (/data/esphome/d200) ⇄ socat pty (/run/d200-esp ⇄ /run/d200-daemon) ⇄ ulanzi-control → /dev/fb0, /dev/input/event0, backlight
```

Two variants share the component and the daemon: **host** (this fork: `buildroot/` firmware, `examples/host/`) and
**ESP32** (upstream: stock firmware, ESP32 on the debug UART; `examples/*.yaml`, `docs/d200-preparation.md`,
`firmware/`, `install.*`). Keep the ESP32 variant building; the daemon's default key map covers both firmwares.

## Where to change what

| To change | Edit | Ship it with |
|---|---|---|
| Keys, pages, HA entities, labels, colours, hold-to-activate | `examples/host/d200.yaml` (entities and labels in `substitutions:`), `examples/host/d200-toggle.yaml` (one toggle key), `d200-vacuum.yaml` (one vacuum key) | ESPHome deploy |
| Widget types, YAML schema, layout logic, HA services of the component | `components/ulanzi_d200/` (`__init__.py` schema/codegen, `.h`/`.cpp` runtime), document in `docs/configuration.md` | ESPHome deploy |
| Drawing, fonts, icons, key-code map, status bar, clock | `daemon/` (`pkg/ui/`, `cmd/ulanzi-control/main.go`) | daemon deploy |
| NDJSON protocol | both sides together: `components/ulanzi_d200/ulanzi_d200.cpp` and `daemon/pkg/transport/commands.go` | both deploys |
| OS: kernel config, drivers, DT, init scripts, USB mode, WiFi, NTP, SSH, packages | d200-buildroot dev tree → `buildroot/patches/` | firmware workflow |
| Per-device settings | on the D200: `/data/wpa_supplicant.conf`, `/data/ntp.conf`, `/data/ulanzi-control/config/timezone`, `/data/esphome/mdns.txt` | edit in place, restart the service |

## Workflows

Builds need Linux (or WSL2) with the work dirs on a Linux file system. Machine-specific setup, if any, is in the
git-ignored `CLAUDE.local.md`.

**ESPHome deploy** (YAML or component changes): `DEVICE=<ip> bash examples/host/deploy.sh` builds and OTAs
(about 1.5 min). `examples/host/build.sh` alone builds both binaries into `$WORK/out/`. Set `BR_HOST` to the
firmware build's Buildroot host dir (`<firmware work tree>/BuildEnv/output-debug/host`), so the binary matches the
device's glibc. Keep its flags: `-O2`, static libstdc++/libgcc, static OpenSSL `libcrypto` (needed by OTA), stripped.

**Daemon deploy**: the OTA carries only ESPHome. Copy `$WORK/out/ulanzi-control` to `/data/ulanzi-control/`, then
`kill $(pidof ulanzi-control)`; `S70esphome` restarts it.

**Firmware workflow** (`buildroot/` = patch series on riptidewave93/d200-buildroot, pin in `buildroot/UPSTREAM`):
1. Change and commit in a d200-buildroot work tree with the series applied (`buildroot/build.sh` creates one),
   following that repo's `AGENTS.md`. Build with `make build FLAVOR=debug` (or `make linux-rebuild`, etc.).
2. Re-export the whole series: `rm buildroot/patches/*.patch; git format-patch --binary <UPSTREAM_COMMIT>..HEAD -o <repo>/buildroot/patches`, then commit here.
3. Install the `output/<date>-debug/*-update.tar` through deckd's update page:
   - over SSH: `/etc/init.d/S70esphome stop; /etc/init.d/S60deckd start` (the page only runs with deckd);
   - upload: `curl -H "Expect:" -H "X-Deck-Update: 1" -T <tar> http://<ip>/update/bundle`, with a long `-m`, because 20 MB over weak WiFi can take minutes;
   - someone must press **Install** (wide key, bottom right) within 2 min of the prompt; poll `http://<ip>/update/status`.
   A U-Boot change makes the update rewrite U-Boot. First install from stock firmware (maskrom, usbipd on Windows):
   `buildroot/README.md`.

## Device

- **SSH:** `root`, no password, over every interface (USB, WiFi, Ethernet): the device belongs on a trusted LAN.
- **USB modes:** in USB host mode (default, `usb_mode=host` in the U-Boot env) the device is on WiFi/Ethernet. In USB
  device mode (`fw_setenv usb_mode device`, reboot) the PC sees a USB network link instead. Recovery:
  `buildroot/README.md`.
- **Logs:** `/var/log/messages`, tags `esphome`, `ulanzi`, `socat`, `ntpd`, `deckd`. Stack:
  `/etc/init.d/S70esphome start|stop|restart`. The rootfs is read-only; everything persistent lives in `/data`.

## Invariants

- **HA identity:** keep `esphome: name`, `host: mac_address` and the API key in `examples/host/d200.yaml` unchanged,
  or Home Assistant sees a new device. `mac=` in `/data/esphome/mdns.txt` must match `host: mac_address`.
- **Compressor and PC keys:** hold-only (1 s), so they never fire by accident. Lock keys only lock: the D200 hangs outside.
- **Secrets stay out of git:** `examples/host/secrets.yaml` (real API/OTA key) and `backup/` (stock firmware dump) are
  git-ignored.
- **Line endings:** `.gitattributes` forces LF (bash in WSL rejects CRLF). Commit new scripts with
  `git add --chmod=+x`. `buildroot/patches/*.patch` are `-text`, so `git am` gets them byte for byte.
- **WiFi dongle:** MT7601U works. AR9271 (ath9k_htc) fails behind DWC2 with "Failed to read SREV register"
  (interrupt-endpoint register reads). Details, measurements, open issues: `docs/development-notes.md`.

## ESP32 variant (upstream)

```bash
esphome compile examples/basic.yaml        # needs examples/secrets.yaml (from secrets.yaml.example)
cd daemon && GOOS=linux GOARCH=arm GOARM=7 go build -o ulanzi-control ./cmd/ulanzi-control/   # stock firmware
```
Deploy to a stock D200 over adb: `docs/d200-preparation.md`.

## Architecture reference

### Concept model

```
ulanzi_d200 (1 per device)
  └── sections[] (button positions 0–13, in the viewer's reading order; see rotation)
        ├── position, page (-1 = all pages, 0+ = specific page)
        ├── on_press / on_release (ESPHome automations)
        └── layouts[] (conditional display states, first match wins)
              ├── when (lambda condition, evaluated every 1s)
              ├── widget type (entity_view | gauge | line_graph | status | chips | notification | forecast)
              └── properties (text, color, icon_id, etc. — static or lambda)
```

### ESPHome component (C++/Python)

- `__init__.py`: YAML schema validation and C++ code generation; all widget types and their properties; lambda vs static string properties.
- `ulanzi_d200.h`: `UlanziBridge` (`Component`, `UARTDevice`), `UlanziBacklightLight`, trigger classes, section/layout/widget structures.
- `ulanzi_d200.cpp`:
  - layout evaluation every 1 s, first matching layout sent;
  - properties sent only when changed;
  - batch queue: max 4 commands per batch, 3 s response timeout;
  - NDJSON parsing robust against printk noise;
  - page navigation (`navigate_page_next/prev/jump`);
  - backlight light entity (`wake_on_press`: in the dark a press only wakes the screen), notification services;
  - ping every 5 s against a 10 s timeout.

### Go daemon

- `cmd/ulanzi-control/main.go`:
  - framebuffer: geometry from the fb ioctls, 720x1280 at 32 bpp, rotated to landscape;
  - keys from `/dev/input/event0`; `struct input_event` size follows the architecture;
  - key map holds both mainline (KEY_F13–F24, KEY_PROG1/2) and stock codes; `config/keymap.json` overrides;
  - clock time zone from `TZ`, else `config/timezone`, using embedded Go tzdata;
  - status bar redrawn when the minute changes;
  - rotation (`rotation: 0|90|180|270`): `keyOrder` renumbers the keys from the rotated key-window centres
    (test: `rotation_test.go`), `drawTile` draws each tile upright in a canvas and turns it into its window;
    calibration and layout-edit modes stay unrotated.
- `animation.go`: full-screen animation (`play_animation`): `config/ring.gif` cropped to fill the screen, shown once
  loaded (nothing without it); drawn by the display ticker through `fbMap`'s table (test: `animation_test.go`),
  a key press ends it.
- `pkg/state/store.go`: ButtonConfig/ButtonState, RingBuffer time series, RWMutex, persisted to `config/state.json`.
- `pkg/transport/commands.go`: NDJSON modules `display`, `sensor`, `page`, `system`; codes OK=0, ERROR=1, INVALID=2, TIMEOUT=3, INTERNAL=5.
- `pkg/transport/serial.go`: 1.5 Mbps; port from `ULANZI_SERIAL`, default `/dev/ttyFIQ0`.
- `pkg/ui/draw.go`, `font.go`: rendering with per-button dirty flags.

### NDJSON protocol

ESPHome → D200: `{"type":"command","module":"display","action":"set_button_config","params":{...}}`
D200 → ESPHome: `{"type":"response","status":"ok","code":0,...}` and events such as `{"type":"event","event":"page_change",...}`.

Rotation: `{"module":"display","action":"set_rotation","params":["90"]}` (0, 90, 180 or 270, degrees the D200 is
mounted turned clockwise; anything else → INVALID). The component sends it first in every full config upload
(`send_all_section_configs_`: daemon ready, connect fallback, back to run mode). The daemon keeps it in memory only,
redraws everything, and from then on maps key presses and button IDs to the viewer's reading order.

Animation: `{"module":"display","action":"play_animation","params":["15"]}` (seconds, 1-120) plays
`/data/ulanzi-control/config/ring.gif` over the whole screen; call it with `id(d200).send_command(...)`.

### Key patterns

- Layouts are evaluated top to bottom; the first matching `when` wins; a layout without `when` is the fallback.
- String properties take a static string or a `!lambda` returning `optional<std::string>`.
- HA state sync: an optimistic template switch calls the HA action, and a `platform: homeassistant` binary_sensor syncs the real state back (`examples/host/d200-toggle.yaml`).
- Page -1: sections shown on every page (navigation, status bar).
