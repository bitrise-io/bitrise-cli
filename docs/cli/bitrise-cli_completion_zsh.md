## bitrise-cli completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(bitrise-cli completion zsh)

To load completions for every new session, execute once:

#### Linux:

	bitrise-cli completion zsh > "${fpath[1]}/_bitrise-cli"

#### macOS:

	bitrise-cli completion zsh > $(brew --prefix)/share/zsh/site-functions/_bitrise-cli

You will need to start a new shell for this setup to take effect.


```
bitrise-cli completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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

