## bitrise-cli api

Make an authenticated request to the Bitrise API

### Synopsis

Make an authenticated HTTP request to the Bitrise API and print the response.

PATH is resolved against the configured API base URL (https://api.bitrise.io/v0.1
by default), so "/me" and "me" both work; an absolute http(s):// URL is used
verbatim.

The method defaults to GET, or POST when a body is supplied via --field or
--input. Use -X to set it explicitly.

Parameters (--field/-f, key=value):
  GET requests    appended as query-string parameters
  other methods   collected into a JSON request body
For request bodies the CLI can't express as flat key=value pairs (e.g. nested
objects), pass the JSON directly with --input.

Output:
  --output is ignored — the response body is written to stdout as-is. JSON is
  pretty-printed when stdout is a terminal; piped output is passed through
  unmodified, so "... | jq" works. A non-2xx status still prints the body but
  exits non-zero, with a diagnostic on stderr.

```
bitrise-cli api PATH [flags]
```

### Examples

```
  bitrise-cli api /me
  bitrise-cli api /apps -f sort_by=last_build_at --all | jq '.data[].title'
  bitrise-cli api /apps/APP_ID/builds?limit=10
  bitrise-cli api /apps/APP_ID/builds -X POST --input body.json
  bitrise-cli api -X DELETE /apps/APP_ID/builds/BUILD_ID -i
```

### Options

```
      --all                  follow cursor pagination and merge every page's data array
  -f, --field stringArray    add a key=value parameter (repeatable): query param for GET, JSON body field otherwise
  -H, --header stringArray   add or override a request header in 'Name: value' form (repeatable)
  -h, --help                 help for api
  -i, --include              print the response status line and headers before the body
      --input string         read the request body from a file (use "-" for stdin)
  -X, --method string        HTTP method (default "GET", or "POST" when a body is set)
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

