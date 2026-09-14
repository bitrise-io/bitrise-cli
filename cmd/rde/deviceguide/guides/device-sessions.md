<!-- Mirror of the RDE device-session guide shipped with the RDE backend; keep in sync with the backend release you target — do not edit here. -->
# Device sessions — the agent guide

An RDE **device session** is an ordinary RDE session that boots a virtual
device: an **Android emulator** (Linux stack) or an **iOS simulator** (macOS
stack). You get a full development VM *plus* a booted device you drive over
`adb` / `xcrun simctl` / serve-sim, and a human can watch it in the browser.
This guide is the know-how: create, know when it is ready, connect, drive it
efficiently, and what never to do. The platform recipes are in the iOS and
Android guides — MCP: `bitrise_devenv_device_guide` with `guide: "ios"` /
`"android"` (or the resources `bitrise-devenv://guides/device-sessions/ios` /
`.../android` if your client reads resources); CLI: `bitrise-cli rde
device-guide ios|android`.

## 1. Create

Create a session with a `device_spec` (and optionally an `artifact`). Nothing
else is required — stack, machine type and cluster default to the deployment's
known-good pair for the platform.

MCP (`bitrise_devenv_create`):

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

**From a template.** A template can declare the device every session created
from it boots (`Template.device_spec`, the same message; set it with
`bitrise_devenv_create_template` / `bitrise_devenv_update_template`,
`bitrise-cli rde template create|update --device-platform …`, or the template
form in the web UI). The template is the base and the request overrides it the
way `stack_id` / `machine_type` do:

- no `device_spec` on the request → the template's device boots as declared;
- a `device_spec` **without** a `platform` → a per-field tweak: fields you
  leave empty inherit the template's (`{"device_model": "iPhone 15"}` changes
  only the model; CLI `--template T --device-model "iPhone 15"`);
- a `device_spec` **with** a `platform` → the complete device to boot: the
  template's is ignored and empty fields are the platform defaults (this is
  what the web form sends, so what it shows is what boots);
- `no_device: true` (CLI `--no-device`) → the session boots no device.

Rules:

- `platform` is `ios` or `android`. Everything else in `device_spec` is
  optional: `device_model` (simctl device type / emulator device profile),
  `os_version` (iOS only: an iOS version such as "18.2", or a simctl runtime
  id; anything else — and any value on Android — is rejected with a 400;
  empty = newest installed), `system_image` / `ram_mb` / `cores` /
  `cold_boot` (Android only).
- **`device_model` is a screen profile, not a phone.** On Android
  `"pixel_7"` gives you a 1080×2400 / 420 dpi emulator running the generic
  system image (`ro.product.model` is `sdk_gphone…`, `dev-keys` firmware) —
  not Pixel firmware. Do not answer questions about a real device's stock
  behaviour from it. `system_image` is the Android **API-level** knob
  (`"system-images;android-34;google_apis;x86_64"`; empty = the stack's
  default). If the image you asked for is not installed the VM falls back and
  says so in `device.device_notes`; a *defaulted* image produces no note.
