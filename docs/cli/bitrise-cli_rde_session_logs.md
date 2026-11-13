## bitrise-cli rde session logs

Print a session's setup logs: a lifecycle hook or the platform's own steps

### Synopsis

Print a session's setup log — one lifecycle hook's output or the
platform's own steps — useful for debugging a session stuck provisioning or
one whose setup failed. The log replays from the start every time you connect.

By default this prints the log captured so far and exits, without waiting for
more output. Pass --follow to keep streaming new output live until you stop it
with Ctrl-C (the backend does not signal end-of-log, so --follow runs until
interrupted).

  --stage    which log to show (required). On a session that runs lifecycle
             hooks ('session view' shows a Setup line): on-create, on-claim
             and on-restore are that hook's output, every run in order;
             personalize is the platform's System setup step alone
             (credentials, Claude Code — queued after On create); system is
             the platform's own output (its boot steps and the System setup,
             without the hooks' lines); startup is the combined log of the
             platform and every hook. On a session created before
             lifecycle hooks: warmup is the (deprecated) warmup script,
             run once at session creation; startup is the (deprecated)
             startup script, run on every session start/restart.
  --attempt  on a hook stage, show only this attempt of the hook (1-based,
             the numbers 'session view' lists); default: every attempt in
             order, each re-run preceded by an "── attempt N ──" marker.
  --follow   keep streaming new output until Ctrl-C, and wait for the stage to
             start if it hasn't produced any logs yet (instead of erroring).

--output is ignored — logs stream as raw text. Pipe or redirect as needed;
diagnostics go to stderr so a redirect captures only log text. --output json
is rejected (the feed is plain text, not a single object).

```
bitrise-cli rde session logs SESSION_ID --stage on-create|on-claim|on-restore|personalize|system|startup|warmup [--attempt N] [flags]
```

### Examples

```
  bitrise-cli rde session logs SESSION_ID --stage on-create
  bitrise-cli rde session logs SESSION_ID --stage on-create --follow
  bitrise-cli rde session logs SESSION_ID --stage on-claim --attempt 2
  bitrise-cli rde session logs SESSION_ID --stage startup > session.log
```

### Options

```
      --attempt int    on a hook stage, only this attempt of the hook (1-based); 0 = every attempt in order
  -f, --follow         keep streaming until Ctrl-C, waiting for the stage to start if needed
  -h, --help           help for logs
      --stage string   which log to show: warmup, startup, on-create, on-claim, on-restore, system or personalize (required)
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

