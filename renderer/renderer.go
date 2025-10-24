package renderer

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
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

	funcMap := template.FuncMap{
		"toJSON": func(v any) template.JS {
			data, err := json.Marshal(v)
			if err != nil {
				log.Printf("failed to marshal JSON in template: %v", err)
				return template.JS("null")
			}
			return template.JS(data)
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFiles(templates...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %v", err)
	}

	return &Renderer{
		templates: tmpl,
	}, nil
}

// Render renders a template with the given data
func (r *Renderer) Render(w http.ResponseWriter, tmplName string, data interface{}) {
	output, err := r.RenderToString(tmplName, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", tmplName, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set content type and write the buffered output
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(output))
}

// RenderToString executes a template and returns the rendered HTML
func (r *Renderer) RenderToString(tmplName string, data interface{}) (string, error) {
	buf := new(strings.Builder)
	err := r.templates.ExecuteTemplate(buf, tmplName, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ReloadTemplates reloads all templates
func (r *Renderer) ReloadTemplates() error {
	templates, err := filepath.Glob("templates/*.html")
	if err != nil {
		return fmt.Errorf("failed to find templates: %v", err)
	}

	funcMap := template.FuncMap{
		"toJSON": func(v any) template.JS {
			data, err := json.Marshal(v)
			if err != nil {
				log.Printf("failed to marshal JSON in template reload: %v", err)
				return template.JS("null")
			}
			return template.JS(data)
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFiles(templates...)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %v", err)
	}

	r.templates = tmpl
	return nil
}
