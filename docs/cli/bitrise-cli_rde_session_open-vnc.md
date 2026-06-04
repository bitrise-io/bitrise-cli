## bitrise-cli rde session open-vnc

Open a session's VNC endpoint in the OS-default viewer

### Synopsis

Hand the session's VNC URL to the operating system's default URL handler:

  - macOS:    /usr/bin/open
  - Linux:    xdg-open (must be installed; install x11-utils or similar)
  - Windows:  cmd /c start

The OS launches whatever app is registered for vnc:// (Screen Sharing on
macOS by default; Remmina/Vinagre on Linux; a third-party client on Windows).

The URL contains the ephemeral VNC password as a userinfo component. The
URL is passed as an argv element to the OS handler, so it is briefly
visible to other processes on the machine that can read this process's
argv (e.g. `ps`). On a single-user dev machine this is usually fine;
on a shared host, prefer `rde session vnc` and paste the URL into your
viewer manually.

```
bitrise-cli rde session open-vnc SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session open-vnc SESSION_ID
  bitrise-cli rde session open-vnc SESSION_ID --output json
```

### Options

```
  -h, --help   help for open-vnc
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace slug (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_slug)
```

### SEE ALSO

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

