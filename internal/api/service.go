// Package api holds the business-logic layer for the raw API passthrough
// command. It assembles a user-directed request (method, path, fields,
// headers, body) and, optionally, follows cursor pagination — leaving all HTTP
// transport to the bitriseapi client and all formatting to the cmd layer.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

// KeyValue is a single -f/--field pair.
type KeyValue struct {
	Key   string
	Value string
}

// Request describes a raw API call assembled from the api command's flags.
type Request struct {
	Method   string      // HTTP method, already defaulted and upcased by the caller
	Path     string      // endpoint path or absolute URL
	Fields   []KeyValue  // -f/--field
	Headers  http.Header // -H/--header
	Body     io.Reader   // --input body; mutually exclusive with Fields
	Paginate bool        // --all
}

// Response is the result of a raw API call. When Paginate is set, Body is the
// merged "data" arrays of every page.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Service performs raw, user-directed Bitrise API requests.
type Service struct {
	client *bitriseapi.Client
}

// NewService returns a Service backed by client.
func NewService(client *bitriseapi.Client) *Service {
	return &Service{client: client}
}

// Do performs the request described by req. Fields become query parameters for
// GET and a JSON object body for write methods. A non-2xx status is returned in
// Response.StatusCode, not as an error; only transport failures error out.
func (s *Service) Do(ctx context.Context, req Request) (Response, error) {
	if s.client == nil {
		return Response{}, fmt.Errorf("API client not configured")
	}
	if len(req.Fields) > 0 && req.Body != nil {
		return Response{}, fmt.Errorf("cannot combine --field with --input")
	}

	isGet := req.Method == http.MethodGet
	header := cloneHeader(req.Headers)

	var query url.Values
	body := req.Body
	if len(req.Fields) > 0 {
		if isGet {
			query = url.Values{}
			for _, f := range req.Fields {
				query.Add(f.Key, f.Value)
			}
		} else {
			obj := make(map[string]string, len(req.Fields))
			for _, f := range req.Fields {
				obj[f.Key] = f.Value
			}
			data, err := json.Marshal(obj)
			if err != nil {
				return Response{}, fmt.Errorf("encode fields: %w", err)
			}
			body = bytes.NewReader(data)
		}
	}
	// Default Content-Type for any write request carrying a body.
	if body != nil && !isGet && header.Get("Content-Type") == "" {
		header.Set("Content-Type", "application/json")
	}

	if req.Paginate {
		if !isGet {
			return Response{}, fmt.Errorf("--all is only supported for GET requests")
		}
		return s.paginate(ctx, req.Path, query, header)
	}

	resp, err := s.client.RawRequest(ctx, req.Method, req.Path, query, header, body)
	if err != nil {
		return Response{}, err
	}
	return Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: resp.Body}, nil
}

// pagedBody is the minimal envelope shape needed to follow cursor pagination.
type pagedBody struct {
	Data   json.RawMessage `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}

// paginate walks every page of a list endpoint, following paging.next, and
// returns a single {"data":[…]} merging each page's items. If the first
// response is non-2xx or isn't a paged envelope with an array "data" field, it
// is returned unchanged — so --all is a safe no-op on non-list endpoints.
func (s *Service) paginate(ctx context.Context, path string, query url.Values, header http.Header) (Response, error) {
	items := []json.RawMessage{}
	var last *bitriseapi.RawResponse
	cursor := ""
	for {
		q := cloneValues(query)
		if cursor != "" {
			q.Set("next", cursor)
		}
		resp, err := s.client.RawRequest(ctx, http.MethodGet, path, q, header, nil)
		if err != nil {
			return Response{}, err
		}
		last = resp

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return asResponse(resp), nil
		}
		var pb pagedBody
		var pageItems []json.RawMessage
		if json.Unmarshal(resp.Body, &pb) != nil || len(pb.Data) == 0 || json.Unmarshal(pb.Data, &pageItems) != nil {
			// Not a paged list envelope. On the first page, pass it through
			// untouched; mid-pagination this shouldn't happen, so stop.
			if cursor == "" {
				return asResponse(resp), nil
			}
			break
		}
		items = append(items, pageItems...)
		if pb.Paging.Next == "" {
			break
		}
		cursor = pb.Paging.Next
	}

	merged, err := json.Marshal(struct {
		Data []json.RawMessage `json:"data"`
	}{Data: items})
	if err != nil {
		return Response{}, fmt.Errorf("merge pages: %w", err)
	}
	return Response{StatusCode: last.StatusCode, Header: last.Header, Body: merged}, nil
}

func asResponse(r *bitriseapi.RawResponse) Response {
	return Response{StatusCode: r.StatusCode, Header: r.Header, Body: r.Body}
}

// cloneHeader returns a non-nil copy of h.
func cloneHeader(h http.Header) http.Header {
	out := http.Header{}
	for k, vs := range h {
		for _, v := range vs {
			out.Add(k, v)
		}
	}
	return out
}

// cloneValues returns a non-nil copy of v.
func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		for _, val := range vs {
			out.Add(k, val)
		}
	}
	return out
}
