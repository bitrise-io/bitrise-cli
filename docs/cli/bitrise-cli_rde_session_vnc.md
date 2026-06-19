## bitrise-cli rde session vnc

Print VNC connection credentials for a session

### Synopsis

Print the VNC connection details (address, username, password, and a
ready-to-use vnc:// URL) for a session.

The VNC password is ephemeral and tied to this session. Avoid pasting the
output into chat or sharing it — anyone with the URL can connect to the
session. `rde session view` and other commands intentionally hide it.

In human mode the URL is the only thing on stdout, so it's safe to pipe:

  open "$(bitrise-cli rde session vnc SESSION_ID)"

In --output json mode a {address, username, password, url} object is
emitted. Prefer `rde session open-vnc` when you just want to launch your
viewer — that hands the URL to the OS without printing the password.

```
bitrise-cli rde session vnc SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session vnc SESSION_ID
  bitrise-cli rde session vnc SESSION_ID --output json
  open "$(bitrise-cli rde session vnc SESSION_ID)"
```

### Options

```
  -h, --help   help for vnc
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

