<!-- Mirror of the RDE device-session guide shipped with the RDE backend; keep in sync with the backend release you target — do not edit here. -->
# Device sessions — the agent guide

An RDE **device session** is an ordinary RDE session that boots a virtual
device: an **Android emulator** (Linux stack) or an **iOS simulator** (macOS
stack). You get a full development VM *plus* a booted, streamable device, and
a human can watch what you do on it in the browser. This guide is the
know-how: how to create one, how to know it is ready, how to drive the device
efficiently, and what never to do.

Where to find this guide and the platform specifics, whichever way you reach
RDE: in the `bitrise-dev-environments` MCP server as the resources
`bitrise-devenv://guides/device-sessions`, `bitrise-devenv://guides/device-sessions/ios`
and `bitrise-devenv://guides/device-sessions/android` (or, when your MCP client
cannot read resources, the `bitrise_devenv_device_guide` tool returns the same
text); in the CLI as
`bitrise-cli rde device-guide`, `bitrise-cli rde device-guide ios` and
`bitrise-cli rde device-guide android`; in the docs folder as `README.md`,
`ios.md` and `android.md`.

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

CLI: `bitrise-cli rde session create ios-check --device-platform ios --device-model "iPhone 16" --device-os-version 18.2`

REST: `POST /v1/workspaces/{ws}/sessions` with the same fields (`device_spec`,
`artifact`).

Rules:

- `platform` is `ios` or `android`. Everything else in `device_spec` is
  optional: `device_model` (simctl device type / emulator device profile),
  `os_version` (iOS only: an iOS version such as "18.2", or a simctl runtime
  id; anything else — and any value on Android — is rejected with a 400;
  empty = newest installed), `system_image` / `ram_mb` / `cores` /
  `cold_boot` (Android only).
- If you name a `stack_id`/`machine_type`, they must fit the platform (macOS
  for iOS; for Android a *dockerless* Android stack such as
  `ubuntu-resolute-26.04-bitrise-2026-android` — the Docker-based
  `linux-docker-*` stacks keep the Android SDK inside a container and are
  rejected; 4 vCPU / 6–8 GB minimum) or the request is rejected with the
  reason. Prefer omitting them: the deployment default is the known-good
  pair. `cluster` is never needed with a `device_spec`; the backend picks one.
- `artifact.url` must be an absolute http(s) URL the VM can download (a signed
  URL is fine; it is never returned by the API and is stored encrypted at
  rest in the session — it is decrypted only to hand it to the VM's
  installer). iOS: a zipped simulator `.app`; Android: an `.apk`. Omit it
  when you build the app yourself.
- A template is optional and works as usual (scripts, inputs, links); with a
  template, its stack and machine type are used and must fit the platform. A
  session has a device only when the request carries a `device_spec`.
- `auto_terminate_minutes` works exactly as for any session (default 5 days,
  0 disables). Devices run on scarce hardware, so **delete the session when
  you are done**.

## 2. Wait for the device — "running" is not "ready"

The session turns `running` when its startup script begins; the device boots
in the background for another 30 s (iOS warm) to ~2 min (Android/iOS cold).
Poll the session (`bitrise_devenv_get` / `bitrise-cli rde session view` /
`GET /sessions/{id}`) and read `device.state` **together with the session
`status`**. The MCP and REST return the wire enum names; the CLI normalizes
them to the short words in the CLI column (`--output json` omits the
unspecified state; the human output prints `not running` for it).

| `device.state` (MCP / REST) | CLI | Meaning | You |
|---|---|---|---|
| `PREVIEW_DEVICE_STATE_UNSPECIFIED` (or absent) + `status` pending/starting | `not running` / (absent) | VM not running yet | wait |
| `PREVIEW_DEVICE_STATE_UNSPECIFIED` + `status` terminated/terminating/draining/drained/failed | `not running` / (absent) | the device is gone with the VM | stop polling; restore the session or create a new one |
| `PREVIEW_DEVICE_STATE_BOOTING` | `booting` | VM running, device not yet proven | wait |
| `PREVIEW_DEVICE_STATE_READY` | `ready` | device booted **and** its stream delivers frames | go |
| `PREVIEW_DEVICE_STATE_FAILED` | `failed` | the VM gave up on this boot; `device.device_notes` says why | see §6 |

