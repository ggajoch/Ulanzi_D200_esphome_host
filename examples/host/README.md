# ESPHome on the D200 itself (host platform, aarch64)

The ESPHome config and build scripts for running ESPHome on the D200's RK3308.
The device needs the firmware from [`../../buildroot/`](../../buildroot/README.md), which runs `S70esphome`:

```
Home Assistant ──API 6053──▶ /data/esphome/d200 ──/run/d200-esp ⇄ socat ⇄ /run/d200-daemon──▶ /data/ulanzi-control/ulanzi-control
                             (ESPHome host + ulanzi_d200)                                       (draws /dev/fb0, reads /dev/input/event0)
```

| File | What |
|---|---|
| `d200.yaml` | ESPHome config. HA entity IDs and key labels are the `substitutions:` at the top |
| `d200-toggle.yaml` | package for one key that toggles an HA entity (template switch + `homeassistant` binary_sensor) |
| `secrets.yaml.example` | template for `secrets.yaml` (`api_encryption_key`); `build.sh` generates one if missing |
| `build.sh` | builds `../../components` + `d200.yaml` and `../../daemon` into `~/d200/esphome/out/{d200,ulanzi-control}` |
| `deploy.sh` | `build.sh`, then ESPHome OTA to `$DEVICE` |
| `screenshots/` | `/dev/fb0` dumps, turned to landscape |

![Page 0](screenshots/d200-page0-zewnatrz.png)

## Keys

The config is the author's own (Polish labels). Replace the `substitutions:` with your entities.

```
[ 0] [ 1] [ 2] [ 3] [ 4 Dalej ]      Zewnątrz: 0-2 outdoor lights, 3 all three, 5 Kuchnia
[ 5] [ 6] [ 7] [ 8] [ 9 Wstecz]      Warsztat: 0 workshop power, 1 compressor (hold 1 s)
[10 Start] [11] [12] [13 clock + page]
```

Home Assistant must let the device call actions: ESPHome integration → the device → Configure →
"Allow the device to perform Home Assistant actions". States are read without it.

## Build and deploy

Linux or WSL, after a [buildroot](../../buildroot/README.md) build (its toolchain matches the device's glibc):

```sh
./build.sh                       # build both
DEVICE=<D200 IP> ./deploy.sh     # build both, OTA the ESPHome binary
```

`build.sh` installs ESPHome 2026.10.2 in a venv, Go and a static OpenSSL `libcrypto` (for OTA) under `~/d200/`.
Override with `WORK`, `BR_HOST`, `GOROOT_DIR`, `ESPHOME_VERSION`. ESPHome is built with the Buildroot
aarch64 toolchain, `-O2`, libstdc++/libgcc/libcrypto linked statically, stripped; the daemon is a static
Go binary (`CGO_ENABLED=0 GOARCH=arm64`).

The first OTA needs a running ESPHome on the device. Before that, copy both binaries by hand:

```sh
ssh root@<D200> mkdir -p /data/esphome /data/ulanzi-control
scp ~/d200/esphome/out/d200 root@<D200>:/data/esphome/d200
scp ~/d200/esphome/out/ulanzi-control root@<D200>:/data/ulanzi-control/ulanzi-control
ssh root@<D200> /etc/init.d/S70esphome restart
```

`deploy.sh` does not touch the daemon. When `daemon/` changes, copy `out/ulanzi-control` to
`/data/ulanzi-control/` and `kill $(pidof ulanzi-control)`; `S70esphome` restarts it.

## Run by hand on the device

```sh
/etc/init.d/S70esphome stop
socat pty,link=/run/d200-esp,raw,echo=0 pty,link=/run/d200-daemon,raw,echo=0 &
(cd /data/ulanzi-control && ULANZI_SERIAL=/run/d200-daemon ./ulanzi-control) &
ESPHOME_PREFDIR=/data/esphome/prefs /data/esphome/d200
```

Start socat first: ESPHome opens the uart once at startup and the daemon opens its port once.
The daemon keeps `config/` (layout, key map, state, `timezone`) in its working directory;
`/data/ulanzi-control/config/timezone` holds the clock's time zone, e.g. `Europe/Warsaw` (or set `TZ`).
