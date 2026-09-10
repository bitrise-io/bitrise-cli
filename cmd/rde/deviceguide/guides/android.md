<!-- Mirror of the RDE device-session guide shipped with the RDE backend; keep in sync with the backend release you target — do not edit here. -->
# Android emulator sessions — driving the device

Read the main device sessions guide first (create, wait for READY, connect,
do-nots): `README.md` next to this file, the MCP resource
`bitrise-devenv://guides/device-sessions`, or `bitrise-cli rde device-guide`.
This page is the Android specifics. Everything here runs on the session VM
(in-band via `execute`, or over SSH) or through the adb tunnel from your
machine — the emulator is headless (`-no-window`).

## What is on the VM

- One AVD named **`dev`**, booted headless with KVM. It is the only adb
  device, so bare `adb` addresses it; if you boot more, use `adb -s`.
- **ws-scrcpy** on `127.0.0.1:8000` — the viewer's video source. Leave it.
- The Android SDK (`$ANDROID_HOME`, platform-tools, emulator, build-tools).
- Service ports on the session: `adb` (VM 5555 → local 15555) and
  `emulator-web-view` (8000). From your machine:
  `ssh -N -L 15555:127.0.0.1:5555 … && adb connect 127.0.0.1:15555`.

## Accessibility tree (what is on screen, where)

```bash
adb exec-out uiautomator dump /dev/tty
```

XML of the current window: every `<node>` carries `text`, `content-desc`,
`resource-id`, `class`, `clickable`, `enabled` and `bounds="[x1,y1][x2,y2]"`
in **pixels**. Tap the center of a node's bounds. Filter with
`grep -o '<node[^>]*text="[^"]*"[^>]*>'` or a short python for big screens.
(`uiautomator dump` fails while an animation runs — retry once after 500 ms.)

## Input

```bash
adb shell input tap 540 1200
adb shell input swipe 540 1600 540 600 300     # x1 y1 x2 y2 [ms]
adb shell input text 'hello%sworld'            # %s = space; focus a field first
adb shell input keyevent KEYCODE_HOME          # KEYCODE_BACK, KEYCODE_APP_SWITCH, KEYCODE_ENTER …
```

Screen size: `adb shell wm size` (e.g. `Physical size: 1080x2400`).

## Install, launch, screenshot, logs

```bash
adb install -r /path/app.apk                          # x86_64 or arm64 APKs (arm64 via libndk_translation)
adb shell monkey -p com.example.app -c android.intent.category.LAUNCHER 1
adb shell am start -n com.example.app/.MainActivity   # or an explicit activity
adb shell am force-stop com.example.app
adb exec-out screencap -p > /tmp/shot.png
adb logcat -d -s MyApp:V                              # -d dumps and exits (execute has a 2-minute limit)
adb shell settings put system accelerometer_rotation 0; adb shell settings put system user_rotation 1   # rotate
adb shell cmd uimode night yes                        # dark mode
```

Screenshots are ~1–2 MB PNG at native resolution. Do not base64 them into
`execute` output — point the human at the session page's device view, or pull the file
over the tunnel: `adb shell screencap -p /sdcard/s.png && adb pull
/sdcard/s.png` (the redirect form above writes to the VM's filesystem, so
use `shell screencap` when the file must live on the device).

## Do not

- kill the emulator (`adb emu kill`, `pkill qemu`), wipe or delete the `dev`
  AVD, or `adb reboot` — the viewer and `device.state` follow this device.
- stop `ws-scrcpy` or anything on port 8000; touch `scrcpy-server` on the
  device.

Recovery: `~/bin/emulator-up.sh` (idempotent: boots if needed, restarts
ws-scrcpy, re-reports readiness). Then re-check `device.state`.
