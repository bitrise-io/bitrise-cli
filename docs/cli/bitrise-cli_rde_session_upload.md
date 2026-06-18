## bitrise-cli rde session upload

Upload a local file or directory into a session

### Synopsis

Upload a local file or directory into a running session.

The local path is tarred + gzipped, uploaded to cloud storage via a signed
URL, then extracted on the session VM at REMOTE_FOLDER.

For directories: the directory's contents are extracted into REMOTE_FOLDER
(not the directory itself).

```
bitrise-cli rde session upload SESSION_ID LOCAL_PATH REMOTE_FOLDER [flags]
```

### Examples

```
  bitrise-cli rde session upload SESSION_ID ./project /Users/vagrant/project
  bitrise-cli rde session upload SESSION_ID ./build.tar.gz /Users/vagrant/artifacts
```

### Options

```
  -h, --help   help for upload
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