If you supplied an `artifact`, also watch `device.install_status` — wire
values `PREVIEW_INSTALL_STATUS_UNSPECIFIED` (not fired yet) →
`PREVIEW_INSTALL_STATUS_PENDING` (waiting for the device) →
`PREVIEW_INSTALL_STATUS_RUNNING` → `PREVIEW_INSTALL_STATUS_OK` or
`PREVIEW_INSTALL_STATUS_FAILED` (`device.install_reason` says why); the CLI
shows `pending` / `running` / `ok` / `failed`. A failed install is final for
this boot and leaves the device usable — install the app yourself (§4), or
restore the session to re-run it. Give up waiting for an install after ~3
minutes past READY; keep polling the session while you wait, the poll itself
is what re-fires an install whose runner was lost.

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
   The ports are named the same on both platforms: `device-web-view` is the
   browser view of the device, forwarded to **local 3200** whichever platform
   (the VM side differs: serve-sim on 3200 for iOS, ws-scrcpy on 8000 for
   Android); Android adds `adb`.
   - Android: `-L 15555:127.0.0.1:5555` then `adb connect 127.0.0.1:15555` —
     the full adb surface (~50 ms per command); `-L 3200:127.0.0.1:8000` for
     the web view.
   - iOS: `-L 3200:127.0.0.1:3200` gives you serve-sim's HTTP/WS API (stream,
     `/ax`, gestures) and the web view. For `idb` install it locally
     (`pipx install fb-idb`) and run `idb_companion` on the VM yourself if
     you want it.
2. **In-band through `execute`** (`bitrise_devenv_execute` / `bitrise-cli rde
   session exec`): runs a login shell on the VM, 2-minute limit per call, returns
   text. Everything below works this way. Pixels do not travel well as text
   (a downscaled PNG is ~120 K base64 chars) — prefer the accessibility tree,
   and use the device view (§5) or the tunnel when you must look.

## 4. Drive the device

Platform specifics live in the iOS and Android guides (`ios.md` /
`android.md`; MCP resources `bitrise-devenv://guides/device-sessions/ios` and
`.../android`, or the `bitrise_devenv_device_guide` tool with `ios`/`android`;
CLI `bitrise-cli rde device-guide ios|android`). The shape is
the same on both:

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
  the tunnel/`download`, or point a human at the device view (§5) instead.
- **Logs.** iOS: `xcrun simctl spawn <UDID> log stream --predicate '…'`.
  Android: `adb logcat`.

## 5. Let a human watch

A human watches and drives the device from the session's page in the RDE web
UI: Sessions → the session → the **Device** row → **Open device view**. It is
the same viewer page a PR preview link opens, attach-only for this session
(it attaches, never creates, and cannot delete or terminate the session), but
it requires being logged in with access to the session — its owner for a
personal session, any workspace member for a workspace-owned one. There is no
shareable link. Human taps and your taps go to the same device; do not fight
over it.

## 6. Do not break the stream — and what to do if you did

The device state and the human's viewer depend on the platform's device and
its streamer process. **Never**:

- shut down, erase, delete or re-create the simulator / kill the emulator or
  wipe the AVD (`bitrise-preview` on iOS, `dev` on Android) — use the booted
  device you were given (`xcrun simctl … booted`, the single adb device);
- stop `serve-sim` (iOS, port 3200) or `ws-scrcpy` (Android, port 8000);
- open `Simulator.app` or use the macOS desktop screenshot tool to look at the
  simulator — it is headless; the desktop shows nothing.

If `device.state` is `FAILED` and the notes say *setup failed during
provisioning*, the device never existed on this VM: the platform setup itself
failed (wrong stack, no KVM). There is nothing to recover — delete the session
and create a new one, preferably without `stack_id` so the default applies.

If the device did break later (`FAILED` after it was READY, or the stream
died): re-run the idempotent orchestrator on the VM — `~/bin/simulator-up.sh` (iOS) or
`~/bin/emulator-up.sh` (Android). It reboots the device if needed, restarts
the streamer, and re-reports readiness, so `device.state` returns to READY
and the viewer recovers by itself. Logs: `~/simulator-up.log`,
`~/serve-sim.log` / `~/emulator-up.log`, `~/emulator.log`, `~/ws-scrcpy.log`.

## 7. Clean up

Delete the session when you are done (`bitrise_devenv_delete` /
`bitrise-cli rde session delete`). A terminated-but-not-deleted device session
can be restored: the device boots again, readiness is re-reported and the
`artifact` (if any) is installed again. `device.state` and
`device.install_status` describe the current boot only — a terminate clears
both, so a stopped session never reports a ready device or an installed app.
