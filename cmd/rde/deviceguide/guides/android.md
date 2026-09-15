<!-- Mirror of the RDE device-session guide shipped with the RDE backend; keep in sync with the backend release you target — do not edit here. -->
# Android emulator sessions — driving the device

Read the main device sessions guide first (create, wait for READY, connect,
do-nots): `bitrise_devenv_device_guide` with `guide: "device-sessions"` (or the
resource `bitrise-devenv://guides/device-sessions`) or `bitrise-cli rde
device-guide` with no platform argument. This page is the Android
specifics. Everything here runs on the session VM (in-band via `execute`, or
over SSH) or through the adb tunnel from your machine — the emulator is
headless (`-no-window`); there is nothing to see on the VM's desktop.

## What is on the VM

- One AVD named **`dev`**, booted headless with KVM. It is the only adb
  device, so bare `adb` addresses it; if you boot more, use `adb -s`. It runs
  the generic emulator system image sized to the requested device profile —
  `ro.product.model` is `sdk_gphone…`, not the phone the profile is named
  after.
- **ws-scrcpy** on `127.0.0.1:8000` — the viewer's video source. Leave it.
- The Android SDK (`$ANDROID_HOME`, platform-tools, emulator, build-tools).
- Service ports on the session: `adb` (VM 5555 → local 15555) and
  `device-web-view` (VM 8000 → local 3200, the same name and local port an
  iOS session uses). From your machine:
  `ssh -N -L 15555:127.0.0.1:5555 -L 3200:127.0.0.1:8000 … && adb connect 127.0.0.1:15555`.

## Accessibility tree (what is on screen, where)

```bash
adb exec-out uiautomator dump /dev/tty
```

XML of the current window: every `<node>` carries `text`, `content-desc`,
`resource-id`, `class`, `clickable`, `enabled` and `bounds="[x1,y1][x2,y2]"`
in **pixels**. Tap the center of a node's bounds. Every node has a `text`
attribute (usually empty), so filter for non-empty ones — `grep -oE
'<node[^>]*text="[^"]+"[^>]*>'` — or use a short python for big screens.
The output ends `…</hierarchy>UI hierchary dumped to: /dev/tty` — uiautomator's
own status line, glued to the XML with no newline — so strip it before
handing the text to an XML parser: `| sed 's#UI hierchary dumped to:.*##'`.
(`uiautomator dump` fails while an animation runs — retry once after 500 ms.
Through `execute`, some system images (android-37) add a harmless stderr line,
`tcsetattr: Inappropriate ioctl for device`, to the two the shell always
prints; android-36 does not. Ignore it either way.)

## Input

```bash
adb shell input tap 540 1200
adb shell input swipe 540 1600 540 600 300     # x1 y1 x2 y2 [ms]
adb shell input text 'hello%sworld'            # %s = space; focus a field first
adb shell input keyevent KEYCODE_HOME          # KEYCODE_BACK, KEYCODE_APP_SWITCH, KEYCODE_ENTER …
```

Other tooling is welcome on this emulator — Espresso / instrumentation
tests, Appium's UiAutomator2 driver, Maestro, your own `adb` scripts — as
long as it drives **this** device (it is the only adb device, so the default
serial is right) and never reboots, wipes or recreates it (see Do not).

Screen size: `adb shell wm size` (e.g. `Physical size: 1080x2400`) — the
*physical* portrait size; it does not change under rotation, so in landscape
swap width and height when you compute tap coordinates (or read `bounds` from
a fresh `uiautomator dump`, which are already in the current orientation).

Verify a tap landed before building on it: `adb shell dumpsys window | grep
mCurrentFocus` shows the focused window/activity, and a second `uiautomator
dump` shows what changed.

## Install, launch, screenshot, logs

```bash
adb install -r /path/app.apk                          # x86_64 or arm64 APKs (arm64 via libndk_translation)
adb shell monkey -p com.example.app -c android.intent.category.LAUNCHER 1
adb shell am start -n com.example.app/.MainActivity   # or an explicit activity
adb shell am force-stop com.example.app
adb exec-out screencap -p > /tmp/shot.png
adb logcat -d -s MyApp:V                              # -d dumps and exits (MCP execute caps a call at 2 min; CLI exec at 10 min by default)
adb shell settings put system accelerometer_rotation 0; adb shell settings put system user_rotation 1   # rotate
adb shell cmd uimode night yes                        # dark mode
```

`adb exec-out screencap -p` yields a ~0.4–1.5 MB PNG at native resolution
(content-dependent; a `screencap` written *on the device* is larger). **Let
the screen settle before you shoot**: within ~1–2 s of a rotation or a
launch the frame can be blank below the status bar while `uiautomator dump`
already shows the new layout — wait ~2 s and re-shoot before reporting a
broken screen; the tree is the ground truth. Do not base64 it into
`execute` output — point the human at the session page's device view, or
bring the file out with `download` / `scp` as in the main guide §3. `adb shell screencap -p /sdcard/s.png && adb pull /sdcard/s.png`
is the form to use when the file must live on the device first (the redirect
form above writes to the VM's filesystem).

## Do not

- kill the emulator (`adb emu kill`, `pkill qemu`), wipe or delete the `dev`
  AVD, or `adb reboot` — the viewer and `device.state` follow this device.
- stop `ws-scrcpy` or anything on port 8000; touch `scrcpy-server` on the
  device.
- run `~/bin/emulator-up.sh` while `device.state` is `BOOTING`: after boot it
  drops the scrcpy-server unconditionally, so a copy started mid-boot kills
  the server the in-progress boot is waiting on and turns a healthy boot into
  a permanent stream failure.

Recovery — only when `device.state` is `FAILED` **and** `pgrep -f
emulator-up.sh` prints nothing: `~/bin/emulator-up.sh` (boots what is not
running, restarts ws-scrcpy, re-reports readiness; exits at once if a run is
already in progress). Then re-check `device.state`. `~/emulator-up.log`
narrates every step on a healthy boot and can go quiet for a minute or two
while the frame probe retries on a slow one — silence is not proof of a hang.
