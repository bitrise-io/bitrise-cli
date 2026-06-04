## bitrise-cli completion

Generate the autocompletion script for the specified shell

### Synopsis

Generate a shell completion script for your shell.

bash — load now:                    source <(bitrise-cli completion bash)
bash — make permanent:              echo 'source <(bitrise-cli completion bash)' >> ~/.bashrc

zsh — load now:                     source <(bitrise-cli completion zsh)
zsh — make permanent:               echo 'source <(bitrise-cli completion zsh)' >> ~/.zshrc

fish — load now:                    bitrise-cli completion fish | source
fish — make permanent:              bitrise-cli completion fish > ~/.config/fish/completions/bitrise-cli.fish

PowerShell — load now:              bitrise-cli completion powershell | Out-String | Invoke-Expression
PowerShell — make permanent:        add the above line to your $PROFILE

### Options

```
  -h, --help   help for completion
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
* [bitrise-cli completion bash](bitrise-cli_completion_bash.md)	 - Generate the autocompletion script for bash
* [bitrise-cli completion fish](bitrise-cli_completion_fish.md)	 - Generate the autocompletion script for fish
* [bitrise-cli completion powershell](bitrise-cli_completion_powershell.md)	 - Generate the autocompletion script for powershell
* [bitrise-cli completion zsh](bitrise-cli_completion_zsh.md)	 - Generate the autocompletion script for zsh

