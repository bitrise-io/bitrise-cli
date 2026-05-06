package bitriseapi

// Paging holds cursor-based pagination metadata returned by list endpoints.
type Paging struct {
	TotalItemCount int    `json:"total_item_count"`
	PageItemLimit  int    `json:"page_item_limit"`
	Next           string `json:"next"` // empty when no further pages exist
}

// HasMore reports whether additional pages are available.
func (p Paging) HasMore() bool { return p.Next != "" }

// Page is the result of a single paginated list call.
type Page[T any] struct {
	Items  []T
	Paging Paging
}

// ListOptions controls pagination for list endpoints.
type ListOptions struct {
	// Limit is the maximum number of items per page. 0 uses the server default (50).
	Limit int
	// Cursor is the opaque token for the next page, taken from Page.Paging.Next.
	// Leave empty to start from the first page.
	Cursor string
}
