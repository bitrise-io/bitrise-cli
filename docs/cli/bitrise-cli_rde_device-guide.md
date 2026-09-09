## bitrise-cli rde device-guide

Print the guide for driving a session's iOS simulator / Android emulator

### Synopsis

Print the device session guide: how to create a session that boots a virtual
device ('rde session create --device-platform ios|android'), wait for the
device to be ready ('rde session view'), connect, drive it efficiently
(accessibility tree first, then input), let a human watch, and what never to
do. Pass ios or android for that platform's specifics.

The guide is Markdown prose; --output json is rejected (there is no
single-object JSON shape for it).

```
bitrise-cli rde device-guide [ios|android] [flags]
```

### Examples

```
  bitrise-cli rde device-guide
  bitrise-cli rde device-guide ios
  bitrise-cli rde device-guide android
```

### Options

```
  -h, --help   help for device-guide
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

* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)

