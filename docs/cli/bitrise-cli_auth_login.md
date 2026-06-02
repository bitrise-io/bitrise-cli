## bitrise-cli auth login

Save a Bitrise access token

### Synopsis

Save a Bitrise access token for future commands to use.

There are two modes:

  1. Token paste (default).
     Prompts for a Personal Access Token (or pipes one in with --with-token).
     The token is masked when stdin is a terminal:

         bitrise-cli auth login
         echo "$BITRISE_TOKEN" | bitrise-cli auth login --with-token

  2. Email and password (--email).
     Signs in to app.bitrise.io with your account credentials, then asks the
     server to mint a fresh Personal Access Token and stores it. The cookie
     session used to mint the token is dropped immediately. Your account
     must have its email verified — run 'bitrise-cli user create' first if
     you don't yet have an account:

         bitrise-cli auth login --email alice@example.com
         printf '%s' "$PW" | bitrise-cli auth login --email alice@example.com --password-stdin

Either way the resulting token is written to
$XDG_CONFIG_HOME/bitrise/auth.yaml with 0600 permissions. The token is NOT
echoed in any output (use 'auth status' to verify, 'auth logout' to clear).

```
bitrise-cli auth login [flags]
```

### Examples

```
  bitrise-cli auth login                                       # interactive token prompt
  echo "$BITRISE_TOKEN" | bitrise-cli auth login --with-token
  bitrise-cli auth login --email alice@example.com             # interactive password prompt
  printf '%s' "$PW" | bitrise-cli auth login --email alice@example.com --password-stdin
```

### Options

```
      --email string     sign in by email/password and mint a Personal Access Token
  -h, --help             help for login
      --password-stdin   with --email, read the password from stdin without prompting
      --with-token       read token from stdin without an interactive prompt
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli auth](bitrise-cli_auth.md)	 - Manage the Bitrise access token

