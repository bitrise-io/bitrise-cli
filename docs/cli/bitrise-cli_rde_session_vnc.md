## bitrise-cli rde session vnc

Print VNC connection details, or forward the endpoint to a local port

### Synopsis

Print the VNC connection details (address, host, port, username, password,
and a ready-to-use vnc:// URL) for a session.

The VNC password is ephemeral and tied to this session. Avoid pasting the
output into chat or sharing it — anyone with the URL can connect to the
session. `rde session view` and other commands intentionally hide it.

In human mode the URL is the only thing on stdout, so it's safe to pipe:

  open "$(bitrise-cli rde session vnc SESSION_ID)"

In --output json mode a fully-decomposed {address, host, port, username,
password, url} object is emitted — host and port are always discrete fields,
so a caller building its own connection never has to parse the address or URL.

Pass --forward PORT to open an SSH tunnel and expose the session's VNC endpoint
on a local port, then block until Ctrl-C (use 0 to auto-pick a free port):

  bitrise-cli rde session vnc SESSION_ID --forward 0      # auto-pick a local port
  bitrise-cli rde session vnc SESSION_ID --forward 5901   # bind localhost:5901

A native VNC client (macOS Screen Sharing, Remmina, …) can then connect to the
printed localhost address. The tunnel rides the same SSH connection the CLI
already uses, so no direct network route to the session is required and no
credentials are embedded in a URL handed to the OS. Prefer `rde session open-vnc`
when you just want to launch your viewer against a directly-reachable endpoint.

```
bitrise-cli rde session vnc SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session vnc SESSION_ID
  bitrise-cli rde session vnc SESSION_ID --output json
  bitrise-cli rde session vnc SESSION_ID --forward 5901
  open "$(bitrise-cli rde session vnc SESSION_ID)"
```

### Options

```
      --forward int   forward the session's VNC endpoint to this local port, then block until Ctrl-C; use 0 to auto-pick a free port
  -h, --help          help for vnc
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace ID (or set BITRISE_WORKSPACE_ID or default_workspace_id; auto-detected if you have exactly one workspace)
```

### SEE ALSO

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

