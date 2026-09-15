## bitrise-cli rde session ssh

Print SSH connection details (command, host, port, user, password) for a session

### Synopsis

Print the SSH connection details for a running session: a ready-to-run ssh
command line, the host, port and user it decomposes into, and the session's
SSH password.

The password is ephemeral and tied to this session. `rde session view` and
`--output json` on other commands intentionally hide it; this command is the
opt-in way to get it — for scp, an SSH tunnel to a device session's ports, or
any tool that is not `rde session exec` (which dials for you and needs none
of this).

Human mode prints the command on the first line and the password on the
second. --password-only prints just the password, so it can be fed to an
SSH_ASKPASS helper without parsing:

  RDE_SSH_PASSWORD="$(bitrise-cli rde session ssh SESSION_ID --password-only)"

--output json emits {address, host, port, user, password, command}.

```
bitrise-cli rde session ssh SESSION_ID [flags]
```

### Examples

```
  bitrise-cli rde session ssh SESSION_ID
  bitrise-cli rde session ssh SESSION_ID --output json
  RDE_SSH_PASSWORD="$(bitrise-cli rde session ssh SESSION_ID --password-only)"
```

### Options

```
  -h, --help            help for ssh
      --password-only   print only the SSH password (for SSH_ASKPASS helpers and scripts)
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

