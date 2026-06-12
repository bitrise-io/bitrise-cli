package bitriseapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RawResponse is the result of a RawRequest call: the full HTTP response with
// its status, headers, and body read into memory. Unlike the typed client
// methods, a non-2xx status is reported here as a normal response (StatusCode
// set, no error) so a raw passthrough can print the body and pick its own exit
// behavior.
type RawResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// RawRequest issues an arbitrary authenticated request to the Bitrise API and
// returns the full response. path may be relative to the configured base URL
// (with or without a leading slash) or an absolute http(s):// URL, which is
// used verbatim. query is merged onto any query already present in path.
// header values are applied on top of the defaults (Authorization, Accept) and
// may override them. body is the optional request body.
//
// Only transport or request-construction failures return a non-nil error; an
// HTTP error status is conveyed via RawResponse.StatusCode, not as an error.
func (c *Client) RawRequest(ctx context.Context, method, path string, query url.Values, header http.Header, body io.Reader) (*RawResponse, error) {
	u, err := c.resolveURL(path)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		merged := u.Query()
		for k, vs := range query {
			for _, v := range vs {
				merged.Add(k, v)
			}
		}
		u.RawQuery = merged.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	for k, vs := range header {
		req.Header.Del(k)
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.httpClient.Do(req) //nolint:gosec // the raw API command intentionally lets the user control the method and URL
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return &RawResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       respBody,
	}, nil
}

// resolveURL turns a user-supplied path into an absolute URL. Absolute http(s)
// URLs are used as-is; anything else is joined to the client's base URL with
// exactly one slash between them, so both "/apps" and "apps" resolve correctly.
func (c *Client) resolveURL(path string) (*url.URL, error) {
	raw := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		raw = strings.TrimRight(c.baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	return u, nil
}
