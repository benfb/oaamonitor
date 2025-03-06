package renderer

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

// Renderer handles template rendering
type Renderer struct {
	templates *template.Template
}

// New creates a new Renderer instance
func New() (*Renderer, error) {
	templates, err := filepath.Glob("templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to find templates: %v", err)
	}

	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %v", err)
	}

	return &Renderer{
		templates: tmpl,
	}, nil
}

// Render renders a template with the given data
func (r *Renderer) Render(w http.ResponseWriter, tmplName string, data interface{}) {
	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute template
	err := r.templates.ExecuteTemplate(w, tmplName, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", tmplName, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// ReloadTemplates reloads all templates
func (r *Renderer) ReloadTemplates() error {
	templates, err := filepath.Glob("templates/*.html")
	if err != nil {
		return fmt.Errorf("failed to find templates: %v", err)
	}

	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %v", err)
	}

	r.templates = tmpl
	return nil
}
