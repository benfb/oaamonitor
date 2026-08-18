package renderer

import (
	"strings"
	"testing"
)

func TestNewLoadsAndRendersTemplates(t *testing.T) {
	t.Chdir("..")

	r, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	html, err := r.RenderToString("header", struct {
		Title string
		Teams []any
	}{
		Title: "Template Test",
	})
	if err != nil {
		t.Fatalf("RenderToString returned error: %v", err)
	}
	if !strings.Contains(html, "<title>Template Test</title>") {
		t.Fatalf("rendered header did not contain title: %s", html)
	}
}
