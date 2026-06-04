## bitrise-cli rde session logs

Print a session's warmup or startup logs

### Synopsis

Print the warmup or startup script logs for a session — useful for debugging a
session stuck provisioning or one that came up failed. The stream replays the
whole stage log from the start on every connect.

Note: the backend does not currently signal end-of-log, so the command keeps
running — even after the script has finished — until you stop it with Ctrl-C.
This applies to both modes; redirect or pipe stdout and Ctrl-C once output
stops.

  --stage    which script's logs to show: warmup or startup (required). warmup
             runs once at session creation; startup runs on every session
             start/restart.
  --follow   if the stage hasn't produced any logs yet, wait for it to start
             rather than erroring. Without --follow the command errors right
             away when logs aren't available yet.

--output is ignored — logs stream as raw text. Pipe or redirect as needed;
diagnostics go to stderr so a redirect captures only log text. --output json
is rejected (the feed is plain text, not a single object).

```
bitrise-cli rde session logs SESSION_ID --stage warmup|startup [flags]
```

### Examples

```
  bitrise-cli rde session logs SESSION_ID --stage startup
  bitrise-cli rde session logs SESSION_ID --stage warmup
  bitrise-cli rde session logs SESSION_ID --stage startup --follow
  bitrise-cli rde session logs SESSION_ID --stage startup > session.log
```

### Options

```
  -f, --follow                    keep streaming until Ctrl-C, waiting for the stage to start if needed
  -h, --help                      help for logs
      --retry-interval duration   poll interval while waiting for the stage to start (only with --follow) (default 3s)
      --stage string              which logs to show: warmup or startup (required)
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

