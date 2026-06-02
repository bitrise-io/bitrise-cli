## bitrise-cli user create

Create a new Bitrise account

### Synopsis

Create a new Bitrise account by email and password.

Required flags:
  --email ADDRESS    the email address to register
  --username NAME    desired username (must be unique)

Optional flags:
  --first-name N     first name on the account
  --last-name N      last name on the account
  --password-stdin   read the password from stdin instead of prompting

Password input:
  By default the command prompts for the password (input is masked when stdin
  is a terminal). Use --password-stdin to read it from stdin without a prompt
  — the right choice for piping or scripts:

      printf '%s' "$NEW_PASSWORD" | bitrise-cli user create \
          --email a@b.io --username alice --password-stdin

Email verification:
  After signup the server emails a verification link. Click it before running
  'bitrise-cli auth login --email <addr>' — sign-in is blocked on unverified
  accounts.

```
bitrise-cli user create [flags]
```

### Examples

```
  bitrise-cli user create --email alice@example.com --username alice --first-name Alice --last-name L
  printf '%s' "$NEW_PASSWORD" | bitrise-cli user create \
      --email alice@example.com --username alice --password-stdin --output json
```

### Options

```
      --email string        email address to register (required)
      --first-name string   first name on the account
  -h, --help                help for create
      --last-name string    last name on the account
      --password-stdin      read the password from stdin without prompting
      --username string     desired username (required)
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli user](bitrise-cli_user.md)	 - Create and manage your Bitrise account

