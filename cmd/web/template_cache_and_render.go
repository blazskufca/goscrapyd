package main

import (
	"bytes"
	"fmt"
	"github.com/blazskufca/goscrapyd/assets"
	"github.com/blazskufca/goscrapyd/internal/funcs"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

func newTemplateCache() (map[templateName]*template.Template, error) {
	cache := map[templateName]*template.Template{}
	pages, err := fs.Glob(assets.EmbeddedFiles, "templates/*/*.tmpl")
	if err != nil {
		return nil, err
	}
	for _, page := range pages {
		name := templateName(filepath.Base(page))
		patterns := []string{
			"templates/base.tmpl",
			"templates/partials/*.tmpl",
			"templates/htmx/*.tmpl",
			page,
		}
		ts, err := template.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, patterns...)
		if err != nil {
			return nil, err
		}
		cache[name] = ts
	}
	return cache, nil
}

func (app *application) render(w http.ResponseWriter, r *http.Request, status int, page templateName, headers http.Header, data any) {
	ts, ok := app.templateCache[page]
	if !ok {
		err := fmt.Errorf("the template %s does not exist", page)
		app.serverError(w, r, err)
		return
	}
	buffer := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buffer, "base", data)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	for key, value := range headers {
		w.Header()[key] = value
	}
	w.WriteHeader(status)
	_, err = buffer.WriteTo(w)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) renderHTMX(w http.ResponseWriter, r *http.Request, status int, page templateName, headers http.Header, templateName string, data any) {
	ts, ok := app.templateCache[page]
	if !ok {
		err := fmt.Errorf("the template %s does not exist", page)
		app.serverError(w, r, err)
		return
	}
	buffer := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buffer, templateName, data)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	for key, value := range headers {
		w.Header()[key] = value
	}
	w.WriteHeader(status)
	_, err = buffer.WriteTo(w)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) writeSSEResponse(w http.ResponseWriter, r *http.Request, flusher http.Flusher, data any, page templateName, event, templateName string) {
	defer flusher.Flush()
	ts, ok := app.templateCache[page]
	if !ok {
		err := fmt.Errorf("the template %s does not exist", page)
		app.sseError(r, w, event, err)
		return
	}
	buffer := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buffer, templateName, data)
	if err != nil {
		app.sseError(r, w, event, err)
		return
	}

	rawData := buffer.String()
	cleanedData := strings.Join(strings.Fields(rawData), " ")
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, cleanedData)
	if err != nil {
		app.sseError(r, w, event, err)
	}
}

// Helper function to handle error reporting and writing SSE response
func (app *application) sseError(r *http.Request, w http.ResponseWriter, event string, err error) {
	app.reportServerError(r, err)
	_, writeErr := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, err)
	if writeErr != nil {
		app.reportServerError(r, writeErr)
	}
}
