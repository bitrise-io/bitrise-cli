## bitrise-cli rde preview-link

Create shareable device preview links for app builds

### Synopsis

Create a shareable link that opens an app build on a live iOS simulator or
Android emulator in the recipient's browser — no Bitrise login, no local
tooling, nothing for them to install.

Minting is the only operation: nothing is stored when a link is created, so a
link cannot be listed, read back or revoked — it is used until it expires. The
sessions a link opens are ordinary sessions owned by the workspace, whoever
minted the link; see 'rde session list --scope workspace'.

### Options

```
  -h, --help   help for preview-link
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

* [bitrise-cli rde](bitrise-cli_rde.md)	 - Manage Bitrise Remote Dev Environments (sessions, templates, …)
* [bitrise-cli rde preview-link create](bitrise-cli_rde_preview-link_create.md)	 - Mint a shareable device preview link for an app build

