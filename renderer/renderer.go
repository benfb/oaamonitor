package renderer

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
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

// RenderToString executes a template and returns the rendered HTML
func (r *Renderer) RenderToString(tmplName string, data interface{}) (string, error) {
	buf := new(strings.Builder)
	err := r.templates.ExecuteTemplate(buf, tmplName, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
