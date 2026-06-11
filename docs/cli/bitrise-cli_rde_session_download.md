## bitrise-cli rde session download

Download a file or directory from a session

### Synopsis

Download a file or directory from a running session to the local machine.

The remote path is tar+gzip'd server-side, served via a signed download URL,
then extracted into LOCAL_PATH locally.

When REMOTE_PATH is a directory, the directory itself is recreated inside
LOCAL_PATH by default. Pass --only-contents to drop just its contents into
LOCAL_PATH instead.

```
bitrise-cli rde session download SESSION_ID REMOTE_PATH LOCAL_PATH [flags]
```

### Examples

```
  bitrise-cli rde session download SESSION_ID /Users/vagrant/project/build ./build
  bitrise-cli rde session download SESSION_ID /Users/vagrant/logs ./logs --only-contents
```

### Options

```
  -h, --help            help for download
      --only-contents   when REMOTE_PATH is a directory, extract only its contents (not the directory itself)
```

### Options inherited from parent commands

```
      --no-color           disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string      output format: human|json (default "human")
  -q, --quiet              suppress non-error diagnostic messages
      --theme string       color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
      --workspace string   workspace ID (or set BITRISE_WORKSPACE_ID; defaults to default_workspace_id)
```

### SEE ALSO

* [bitrise-cli rde session](bitrise-cli_rde_session.md)	 - Create, list, inspect, and manage RDE sessions

