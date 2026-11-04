# D200 firmware for ESPHome

Patches for [d200-buildroot](https://github.com/riptidewave93/d200-buildroot), the open firmware for the Ulanzi D200,
that let the D200 run an ESPHome `host` build on WiFi. They apply to the commit in [`UPSTREAM`](UPSTREAM).

## What the patches add

| Patch | What |
|---|---|
| `0001` USB host mode | The USB-C port as a USB host (for a powered hub), chosen at boot by the U-Boot env `usb_mode`; host is the default |
| `0002` WiFi | USB WiFi dongles: MT7601U (works), AR9271 (`ath9k_htc`, doesn't initialise yet); firmware built into the kernel; `wpa_supplicant`, `udhcpc` |
| `0003` Ethernet | USB Ethernet adapters (AX88179, CDC Ethernet/NCM, RTL8152/8153) with DHCP |
| `0004` Display | `/dev/fb0` (720x1280, 32 bpp) through DRM fbdev emulation, no console on it |
| `0005` ESPHome | `S70esphome`: runs `/data/esphome/d200` instead of the stock key app, optionally with a display daemon over a `socat` pty pair; mDNS `_esphomelib._tcp` |
| `0006` Debugging | debugfs, usbmon, dynamic debug, `lsusb`, `usbreset`, boot logs in `/data/wifi-test/` |
| `0007` NTP | BusyBox `ntpd`; servers from `/data/ntp.conf`, default `pool.ntp.org` |
| `0008` Docs | `docs/esphome.md` in the patched tree: setup, USB modes, logs, debugging |

The device side is described in `docs/esphome.md` of the patched tree; the rest of the firmware in its `docs/`.

**SSH:** root has no password, over USB and over the network (WiFi, Ethernet), as in d200-buildroot's debug
builds. Anyone on the same network can log in as root: put the D200 on a trusted LAN only.

## Build

On Linux, or in WSL2, with Docker, git and make:
```bash
./build.sh                      # work dir ~/d200-buildroot-esphome; or ./build.sh <dir>
```
It clones d200-buildroot, checks out the pinned commit, applies `patches/` with `git am` and runs
`make build FLAVOR=debug`. The first build takes about 40 minutes. It prints the image folder,
`output/<date>-debug/` in the work dir, with `d200-full.img`, `d200-install.py` and
`d200-<version>-debug-update.tar`. In WSL2, keep the work dir on the Linux file system, not under `/mnt/c`.

## First install, from the stock firmware

This erases the D200 and installs the build (d200-buildroot's `docs/install.md`, debug flavour).

1. **Maskrom.** With the D200 on the stock firmware and plugged into the PC, check that `adb devices` lists it, then:
   ```bash
   adb shell "sync; devmem 0xff000500 32 0xEF08A53C; devmem 0xff5000b8 32 0xfdb9"
   ```
   The D200 restarts in maskrom, USB device `2207:330e`. Nothing is written yet.
2. **Windows only: hand the USB device to WSL2** with [usbipd-win](https://github.com/dorssel/usbipd-win). Keep
   a WSL2 terminal open: `usbipd attach` fails when the WSL2 distribution isn't running.
   ```powershell
   usbipd list                                          # the BUSID of 2207:330e
   usbipd bind --busid <BUSID>                          # administrator PowerShell
   usbipd attach --wsl --busid <BUSID> --auto-attach    # keep it running
   ```
3. **Install**, in Linux or WSL2, from the image folder:
   ```bash
   cd ~/d200-buildroot-esphome/output/<date>-debug
   python3 -m venv d200-venv
   d200-venv/bin/pip install pyusb libusb-package
   sudo d200-venv/bin/python d200-install.py
   ```
   On Windows the first run usually stops with "Rockchip's flash loader didn't start". Once the loader runs, the
   D200 reconnects as a new USB device (`2207:330e` again, named "USB-MSC" in `usbipd list`), and usbipd doesn't
   share it until you bind it again. Run `usbipd bind --busid <BUSID>` once more (administrator), let
   `--auto-attach` pick it up, then run `d200-install.py` again: it notices that the flash loader is already running,
   erases and writes. The D200 then restarts into the new firmware.
4. **Set up over USB.** The first boot is a USB device: the PC gets a USB network interface, and the D200 is
   `d200-<serial>.local`. Over SSH (root, no password), add the WiFi network to `/data/wpa_supplicant.conf` and the
   ESPHome build to `/data/esphome/d200` (`docs/esphome.md` has the commands). Then `fw_setenv usb_mode host`,
   unplug the D200 from the PC and plug it into a powered USB-C hub, with the WiFi dongle on the hub. From then on
   it's on WiFi, with the same SSH login.

## Updates

New builds of these patches install from the D200's update page, over WiFi. The update page is part of the stock
key app, which `S70esphome` stops, so start it first:
```bash
ssh root@d200-<serial>.local '/etc/init.d/S70esphome stop; /etc/init.d/S60deckd start'
```
Then open `http://d200-<serial>.local/update`, upload `d200-<version>-debug-update.tar`, and press **Install** on the
D200's wide key (bottom right). It writes the other A/B slot and reboots once. ESPHome starts again after the D200
has confirmed the update.

## USB mode and recovery

| To | How |
|---|---|
| USB device (back on the PC) | Over SSH: `fw_setenv usb_mode device && reboot`, then plug into the PC |
| USB host (the default) | Over SSH: `fw_setenv usb_mode host && reboot`, then plug into the hub |
| USB device without SSH | Hold the top-left key while plugging into the PC (fastboot), then `fastboot oem "run:setenv usb_mode device; saveenv"` and `fastboot reboot` |
| A D200 that doesn't start | Hold the top-left and bottom-left keys while plugging into the PC (maskrom), then install again (steps 2-3 above) |
| Back to the stock firmware | d200-buildroot's `docs/recovery.md` |

## Licences

- The patches: GPL-2.0-or-later, like d200-buildroot. The Linux and U-Boot patches inside them are under those
  projects' licences.
- Firmware files in patch `0002` (`board/ulanzi/d200/firmware/`, each with its licence file):

  | File | From | Licence |
  |---|---|---|
  | `ath9k_htc/htc_9271-1.4.0.fw` | linux-firmware 20260916 | open-ath9k-htc-firmware: free software |
  | `mediatek/mt7601u.bin` | linux-firmware 20260916 | Ralink/MediaTek: redistributable, not free |
  | `regulatory.db` | wireless-regdb 2026.09.03 | ISC |

  They are built into the kernel image. The kernel's `CONFIG_EXTRA_FIRMWARE` help warns that distributing a kernel
  with non-GPL firmware built in may conflict with the GPL; keep that in mind before publishing images.
