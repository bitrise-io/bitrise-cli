## bitrise-cli purr

Visit Purr Request, the Bitrise CLI mascot

### Synopsis

Visit Purr Request — the rocket-powered cat that's always here to help you.

The mascot animates with a swinging tail. The animation runs for --duration
(default 8s) or until Ctrl-C; --once disables animation and prints a single
frame. When stdout is not a terminal (piped output, log file) the command
always prints once and exits regardless of --once.

```
bitrise-cli purr [flags]
```

### Examples

```
  bitrise-cli purr
  bitrise-cli purr --duration 30s
  bitrise-cli purr --once
```

### Options

```
      --duration duration   how long to animate before exiting (default 8s)
  -h, --help                help for purr
      --interval duration   delay between animation frames (default 250ms)
      --once                print a single frame instead of animating
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli](bitrise-cli.md)	 - Bitrise platform CLI

