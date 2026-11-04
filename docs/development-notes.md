# Development notes

Findings from getting ESPHome to run on the D200 itself. Firmware details live in
[`../buildroot/README.md`](../buildroot/README.md); the host build in [`../examples/host/`](../examples/host/README.md).

## Hardware

- SoC: Rockchip RK3308 (quad Cortex-A35). The stock firmware has a 32-bit ARMv7 userspace; d200-buildroot has a
  64-bit aarch64 one, so everything here (ESPHome, the daemon) is built for aarch64.
- Display: `/dev/fb0`, 720x1280, 32 bpp. Keys: a matrix keypad on
  `/dev/input/event0` that reports KEY_F13–F24, KEY_PROG1 and KEY_PROG2 on the mainline kernel (the stock kernel
  used other codes). Backlight: `/sys/class/backlight/backlight`.
- Power: a USB-C hub with a PD input (tested: Hiearcool UCN3273) and a charger on that input powers the D200
  through the hub's upstream cable. Plug in charger → hub → D200.

## USB host mode

- The D200's only USB-C port is the RK3308's DWC2 OTG controller. Host mode works with `dr_mode = "host"` in the
  device tree. The mode is fixed at boot, not switched at runtime.
- Behind the hub, the hub itself, an AR9271 WiFi dongle and an AX88179 Ethernet adapter all enumerate at 480M.
- In host mode there is no USB link to a PC: debug over SSH (WiFi or Ethernet), or as a last resort on the UART2
  pads (TX/RX, 1.5 Mbaud). The first firmware tests made host mode one-shot (the next boot returned to device mode)
  so a broken WiFi setup could not lock the device out.

## WiFi dongles

| Dongle | Driver | Result |
|---|---|---|
| Atheros AR9271 (`0cf3:9271`) | `ath9k_htc` | Firmware loads and HTC starts, then `Failed to read SREV register`: the register read over the interrupt endpoint times out behind DWC2. Not usable. |
| MediaTek MT7601U (`148f:7601`) | `mt7601u` | Works: −41 dBm, 72 Mb/s link. A short ping test: no loss, 2–24 ms. |

The AX88179 Ethernet adapter in the hub works as well.

## ESPHome host build

- ESPHome 2026.10.2, `host` platform. It compiles with whatever `CC`/`CXX`/`AR` say (ninja, no PlatformIO), so
  `examples/host/build.sh` points them at the d200-buildroot (Bootlin, GCC 15.3) toolchain: the binary then
  matches the device's glibc.
- The host build has no default optimisation (it would be `-O0`): `-O2` comes from `esphome.build_flags`.
- The image has no `libstdc++.so` and no OpenSSL: libstdc++ and libgcc are linked statically, and so is a
  `libcrypto.a` built from the same OpenSSL version as Buildroot's (OTA uses its MD5/SHA-256).
  The result is dynamically linked only to glibc (`libc`, `libm`, `ld-linux-aarch64`).
- Host OTA (TCP 8082) writes `<exe>.ota.new`, checks that its ELF machine matches the running binary, renames it
  over the executable and re-execs. ESPHome's preferences go to `ESPHOME_PREFDIR` (`/data/esphome/prefs`).
- Upstream's component needed one change for ESPHome 2026.10 (the backlight light entity now uses ESPHome's light
  schema), and the daemon needed several for the mainline kernel (`ULANZI_SERIAL`, the 24-byte aarch64
  `input_event`, the key codes, the fb geometry from ioctls). They are separate commits on top of upstream.

## Measurements

| What | Value |
|---|---|
| `d200` (ESPHome, stripped) | 6.9 MB |
| `ulanzi-control` (static Go, stripped) | 5.8 MB |
| Daemon RSS | 17.5 MB |
| Daemon CPU | about 34% of a core before the status-bar fix (it redrew and rotated the wide key at 20 fps for a clock that shows minutes), about 2% after |
| Clean ESPHome compile (WSL) | about 45 s |

The ESPHome process's own RSS and CPU have not been measured yet.

## Recovery

- Back up the stock firmware first: the whole `rkflash0` over `adb`.
- Maskrom/loader mode: `adb shell reboot loader` on the stock firmware; otherwise short pad R9602 to GND while
  plugging in USB. Then flash with RKDevTool, or restore Ulanzi's image from
  [CHADREX/ulanzi-d200-recovery](https://github.com/CHADREX/ulanzi-d200-recovery).
- With WSL and `usbipd`, `usbipd unbind` the Rockchip device (`2207:330e`) before using RKDevTool on Windows.

## Open issues

1. ESPHome OTA carries only the ESPHome binary. A daemon change is copied to `/data/ulanzi-control/` by hand.
2. The clock's time zone lives in the daemon (`/data/ulanzi-control/config/timezone` or `TZ`; the image has no
   zoneinfo, so the daemon embeds Go's). It belongs in the firmware (`TZ` or `BR2_TARGET_LOCALTIME`).
3. WiFi stability has only been tested over short runs. Weak signal (-69 dBm) made a 20 MB firmware upload take
   over 2 minutes; use a long `curl -m` for the update page.
4. Rotation isn't stored by the daemon: after a daemon restart it draws at 0° until ESPHome re-sends the rotation
   (a short flash). Gauge and line graph use only the middle square of the tall key at 90°/270°; chips,
   notification and side-by-side in a tall key were checked only in offscreen renders.
5. Possible next step: native ESPHome components for `/dev/fb0` (display, LVGL), `/dev/input/event0` (keys) and
   the sysfs backlight, which would replace the daemon and socat with a single process.
