<!-- Mirror of the RDE device-session guide shipped with the RDE backend; keep in sync with the backend release you target — do not edit here. -->
# Device sessions — the agent guide

An RDE **device session** is an ordinary RDE session that boots a virtual
device: an **Android emulator** (Linux stack) or an **iOS simulator** (macOS
stack). You get a full development VM *plus* a booted, streamable device, and
a human can watch what you do on it in the browser. This guide is the
know-how: how to create one, how to know it is ready, how to drive the device
efficiently, and what never to do.

The same text is served by the `bitrise-dev-environments` MCP server as the
resources `bitrise-devenv://guides/device-sessions`, `.../ios` and
`.../android`, and the `bitrise rde` CLI prints it with
`bitrise rde device-guide [ios|android]`.

## 1. Create

Create a session with a `device_spec` (and optionally an `artifact`). Nothing
else is required — stack, machine type and cluster default to the deployment's
known-good pair for the platform.

MCP:

```json
{ "name": "ios-check", "device_spec": { "platform": "ios", "device_model": "iPhone 16", "os_version": "18.2" } }
```

```json
{ "name": "android-check",
  "device_spec": { "platform": "android", "device_model": "pixel_7" },
  "artifact": { "url": "https://…signed…/app.apk", "app_name": "Demo" } }
```

CLI: `bitrise rde session create ios-check --device-platform ios --device-model "iPhone 16" --device-os-version 18.2`

REST: `POST /v1/workspaces/{ws}/sessions` with the same fields (`device_spec`,
`artifact`).

Rules:

- `platform` is `ios` or `android`. Everything else in `device_spec` is
  optional: `device_model` (simctl device type / emulator device profile),
  `os_version` (iOS only, "18.2" or a simctl runtime id; empty = newest
  installed), `system_image` / `ram_mb` / `cores` / `cold_boot` (Android only).
- If you name a `stack_id`/`machine_type`, they must fit the platform (macOS
  for iOS, an Android-flavored Linux stack for Android; 4 vCPU / 6–8 GB
  minimum) or the request is rejected with the reason. Prefer omitting them.
- `artifact.url` must be an absolute http(s) URL the VM can download (a signed
  URL is fine; it is never returned by the API). iOS: a zipped simulator
  `.app`; Android: an `.apk`. Omit it when you build the app yourself.
- A template is optional and works as usual (scripts, inputs, links). A
  `device_spec` wins over the template's own Android emulator declaration.
- `auto_terminate_minutes` defaults to **240** (4 hours) for device sessions,
  not 5 days: devices run on scarce hardware and nothing extends them while
  you work. Set it explicitly for longer jobs, and **delete the session when
  you are done**.

## 2. Wait for the device — "running" is not "ready"

The session turns `running` when its startup script begins; the device boots
in the background for another 30 s (iOS warm) to ~2 min (Android/iOS cold).
Poll the session (`bitrise_devenv_get` / `rde session view` / `GET
/sessions/{id}`) and read `device`:

| `device.state` | Meaning | You |
|---|---|---|
| `PREVIEW_DEVICE_STATE_UNSPECIFIED` | VM not running yet (or a VM predating the signal) | wait |
| `PREVIEW_DEVICE_STATE_BOOTING` | VM running, device not yet proven | wait |
| `PREVIEW_DEVICE_STATE_READY` | device booted **and** its stream delivers frames | go |
| `PREVIEW_DEVICE_STATE_FAILED` | the VM gave up on this boot; `device.device_notes` says why | see §6 |

If you supplied an `artifact`, also watch `device.install_status`: `PENDING`
(waiting for the device) → `RUNNING` → `OK` or `FAILED` (`install_reason`
says why). A failed install leaves the device usable — install the app
yourself (§4). Give up waiting for an install after ~3 minutes past READY.