- The platform must be enabled for the workspace: Android rides the
  `enable-rde-android-emulator` flag, iOS `enable-rde-ios-simulator`. Without
  it every front door answers `FailedPrecondition` ("The … capability is not
  enabled for this workspace.") — the request is well-formed, the workspace
  just cannot boot that device. Ask an admin; retrying will not help.
- Prefer **omitting** `stack_id` / `machine_type` — the deployment default is
  the known-good pair. If you name them they must fit the platform or the
  request is rejected with the reason: iOS needs a macOS stack (device
  sessions are validated on macOS 26 stacks — `os_version` 26 in
  `bitrise_devenv_list_stacks`, e.g. `osx-xcode-26.4.x` and newer; on the
  Xcode 16.x / macOS 15 stacks the iOS streamer crashes and the device ends
  `FAILED`); Android needs a *dockerless* Android stack such as
  `ubuntu-resolute-26.04-bitrise-2026-android` (the Docker-based
  `linux-docker-*` and `ubuntu-noble-24.04-*` stacks keep the SDK inside a
  container and are rejected); 4 vCPU / 6–8 GB minimum. `cluster` is never
  needed with a `device_spec`; the backend picks one.
- If creation fails with `stack is not available: <id>` **without** you
  naming a stack, that deployment's device default is stale. Recover once,
  explicitly: `bitrise_devenv_list_stacks` → pick a stack as above →
  `bitrise_devenv_list_machine_types` → a machine type whose `cluster_name`
  is one of that stack's `cluster_names` → pass both. Do not retry the
  zero-config create; it will fail the same way.
- `artifact.url` must be an absolute http(s) URL the VM can download (a signed
  URL is fine; it is never returned by the API and is stored encrypted at
  rest in the session — it is decrypted only to hand it to the VM's
  installer). iOS: a zipped simulator `.app`; Android: an `.apk`. It needs a
  device to land on: an `artifact` on a session that boots none is rejected.
  Omit it when you build the app yourself.
- A template is optional and works as usual (scripts, inputs, links); with a
  template, its stack and machine type are used and must fit the platform.
- `auto_terminate_minutes` works exactly as for any session (default 5 days,
  0 disables). Devices run on scarce hardware, so **delete the session when
  you are done** — and delete only session IDs from your own `create`
  responses; a listing can show other people's sessions.

## 2. Wait for the device — "running" is not "ready"

The session turns `running` when its startup script begins; the device boots
in the background. Measured: an Android cold boot reaches READY ~2.5 minutes
after `running`; an iOS simulator ~1 minute after `running`. The VM
itself is the slow part on macOS — a macOS session can take 5–10 minutes to
reach `running` at all — so budget per platform: Android ~10 minutes and iOS
~15 minutes from `create` to READY, of which at most ~8 minutes may be spent
`BOOTING`. The VM reports `FAILED` on its own when a boot gives up (Android
stops waiting after 10 minutes); wait for that verdict.

**While `device.state` is `BOOTING`, do nothing.** Do not run the
orchestrator, do not restart anything on the VM, do not delete and recreate.
In testing the one thing that reliably turned a healthy 2.5-minute boot into
a permanent failure was an agent "helping" at minute four.

Poll the session (`bitrise_devenv_get` / `bitrise-cli rde session view` /
`GET /sessions/{id}`) every ~5 s and read `device.state` **together with the
session `status`**. The MCP and REST return the wire enum names; the CLI
normalizes them to the short words in the CLI column (`--output json` omits
the unspecified state; the human output prints `not running` for it).

| `device.state` (MCP / REST) | CLI | Meaning | You |
|---|---|---|---|
| `PREVIEW_DEVICE_STATE_UNSPECIFIED` (or absent) + `status` pending/starting | `not running` / (absent) | VM not running yet | wait |
| `PREVIEW_DEVICE_STATE_UNSPECIFIED` + `status` terminated/terminating/draining/drained/failed | `not running` / (absent) | the device is gone with the VM | stop polling; restore the session or create a new one |
| `PREVIEW_DEVICE_STATE_BOOTING` | `booting` | VM running, device not yet proven | wait — touch nothing |
| `PREVIEW_DEVICE_STATE_READY` | `ready` | device booted **and** its stream delivers frames | go |
| `PREVIEW_DEVICE_STATE_FAILED` | `failed` | this boot gave up — or only its *stream* did; `device.device_notes` says which | read the notes; see §6 before discarding the session |

If you supplied an `artifact`, also watch `device.install_status` — wire
values `PREVIEW_INSTALL_STATUS_UNSPECIFIED` (not fired yet) →
`PREVIEW_INSTALL_STATUS_PENDING` (waiting for the device) →
`PREVIEW_INSTALL_STATUS_RUNNING` → `PREVIEW_INSTALL_STATUS_OK` or
`PREVIEW_INSTALL_STATUS_FAILED` (`device.install_reason` says why); the CLI
shows `pending` / `running` / `ok` / `failed`. A failed install is final for
this boot and leaves the device usable — install the app yourself (§4), or
restore the session to re-run it. A healthy install is not quick — a single
attempt may hold the VM's exec channel for up to 10 minutes, and the
installer retries — so give it ~12 minutes past READY before treating it as
lost; keep polling the session while you wait, the poll itself is what
re-fires an install whose runner was lost.

`device.device_notes` may also carry *degradations* (e.g. "requested system
image X not installed; using Y") on a READY device — read them, they tell you
what you actually got.

## 3. Connect

Two ways:

1. **In-band through `execute`** (`bitrise_devenv_execute` / `bitrise-cli rde
   session exec`) — the agent default. It runs a login shell on the VM, has a
   2-minute limit per call and returns text. Every recipe in the platform
   guides works this way. Right after `running`, `execute` may answer "SSH is
   not ready" for a few minutes even though the session read already shows
   credentials — poll `bitrise_devenv_get` until `ssh_connection_open` is
   true instead of retrying blind.
2. **SSH tunnel to the device ports** from your machine (richest; needs `ssh`
   locally). `ssh_address` is a ready-made command (`ssh <user>@<host> -p
   <port>`) and `ssh_password` the password; both are on `bitrise_devenv_get`
   / `rde session view` — the list call does **not** carry them or the port
   map. `template_snapshot.service_ports` names the ports, the same on both
   platforms: `device-web-view` is the browser view of the device, forwarded
   to **local 3200** (the VM side differs: serve-sim on 3200 for iOS,
   ws-scrcpy on 8000 for Android); Android adds `adb`.
   - Android: `-L 15555:127.0.0.1:5555` then `adb connect 127.0.0.1:15555` —
     the full adb surface (~50 ms per command); `-L 3200:127.0.0.1:8000` for
     the web view.
   - iOS: `-L 3200:127.0.0.1:3200` gives you serve-sim's HTTP/WS API (stream,
     `/ax`, gestures) and the web view.

**Getting a file off the VM** (a screenshot, a log): `bitrise_devenv_download`
/ `bitrise-cli rde session download` when the deployment has a file store; if
it answers "File download is not available", use `scp` with the session's
credentials. Auth is password-based and `sshpass` is not installed, so feed
the password through `SSH_ASKPASS`:

```bash
printf '#!/bin/sh\nprintf %%s "$RDE_SSH_PASSWORD"\n' > /tmp/rde-askpass && chmod +x /tmp/rde-askpass
RDE_SSH_PASSWORD='<ssh_password>' SSH_ASKPASS=/tmp/rde-askpass SSH_ASKPASS_REQUIRE=force \
  scp -o StrictHostKeyChecking=no -P <port> <user>@<host>:/tmp/shot.png .
```

Do **not** base64 binaries through `execute` output: it has corrupted files
in testing and a native screenshot is hundreds of thousands of characters.
Pixels rarely pay for themselves anyway — read the accessibility tree (§4),
and when a human needs to see, point them at the device view (§5).

## 4. Drive the device

Platform specifics live in the iOS and Android guides
(`bitrise_devenv_device_guide` with `ios` / `android`; CLI `bitrise-cli rde
device-guide ios|android`). The shape is the same on both:

- **Accessibility tree, not pixels.** iOS: `curl -s
  http://127.0.0.1:3200/helper/<UDID>/ax` (labels, types, frames, ids as
  JSON of the frontmost app — the home screen included). Android: `adb
  exec-out uiautomator dump /dev/tty` (XML with bounds/text/resource-id). One
  call tells you what is on screen and where to tap.
- **Input.** iOS: serve-sim's CLI (`tap`, `type`, `button`, `gesture`,
  normalized 0..1 coordinates) — run it with `TMPDIR=/tmp`, see the iOS
  guide. Android: `adb shell input tap/text/keyevent`.
