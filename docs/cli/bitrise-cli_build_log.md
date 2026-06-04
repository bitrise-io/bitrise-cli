## bitrise-cli build log

Print the build log

### Synopsis

Print the log output for a single build.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Argument:
  BUILD_SLUG         the unique slug of the build

Flags:
  --wait             wait for the build to finish before printing the log;
                     useful when the build is still in-progress. Ctrl-C
                     detaches without affecting the running build. Exit status
                     reflects log retrieval, not the build outcome — use
                     'build watch' to gate on build success/failure.
  --interval DURATION  polling interval when --wait is active (default 3s)

Note:
  --output is ignored — logs are streamed as raw text. Pipe to other tools or
  redirect to a file as needed.

```
bitrise-cli build log BUILD_SLUG [flags]
```

### Examples

```
  bitrise-cli build log --app my-app-slug <build-slug>
  bitrise-cli build log --app my-app-slug <build-slug> --wait
  bitrise-cli build log --app my-app-slug <build-slug> --wait --interval 10s
  bitrise-cli build log --app my-app-slug <build-slug> > build.log
```

### Options

```
  -h, --help                help for log
      --interval duration   polling interval when --wait is active (default 3s)
      --wait                wait for the build to finish before printing the log
```

### Options inherited from parent commands

```
      --app string      app slug (also accepted as --project; or set BITRISE_APP_SLUG)
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli build](bitrise-cli_build.md)	 - Trigger, list, and inspect builds