Poll every ~5 s; budget 5 minutes for READY before treating the boot as
stuck. `device.device_notes` may also carry *degradations* (e.g. "requested
system image X not installed; using Y") on a READY device — read them, they
tell you what you actually got.

## 3. Connect

Two ways, in order of preference:

1. **SSH tunnel to the device ports** (richest; needs `ssh` locally). The
   session's `ssh_address`/`ssh_password` and `template_snapshot.service_ports`
   are in every session read. Forward the platform's ports and use your local
   tooling:
   - Android: `-L 15555:127.0.0.1:5555` then `adb connect 127.0.0.1:15555` —
     the full adb surface (~50 ms per command).
   - iOS: `-L 3200:127.0.0.1:3200` gives you serve-sim's HTTP/WS API (stream,
     `/ax`, gestures). For `idb` install it locally (`pipx install fb-idb`)
     and run `idb_companion` on the VM yourself if you want it.
2. **In-band through `execute`** (`bitrise_devenv_execute` / `rde session
   exec`): runs a login shell on the VM, 2-minute limit per call, returns
   text. Everything below works this way. Pixels do not travel well as text
   (a downscaled PNG is ~120 K base64 chars) — prefer the accessibility tree,
   and use the viewer URL or the tunnel when you must look.

## 4. Drive the device

Platform specifics live in [ios.md](ios.md) and [android.md](android.md). The
shape is the same on both:

- **Accessibility tree, not pixels.** iOS: `curl -s
  http://127.0.0.1:3200/helper/<UDID>/ax` (labels, types, frames, ids as
  JSON — needs an app in the foreground). Android: `adb exec-out uiautomator
  dump /dev/tty` (XML with bounds/text/resource-id). One call tells you what
  is on screen and where to tap.
- **Input.** iOS: serve-sim's CLI (`tap`, `type`, `button`, `gesture`,
  normalized 0..1 coordinates). Android: `adb shell input tap/text/keyevent`.
- **Install / launch.** iOS: `xcrun simctl install <UDID> App.app` +
  `xcrun simctl launch <UDID> <bundle-id>`. Android: `adb install -r app.apk`
  + `adb shell monkey -p <package> 1` (or `am start`).
- **Screenshot.** iOS: `xcrun simctl io <UDID> screenshot /tmp/s.png`.
  Android: `adb exec-out screencap -p > /tmp/s.png`. Bring the file out over
  the tunnel/`download`, or hand a human the viewer URL instead.
- **Logs.** iOS: `xcrun simctl spawn <UDID> log stream --predicate '…'`.
  Android: `adb logcat`.

## 5. Let a human watch

`device.viewer_url` (on every session read) opens the **device viewer page**
— the same page a PR preview link opens — in watch mode for this session: it
attaches, never creates, offers no delete. Anyone holding the URL can view
*and touch* the device until it expires (24 h max, re-minted on every read),
so share it deliberately. Human taps and your taps go to the same device; do
not fight over it.

## 6. Do not break the stream — and what to do if you did

The device state and the human's viewer depend on the platform's device and
its streamer process. **Never**:

- shut down, erase, delete or re-create the simulator / kill the emulator or
  wipe the AVD (`bitrise-preview` on iOS, `dev` on Android) — use the booted
  device you were given (`xcrun simctl … booted`, the single adb device);
- stop `serve-sim` (iOS, port 3200) or `ws-scrcpy` (Android, port 8000);
- open `Simulator.app` or use the macOS desktop screenshot tool to look at the
  simulator — it is headless; the desktop shows nothing.

If the device did break (`FAILED`, or the stream died): re-run the idempotent
orchestrator on the VM — `~/bin/simulator-up.sh` (iOS) or
`~/bin/emulator-up.sh` (Android). It reboots the device if needed, restarts
the streamer, and re-reports readiness, so `device.state` returns to READY
and the viewer recovers by itself. Logs: `~/simulator-up.log`,
`~/serve-sim.log` / `~/emulator-up.log`, `~/emulator.log`, `~/ws-scrcpy.log`.

## 7. Clean up

Delete the session when you are done (`bitrise_devenv_delete` / `rde session
delete`). A terminated-but-not-deleted device session can be restored; the
device boots again on restore and readiness is re-reported.
