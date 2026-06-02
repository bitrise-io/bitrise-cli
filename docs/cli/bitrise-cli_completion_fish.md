## bitrise-cli completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	bitrise-cli completion fish | source

To load completions for every new session, execute once:

	bitrise-cli completion fish > ~/.config/fish/completions/bitrise-cli.fish

You will need to start a new shell for this setup to take effect.


```
bitrise-cli completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli completion](bitrise-cli_completion.md)	 - Generate the autocompletion script for the specified shell

