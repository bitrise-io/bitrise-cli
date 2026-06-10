## bitrise-cli build list

List builds for an app

### Synopsis

List builds for an app, newest first.

Required flags:
  --app ID                (or BITRISE_APP_ID env var)

Optional filters:
  --branch BRANCH           filter by branch name
  --workflow ID             filter by workflow ID
  --status STATUS           one of: success, failed, in-progress, aborted, aborted-with-success
  --sort-by ORDER           one of: created_at (default), running_first
  --commit-message TEXT     filter by commit message
  --trigger-event-type TYPE one of: push, pull-request, tag
  --pull-request-id N       filter by pull request ID
  --build-number N          filter by build number
  --after RFC3339           builds triggered after this time (e.g. 2024-01-15T00:00:00Z)
  --before RFC3339          builds triggered before this time
  --pipeline-build          show only pipeline builds

Pagination:
  --limit N               max items per page (server default if 0)
  --cursor TOKEN          opaque token from a previous page's next_cursor
  --all                   fetch all pages automatically

In JSON mode (--output json), the next_cursor field holds the cursor value for scripting:
  bitrise-cli build list --app ID --output json | jq -r '.next_cursor'

```
bitrise-cli build list [flags]
```

### Examples

```
  bitrise-cli build list --app my-app-id
  bitrise-cli build list --app my-app-id --all
  bitrise-cli build list --app my-app-id --branch main --status failed
  bitrise-cli build list --app my-app-id --sort-by running_first
  bitrise-cli build list --app my-app-id --after 2024-01-01T00:00:00Z
  bitrise-cli build list --app my-app-id --output json
```

### Options

```
      --after string                show builds triggered after this time (RFC3339, e.g. 2024-01-15T00:00:00Z)
      --all                         fetch all pages automatically
      --before string               show builds triggered before this time (RFC3339, e.g. 2024-01-15T00:00:00Z)
      --branch string               filter by branch
      --build-number int            filter by build number
      --commit-message string       filter by commit message
      --cursor string               pagination cursor from a previous response
  -h, --help                        help for list
      --limit int                   max items per page (server default if 0)
      --pipeline-build              show only pipeline builds (omit to show all)
      --pull-request-id int         filter by pull request ID
      --sort-by string              sort order: created_at (default) or running_first
      --status string               filter by status (success, failed, in-progress, aborted, aborted-with-success)
      --trigger-event-type string   filter by trigger event type (push, pull-request, tag)
      --workflow string             filter by workflow ID
```

### Options inherited from parent commands

```
      --app string      app ID (also accepted as --project; or set BITRISE_APP_ID)
      --no-color        disable ANSI colors (NO_COLOR env is also honored)
  -o, --output string   output format: human|json (default "human")
  -q, --quiet           suppress non-error diagnostic messages
      --theme string    color theme: auto|dark|light|none (default "auto"; overrides terminal background detection)
```

### SEE ALSO

* [bitrise-cli build](bitrise-cli_build.md)	 - Trigger, list, and inspect builds

