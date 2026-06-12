// Package api implements the `bitrise-cli api` raw API passthrough command.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapi "github.com/bitrise-io/bitrise-cli/internal/api"
)

// NewCmd returns the `bitrise-cli api` command.
func NewCmd() *cobra.Command {
	var (
		method   string
		fields   []string
		headers  []string
		input    string
		paginate bool
		include  bool
	)

	c := &cobra.Command{
		Use:   "api PATH",
		Short: "Make an authenticated request to the Bitrise API",
		Long: `Make an authenticated HTTP request to the Bitrise API and print the response.

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
  exits non-zero, with a diagnostic on stderr.`,
		Example: `  bitrise-cli api /me
  bitrise-cli api /apps -f sort_by=last_build_at --all | jq '.data[].title'
  bitrise-cli api /apps/APP_ID/builds?limit=10
  bitrise-cli api /apps/APP_ID/builds -X POST --input body.json
  bitrise-cli api -X DELETE /apps/APP_ID/builds/BUILD_ID -i`,
		Args: cmdutil.RequireArgs("PATH"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}

			path := args[0]

			hasBody := len(fields) > 0 || input != ""
			m := strings.ToUpper(method)
			if m == "" {
				m = http.MethodGet
				if hasBody {
					m = http.MethodPost
				}
			}

			kvFields, err := parseFields(fields)
			if err != nil {
				return err
			}
			hdr, err := parseHeaders(headers)
			if err != nil {
				return err
			}

			var body io.Reader
			if input != "" {
				r, cleanup, err := openInput(cmd, input)
				if err != nil {
					return err
				}
				defer cleanup()
				body = r
			}

			svc := internalapi.NewService(client)
			resp, err := svc.Do(cmd.Context(), internalapi.Request{
				Method:   m,
				Path:     path,
				Fields:   kvFields,
				Headers:  hdr,
				Body:     body,
				Paginate: paginate,
			})
			if err != nil {
				return err
			}

			if err := writeResponse(cmd.OutOrStdout(), resp, include); err != nil {
				return err
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("bitrise API responded with HTTP %d", resp.StatusCode)
			}
			return nil
		},
	}

	c.Flags().StringVarP(&method, "method", "X", "", `HTTP method (default "GET", or "POST" when a body is set)`)
	c.Flags().StringArrayVarP(&fields, "field", "f", nil, "add a key=value parameter (repeatable): query param for GET, JSON body field otherwise")
	c.Flags().StringArrayVarP(&headers, "header", "H", nil, "add or override a request header in 'Name: value' form (repeatable)")
	c.Flags().StringVar(&input, "input", "", `read the request body from a file (use "-" for stdin)`)
	c.Flags().BoolVar(&paginate, "all", false, "follow cursor pagination and merge every page's data array")
	c.Flags().BoolVarP(&include, "include", "i", false, "print the response status line and headers before the body")
	c.MarkFlagsMutuallyExclusive("field", "input")
	return c
}

func parseFields(raw []string) ([]internalapi.KeyValue, error) {
	out := make([]internalapi.KeyValue, 0, len(raw))
	for _, f := range raw {
		k, v, ok := strings.Cut(f, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --field %q: expected key=value", f)
		}
		out = append(out, internalapi.KeyValue{Key: k, Value: v})
	}
	return out, nil
}

func parseHeaders(raw []string) (http.Header, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	h := http.Header{}
	for _, hdr := range raw {
		k, v, ok := strings.Cut(hdr, ":")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("invalid --header %q: expected 'Name: value'", hdr)
		}
		h.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}
	return h, nil
}

func openInput(cmd *cobra.Command, path string) (io.Reader, func(), error) {
	if path == "-" {
		return cmd.InOrStdin(), func() {}, nil
	}
	f, err := os.Open(path) //nolint:gosec // the api command intentionally reads a user-named request-body file
	if err != nil {
		return nil, nil, fmt.Errorf("open --input file: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}

func writeResponse(out io.Writer, resp internalapi.Response, include bool) error {
	if include {
		ew := cmdutil.NewErrWriter(out)
		ew.F("HTTP %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		keys := make([]string, 0, len(resp.Header))
		for k := range resp.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range resp.Header[k] {
				ew.F("%s: %s\n", k, v)
			}
		}
		ew.Ln()
		if ew.Err != nil {
			return ew.Err
		}
	}

	body := resp.Body
	if shouldPrettyPrint(out, resp.Header) {
		var buf bytes.Buffer
		if json.Indent(&buf, body, "", "  ") == nil {
			body = buf.Bytes()
			if len(body) > 0 && body[len(body)-1] != '\n' {
				body = append(body, '\n')
			}
		}
	}
	_, err := out.Write(body)
	return err
}

// shouldPrettyPrint reports whether to indent a JSON body: only when the
// response is JSON and stdout is an interactive terminal. Piped output is left
// byte-for-byte so downstream tools (jq, redirects) get exactly what the API
// returned.
func shouldPrettyPrint(w io.Writer, header http.Header) bool {
	if !strings.Contains(header.Get("Content-Type"), "json") {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptors are small ints, no overflow risk
}
