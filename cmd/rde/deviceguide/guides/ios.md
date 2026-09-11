<!-- Mirror of the RDE device-session guide shipped with the RDE backend; keep in sync with the backend release you target — do not edit here. -->
# iOS simulator sessions — driving the device

Read the main device sessions guide first (create, wait for READY, connect,
do-nots): `README.md` next to this file, the MCP resource
`bitrise-devenv://guides/device-sessions` (or the `bitrise_devenv_device_guide`
tool when resources are unavailable), or `bitrise-cli rde device-guide`.
This page is the iOS specifics. Everything here runs on the session VM
(in-band via `execute`, or over SSH); the simulator is **headless** — there is
no Simulator.app window and the macOS desktop screenshot shows nothing.

## What is on the VM

- One simulator named **`bitrise-preview`**, booted. Find it with
  `xcrun simctl list devices booted` (the UDID is the value in parentheses).
  `booted` also works as the UDID argument to most `simctl` commands while it
  is the only booted device.
- **serve-sim** (pinned npm package in `~/serve-sim`) streaming it on
  `127.0.0.1:3200` — this is the viewer's video source *and* your input/AX
  API. Its CLI: `cd ~/serve-sim && node node_modules/.bin/serve-sim …`.
- Service port on the session: `device-web-view` (VM 3200 → local 3200, the
  same name and local port an Android session uses). From your machine:
  `ssh -N -L 3200:127.0.0.1:3200 …` then `http://127.0.0.1:3200`.
- Xcode with the stack's iOS runtimes; `xcrun simctl` for everything device
  lifecycle-ish (install, launch, screenshot, logs, appearance, location…).
- Python 3.13 (asdf) — `pipx install fb-idb` works if you want the `idb`
  client; `idb_companion` is not pre-installed (Homebrew 6 requires
  `brew tap facebook/fb && brew trust facebook/fb && brew install
  idb-companion`). You do not need idb for anything below.

## Accessibility tree (what is on screen, where)

```bash
UDID=$(xcrun simctl list devices booted | sed -nE 's/.*\(([0-9A-F-]{36})\).*/\1/p' | head -1)
curl -s "http://127.0.0.1:3200/helper/$UDID/ax"
```

Returns a JSON array of element trees for the **frontmost app**: each node has
`type` (Button, Heading, StaticText, TextField, Cell, …), `AXLabel`,
`AXValue`, `AXUniqueId` (accessibility identifier when the app sets one),
`enabled`, `frame` (`x`,`y`,`width`,`height` in **points**, portrait-up), and
`children`. On the home screen (no frontmost app) it answers
`503 {"error":"ax_unavailable","message":"noFrontmostApplication"}` — launch
an app first. Filter it (jq / python) rather than reading it raw; a Settings
screen is ~30 nodes, a busy app can be hundreds.

Point → normalized tap coordinate: `x/screenWidth`, `y/screenHeight`, where
the screen size in points comes from the `screen` object in serve-sim's root
`/ax` SSE feed (`curl -sN --max-time 2 http://127.0.0.1:3200/ax | head -c
400` — e.g. `"screen":{"width":393,"height":852}` for an iPhone 16). Frames
in the per-device `/helper/<UDID>/ax` JSON are in the same point space.

## Input (serve-sim CLI)

```bash
S="node node_modules/.bin/serve-sim"; cd ~/serve-sim
$S tap    -d "$UDID" 0.5 0.47            # normalized 0..1 coordinates
$S type   -d "$UDID" "hello world"       # US keyboard; focus a text field first
$S button -d "$UDID" home                # hardware button; "home" is the documented name
$S gesture -d "$UDID" '{"type":"begin","x":0.5,"y":0.8}'   # then move/end events for swipes
```

Each invocation is a Node process (~0.2–0.8 s); batch several in one
`execute` call. Alternatives: `xcrun simctl` has **no** tap/type; `idb ui
tap/text` works if you installed idb.

## Install, launch, screenshot, logs

```bash
xcrun simctl install "$UDID" /path/App.app          # simulator build (arm64 on Apple silicon hosts)
xcrun simctl launch  "$UDID" com.example.app        # prints the pid
xcrun simctl terminate "$UDID" com.example.app
xcrun simctl io "$UDID" screenshot --type=png /tmp/shot.png
xcrun simctl spawn "$UDID" log stream --style compact --predicate 'process == "MyApp"'
xcrun simctl ui "$UDID" appearance dark            # light|dark
xcrun simctl openurl "$UDID" "myapp://deep/link"
```

Screenshots are ~350 KB PNG at native resolution; `sips -Z 800 in.png --out
small.png` gets ~90 KB. Do not base64 them into `execute` output unless you
must — point the human at the session page's device view, or copy the file out over the
tunnel / `bitrise_devenv_download`.

## Do not

- `xcrun simctl shutdown|erase|delete|create` on `bitrise-preview`, or boot a
  second device and drive that one — serve-sim streams the UDID it was started
  with; the viewer and `device.state` follow *that* device.
- kill `serve-sim` or anything on port 3200; `open -a Simulator`.

Recovery: `~/bin/simulator-up.sh` (idempotent: boots if needed, restarts
serve-sim, re-reports readiness). Then re-check `device.state`.
