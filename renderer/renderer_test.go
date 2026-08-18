package renderer

import (
	"os"
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

func TestExternalScriptsUseSubresourceIntegrity(t *testing.T) {
	t.Chdir("..")

	header, err := os.ReadFile("templates/header.html")
	if err != nil {
		t.Fatalf("read header template: %v", err)
	}

	for _, src := range []string{
		"https://cdn.jsdelivr.net/npm/chart.js@4.5.1/dist/chart.umd.js",
		"https://cdn.jsdelivr.net/npm/date-fns@3.6.0/cdn.min.js",
		"https://cdn.jsdelivr.net/npm/chartjs-adapter-date-fns@3.0.0/dist/chartjs-adapter-date-fns.bundle.min.js",
		"https://cdn.jsdelivr.net/npm/chartjs-plugin-datalabels@2.2.0",
	} {
		start := strings.Index(string(header), src)
		if start == -1 {
			t.Fatalf("missing external script %q", src)
		}
		scriptTag := string(header[start:])
		if end := strings.Index(scriptTag, "></script>"); end != -1 {
			scriptTag = scriptTag[:end]
		}
		if !strings.Contains(scriptTag, "integrity=\"") {
			t.Fatalf("external script %q is missing an integrity hash", src)
		}
	}
}