- **Verify, don't assume.** After a tap re-read the tree (iOS) or check the
  focused window (`adb shell dumpsys window | grep mCurrentFocus`) — the
  guides teach read and tap; confirm before you build on it.
- **Install / launch.** iOS: `xcrun simctl install <UDID> App.app` +
  `xcrun simctl launch <UDID> <bundle-id>`. Android: `adb install -r app.apk`
  + `adb shell monkey -p <package> 1` (or `am start`).
- **Screenshot.** iOS: `xcrun simctl io <UDID> screenshot /tmp/s.png`.
  Android: `adb exec-out screencap -p > /tmp/s.png`. Bring the file out as in
  §3, or point a human at the device view (§5) instead.
- **Logs.** iOS: `xcrun simctl spawn <UDID> log stream --predicate '…'`.
  Android: `adb logcat -d …`.

## 5. Let a human watch

A human watches and drives the device from the session's page in the RDE web
UI: Sessions → the session → the **Device** row → **Open device view**. It is
the same viewer page a PR preview link opens, attach-only for this session
(it attaches, never creates, and cannot delete or terminate the session), but
it requires being logged in with access to the session — its owner for a
personal session, any workspace member for a workspace-owned one. There is no
shareable link. Human taps and your taps go to the same device; do not fight
over it.

