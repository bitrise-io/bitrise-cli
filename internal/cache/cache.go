// Package cache provides a lightweight in-memory name→ID mapping cache
// for CLI lookups. Entries live for the duration of a single process invocation.
package cache

import "strings"

// Cache is an in-memory name→ID mapping.
// All lookups and mutations are case-insensitive (keys are stored lowercase).
type Cache struct {
	apps       map[string]string
	workspaces map[string]string
}

// New returns an empty, ready-to-use Cache.
func New() *Cache {
	return &Cache{
		apps:       make(map[string]string),
		workspaces: make(map[string]string),
	}
}

// LookupApp returns the app slug stored for the given title, if any.
func (c *Cache) LookupApp(title string) (string, bool) {
	if c == nil {
		return "", false
	}
	slug, ok := c.apps[strings.ToLower(title)]
	return slug, ok
}

// LookupWorkspace returns the workspace slug stored for the given name, if any.
func (c *Cache) LookupWorkspace(name string) (string, bool) {
	if c == nil {
		return "", false
	}
	slug, ok := c.workspaces[strings.ToLower(name)]
	return slug, ok
}

// SetApp stores a title→slug mapping.
func (c *Cache) SetApp(title, slug string) {
	if c == nil {
		return
	}
	c.apps[strings.ToLower(title)] = slug
}

// SetWorkspace stores a name→slug mapping.
func (c *Cache) SetWorkspace(name, slug string) {
	if c == nil {
		return
	}
	c.workspaces[strings.ToLower(name)] = slug
}
