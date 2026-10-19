package renderer

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"strings"
)

// Renderer handles template rendering
type Renderer struct {
	templates *template.Template
}

// New creates a Renderer from the *.html templates at the root of fsys.
func New(fsys fs.FS) (*Renderer, error) {

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

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(fsys, "*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %v", err)
	}

	return &Renderer{
		templates: tmpl,
	}, nil
}

// RenderToString executes a template and returns the rendered HTML
func (r *Renderer) RenderToString(tmplName string, data any) (string, error) {
	buf := new(strings.Builder)
	err := r.templates.ExecuteTemplate(buf, tmplName, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
