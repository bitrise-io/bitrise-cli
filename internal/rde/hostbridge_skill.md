---
name: rde-host
description: Run an action on the user's local machine from inside this remote session — currently, open a VNC viewer on their screen showing this session's desktop. Use when the user asks to see the desktop, screen, GUI, simulator, emulator, or browser running in the session, or whenever a visual result is worth showing them directly.
---

# Acting on the user's local machine

This session can reach the user's local machine through a small "host bridge".
You cannot show a GUI to the user by opening it here — their screen is on their
own machine, not in this session. To show them something visual, ask the host
bridge to open a VNC viewer locally; it connects back to this session's desktop.

## Opening a VNC viewer on the user's machine

1. Read the bridge address and token. They live in a file and the port can
   change if the connection reconnects, so **read the file every time** rather
   than remembering an old value:

   ```bash
   cat ~/.config/rde/host-bridge.json
   ```

   It contains `{"url": "...", "token": "..."}`. If the file does not exist, the
   host bridge is not available in this session — tell the user you can't open a
   viewer on their machine, and stop.

2. POST to the `open-vnc` action using those values:

   ```bash
   curl -fsS -X POST "$URL/open-vnc" -H "Authorization: Bearer $TOKEN"
   ```

   where `$URL` and `$TOKEN` are the `url` and `token` you just read.

3. On success the response is `{"opened": true, ...}` and a VNC viewer opens on
   the user's machine. Let them know it should now be on their screen. On a
   non-2xx response, report what the bridge returned.

Only these documented actions are available — the bridge ignores anything else.
