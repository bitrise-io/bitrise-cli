## bitrise-cli completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(bitrise-cli completion bash)

To load completions for every new session, execute once:

#### Linux:

	bitrise-cli completion bash > /etc/bash_completion.d/bitrise-cli

#### macOS:

	bitrise-cli completion bash > $(brew --prefix)/etc/bash_completion.d/bitrise-cli

You will need to start a new shell for this setup to take effect.


```
bitrise-cli completion bash
```

### Options

```
  -h, --help              help for bash
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

