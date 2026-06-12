package cache

import "testing"

func TestNew_IsEmpty(t *testing.T) {
	c := New()
	if _, ok := c.LookupApp("anything"); ok {
		t.Error("fresh cache should be empty")
	}
}

func TestSetAndLookupApp(t *testing.T) {
	c := New()
	c.SetApp("My App", "abc12345")

	slug, ok := c.LookupApp("My App")
	if !ok || slug != "abc12345" {
		t.Errorf("expected abc12345, got %q ok=%v", slug, ok)
	}
	slug2, ok2 := c.LookupApp("my app")
	if !ok2 || slug2 != "abc12345" {
		t.Errorf("case-insensitive lookup failed: %q ok=%v", slug2, ok2)
	}
}

func TestSetAndLookupWorkspace(t *testing.T) {
	c := New()
	c.SetWorkspace("Acme Corp", "acme-corp")

	slug, ok := c.LookupWorkspace("Acme Corp")
	if !ok || slug != "acme-corp" {
		t.Errorf("expected acme-corp, got %q ok=%v", slug, ok)
	}
	slug2, ok2 := c.LookupWorkspace("ACME CORP")
	if !ok2 || slug2 != "acme-corp" {
		t.Errorf("case-insensitive lookup failed: %q ok=%v", slug2, ok2)
	}
}

func TestNilCacheIsNoop(t *testing.T) {
	var c *Cache
	c.SetApp("x", "y")
	c.SetWorkspace("x", "y")
	if _, ok := c.LookupApp("x"); ok {
		t.Error("nil cache LookupApp should return false")
	}
	if _, ok := c.LookupWorkspace("x"); ok {
		t.Error("nil cache LookupWorkspace should return false")
	}
}
