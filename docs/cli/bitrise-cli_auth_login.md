## bitrise-cli auth login

Save a Bitrise access token

### Synopsis

Save a Bitrise access token for future commands to use.

By default, in an interactive terminal, this opens your browser to sign in to
Bitrise (OAuth) and stores a managed, auto-refreshing token. The modes:

  Browser sign-in (default in an interactive terminal; explicit with --oauth).
     Opens your browser to sign in, exchanges the result for a Personal Access
     Token, and refreshes it automatically so you rarely sign in again:

         bitrise-cli auth login
         bitrise-cli auth login --oauth

     This needs the browser on the same machine as the CLI (the sign-in is
     handed back over a loopback address). On a remote/headless host over SSH
     it can't complete — pipe a token instead (see below).

  Token (--with-token, or any non-interactive stdin).
     Reads a Personal Access Token from stdin. This is also used automatically
     when stdin is not a terminal, so CI and pipes keep working without a flag:

         echo "$BITRISE_PAT" | bitrise-cli auth login
         echo "$BITRISE_PAT" | bitrise-cli auth login --with-token

  Email and password (--email).
     Signs in to app.bitrise.io with your account credentials, then asks the
     server to mint a fresh Personal Access Token and stores it. The cookie
     session used to mint the token is dropped immediately. Your account must
     have its email verified — run 'bitrise-cli user create' first if you don't
     yet have an account:

         bitrise-cli auth login --email alice@example.com
         printf '%s' "$PW" | bitrise-cli auth login --email alice@example.com --password-stdin

The resulting token is written to $XDG_CONFIG_HOME/bitrise/auth.yaml with 0600
permissions and is never echoed (use 'auth status' to verify, 'auth logout' to
clear).

```
bitrise-cli auth login [flags]
```

### Examples

```
  bitrise-cli auth login                                     # browser sign-in (OAuth)
  echo "$BITRISE_PAT" | bitrise-cli auth login --with-token  # paste/pipe a token
  bitrise-cli auth login --email alice@example.com           # email/password
```

### Options

```
      --email string     sign in by email/password and mint a Personal Access Token
  -h, --help             help for login
      --oauth            sign in via the browser (OAuth) and store a managed, auto-refreshing token
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

