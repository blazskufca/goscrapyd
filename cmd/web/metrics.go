package main

import (
	"net/http"
	"net/http/pprof"
	"time"
)

func (app *application) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Just serve out the static HTML template data is parsed dynamically from json
	templateData := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, metricsPage, nil, templateData)
}

func (app *application) pprofHandler(w http.ResponseWriter, r *http.Request) {
	// Disable the read/write timout here for longer profiles
	rc := http.NewResponseController(w)
	err := rc.SetReadDeadline(time.Time{})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	err = rc.SetWriteDeadline(time.Time{})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	pprof.Index(w, r)
}
