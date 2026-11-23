package domain

import "testing"

func TestOrderedMaterialVisibilityAndIDs(t *testing.T) {
	c := Course{Title: "Introduction", Status: "published", Modules: []Module{{Title: "First", Status: "published", Materials: []Material{{Title: "Public", Status: "published", Kind: "link", Content: "https://example.org/"}, {Title: "Draft", Status: "draft", Kind: "text", Content: "Review"}}}, {Title: "Hidden", Status: "archived"}}}
	if err := c.Prepare(Course{}); err != nil {
		t.Fatal(err)
	}
	vis := c.Published()
	if len(vis.Modules) != 1 || len(vis.Modules[0].Materials) != 1 {
		t.Fatal(vis)
	}
	if _, ok := c.FindPublished(c.Modules[0].Materials[1].ID); ok {
		t.Fatal("draft visible")
	}
	c.Modules[0].Materials[0].Content = "file:///tmp/file"
	if c.Prepare(c) == nil {
		t.Fatal("unsafe link accepted")
	}
}
func TestRejectForeignAndDuplicateIDs(t *testing.T) {
	m := Module{ID: "foreign", Title: "Imported", Status: "published"}
	c := Course{Title: "Course", Status: "published", Modules: []Module{m}}
	if c.Prepare(Course{}) == nil {
		t.Fatal("foreign id accepted")
	}
	c.Modules = append(c.Modules, m)
	if c.Prepare(c) == nil {
		t.Fatal("duplicate")
	}
}
