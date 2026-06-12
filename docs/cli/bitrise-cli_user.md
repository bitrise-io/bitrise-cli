## bitrise-cli user

Create and manage your Bitrise account

### Synopsis

Manage your own Bitrise account from the CLI.

Today this surface is limited to account creation. After running
'user create' you must click the link emailed to you, then run
'bitrise-cli auth login --email <addr>' to mint and store an access token.

### Examples

```
  bitrise-cli user me
  bitrise-cli user me --output json
  bitrise-cli user create --email alice@example.com --username alice --first-name Alice --last-name L
```

### Options

```
  -h, --help   help for user
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
* [bitrise-cli user create](bitrise-cli_user_create.md)	 - Create a new Bitrise account
* [bitrise-cli user me](bitrise-cli_user_me.md)	 - Show the currently authenticated user

