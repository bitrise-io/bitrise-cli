## bitrise-cli app list

List apps the authenticated user can access

### Synopsis

List all apps (projects) the authenticated user can access.

Filters:
  --title TITLE              filter apps whose title contains the given string (case-insensitive)
  --project-type TYPE        e.g. ios, android
  --sort-by FIELD            ordering accepted by the API (e.g. created_at, last_build_at)

Pagination:
  --limit N                  max items per page (server default if 0)
  --cursor TOKEN             opaque token from a previous page's next_cursor
  --all                      fetch all pages automatically

In JSON mode (--output json), the next_cursor field holds the cursor value for scripting:
  bitrise-cli app list --output json | jq -r '.next_cursor'

```
bitrise-cli app list [flags]
```

### Examples

```
  bitrise-cli app list
  bitrise-cli app list --all
  bitrise-cli app list --output json | jq -r '.next_cursor'
  bitrise-cli app list --project-type ios --limit 100
```

### Options

```
      --all                   fetch all pages automatically
      --cursor string         pagination cursor from a previous response
  -h, --help                  help for list
      --limit int             max items per page (server default if 0)
      --project-type string   filter by project type (ios, android, ...)
      --sort-by string        ordering accepted by the API (e.g. created_at, last_build_at)
      --title string          filter apps whose title contains the given string (case-insensitive)
```

### Options inherited from parent commands

```
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli app](bitrise-cli_app.md)	 - List, inspect, and manage apps (also: project)