## 6. Do not break the device — and what a `FAILED` really means

The device state and the human's viewer depend on the platform's device and
its streamer process. **Never**:

- shut down, erase, delete or re-create the simulator / kill the emulator or
  wipe the AVD (`bitrise-preview` on iOS, `dev` on Android) — use the booted
  device you were given (`xcrun simctl … booted`, the single adb device);
- stop `serve-sim` (iOS, port 3200) or `ws-scrcpy` (Android, port 8000);
- look at or drive the device through the VM's *desktop* tools
  (`bitrise_devenv_screenshot` / click / type, `screencapture`, `open -a
  Simulator`). The Android emulator has no window at all; on iOS a
  Simulator.app window may be visible on the VM desktop — leave it alone and
  use the device's own API (§4);
- run the orchestrator (`~/bin/simulator-up.sh` / `~/bin/emulator-up.sh`)
  while the device is `BOOTING`.

`FAILED` is a verdict on **this boot**, and only some verdicts mean the device
is unusable. Read `device.device_notes`:

| `device_notes` says | Device usable? | Do |
|---|---|---|
| "… setup failed during provisioning — the session has no device" | no | the platform setup itself failed (wrong stack, no KVM, no toolset). Delete the session; create a new one — stack rules and the stale-default recovery are in §1 |
| "the Android emulator process never came up" / "did not finish booting within 10 minutes" | no | the VM is too weak or wedged; delete and create a new one |
| "the simulator booted but serve-sim (its streamer …) crashed on this stack" | **yes**, over `xcrun simctl` — no stream, no `/ax`, no serve-sim CLI | this macOS/Xcode stack is not supported for streamed iOS devices; for a viewer, create a new session omitting `stack_id` (or on a macOS 26 stack, §1) |
| "… booted but its video stream never delivered …" | **yes — fully drivable** over adb / simctl; only the human viewer is lost | keep working. Confirm with `adb shell getprop sys.boot_completed` / `xcrun simctl bootstatus <UDID>` before discarding anything |

Recovery by re-running the orchestrator (`~/bin/simulator-up.sh` (iOS) or
`~/bin/emulator-up.sh` (Android): boots what is not running, restarts the
streamer, re-reports readiness) is for **one** case: `device.state` is
`FAILED` **and** no run is in progress (`pgrep -f simulator-up.sh` /
`pgrep -f emulator-up.sh` prints nothing). The script exits at once when
another run holds its lock; run mid-boot it would drop the scrcpy-server the
in-progress boot depends on. It is re-runnable, not idempotent. Logs:
`~/simulator-up.log`, `~/serve-sim.log` / `~/emulator-up.log`,
`~/emulator.log`, `~/ws-scrcpy.log`. On a slow boot the orchestrator log can
go quiet for a minute or two while it probes the stream — silence there is not
proof of a hang.

## 7. Clean up

Delete the session when you are done (`bitrise_devenv_delete` /
`bitrise-cli rde session delete`). A terminated-but-not-deleted device session
can be restored: the device boots again, readiness is re-reported and the
`artifact` (if any) is installed again. `device.state` and
`device.install_status` describe the current boot only — a terminate clears
both, so a stopped session never reports a ready device or an installed app.
