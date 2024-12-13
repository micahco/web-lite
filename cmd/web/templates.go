package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"time"

	"github.com/justinas/nosurf"
	"github.com/micahco/web/ui"
)

type templateData struct {
	CSRFToken       string
	CurrentYear     int
	Flash           *FlashMessage
	FormErrors      FormErrors
	IsAuthenticated bool
	Data            any
}

// Render page template with data
func (app *application) render(w http.ResponseWriter, r *http.Request, statusCode int, page string, data any) error {
	td := templateData{
		CurrentYear:     time.Now().Year(),
		Flash:           app.popFlash(r),
		FormErrors:      app.popFormErrors(r),
		IsAuthenticated: app.isAuthenticated(r),
		CSRFToken:       nosurf.Token(r),
		Data:            data,
	}

	// In production, use template cache
	if !app.config.dev {
		return app.renderFromCache(w, statusCode, page, td)
	}

	// In development, parse files locally
	t, err := template.ParseFiles("./ui/web/base.tmpl")
	if err != nil {
		return err
	}

	t, err = t.Funcs(functions).ParseGlob("./ui/web/partials/*.tmpl")
	if err != nil {
		return err
	}

	t, err = t.ParseFiles("./ui/web/pages/" + page)
	if err != nil {
		return err
	}

	return writeTemplate(t, td, w, statusCode)
}

func (app *application) renderError(w http.ResponseWriter, r *http.Request, statusCode int, err error) error {
	http.Error(w, http.StatusText(statusCode), statusCode)

	return nil
}

func (app *application) renderFromCache(w http.ResponseWriter, statusCode int, page string, td templateData) error {
	t, ok := app.templateCache[page]
	if !ok {
		return fmt.Errorf("template %s does not exist", page)
	}

	return writeTemplate(t, td, w, statusCode)
}

func writeTemplate(t *template.Template, td templateData, w http.ResponseWriter, statusCode int) error {
	buf := new(bytes.Buffer)

	err := t.ExecuteTemplate(buf, "base", td)
	if err != nil {
		return err
	}

	w.WriteHeader(statusCode)

	if _, err := buf.WriteTo(w); err != nil {
		return err
	}

	return nil
}

var functions = template.FuncMap{}

// Create new template cache with ui.Files embedded file system.
// Creates a template for each page in the web/pages directory
// nested with web/base.tmpl and web/partials.
func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}
	fsys := ui.Files

	// Get list of pages
	pages, err := fs.Glob(fsys, "web/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	// Create a new template for each page and add to cache map.
	for _, page := range pages {
		name := filepath.Base(page)

		// Nest page with base template and partials
		patterns := []string{
			"web/base.tmpl",
			"web/partials/*.tmpl",
			page,
		}

		tmpl, err := template.New(name).Funcs(functions).ParseFS(fsys, patterns...)
		if err != nil {
			return nil, err
		}

		cache[name] = tmpl
	}

	return cache, nil
}
