## bitrise-cli rde session upload

Upload a local file or directory into a session

### Synopsis

Upload a local file or directory into a running session.

The local path is tarred + gzipped, uploaded to cloud storage via a signed
URL, then extracted on the session VM inside REMOTE_FOLDER.

REMOTE_FOLDER is always a DIRECTORY (absolute path; created if missing, owned
by the session user). It is never the name of the file you are sending:

  - a directory: its contents are extracted into REMOTE_FOLDER (not the
    directory itself), overwriting files of the same name;
  - a single file: it lands as REMOTE_FOLDER/<basename>. To replace one
    remote file, upload it into the file's PARENT directory. Naming the
    file's own path as REMOTE_FOLDER is rejected by the server.

Extracted files belong to the session user (vagrant on macOS, ubuntu on
Linux); local ownership is not carried over.

```
bitrise-cli rde session upload SESSION_ID LOCAL_PATH REMOTE_FOLDER [flags]
```

### Examples

```
  bitrise-cli rde session upload SESSION_ID ./project /Users/vagrant/project
  # replace one file: upload it into its parent directory
  bitrise-cli rde session upload SESSION_ID ./app/AndroidManifest.xml /home/ubuntu/app
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

