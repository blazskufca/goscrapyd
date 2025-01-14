package main

import (
	"net/http"
)

func (app *application) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Just serve out the static HTML template data is parsed dynamically from json
	templateData := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, metricsPage, nil, templateData)
}
