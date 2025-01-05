package main

import (
	"context"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/request"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"net/http"
	"net/url"
)

func (app *application) settingPage(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancelFunc := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancelFunc()
	settingsExists, err := app.DB.queries.CheckSettingsExist(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		templateData := app.newTemplateData(r)
		if settingsExists == 1 {
			settings, err := app.DB.queries.GetSettings(ctxwt)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			if settings.PersistedSpiderSettings.Valid {
				persistedSettings, err := url.ParseQuery(settings.PersistedSpiderSettings.String)
				if err != nil {
					app.serverError(w, r, err)
					return
				}
				templateData["Settings"] = cleanUrlValues(persistedSettings, "spider", "project", "version", "csrf_token")
			}
			if settings.DefaultProjectPath.Valid {
				templateData["ProjectPath"] = settings.DefaultProjectPath.String
			}
			if settings.DefaultProjectName.Valid {
				templateData["ProjectName"] = settings.DefaultProjectName.String
			}
		}
		app.render(w, r, http.StatusOK, settingsPage, nil, templateData)
	case http.MethodPost:
		formData := struct {
			ProjectPath string              `form:"project_path"`
			ProjectName string              `form:"project_name"`
			Validator   validator.Validator `form:"-"`
		}{}
		err := request.DecodePostForm(r, &formData)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		var cleanPath string
		if formData.ProjectPath != "" {
			cleanPath, err = sanitizePath(formData.ProjectPath)
			if err != nil {
				formData.Validator.AddFieldError("ProjectPath", err.Error())
			}
		}
		if formData.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = formData
			data["ProjectName"] = formData.ProjectName
			app.render(w, r, http.StatusUnprocessableEntity, settingsPage, nil, data)
			return
		}
		form := cleanUrlValues(r.Form, "project_path", "project_name", "csrf_token")
		formString := form.Encode()
		if settingsExists == 0 {
			_, err := app.DB.queries.InsertSettings(ctxwt, database.InsertSettingsParams{
				DefaultProjectPath:      database.CreateSqlNullString(&cleanPath),
				DefaultProjectName:      database.CreateSqlNullString(&formData.ProjectName),
				PersistedSpiderSettings: database.CreateSqlNullString(&formString),
			})
			if err != nil {
				app.serverError(w, r, err)
				return
			}
		} else if settingsExists == 1 {
			err = app.DB.queries.UpdateSettings(ctxwt, database.UpdateSettingsParams{
				DefaultProjectPath:      database.CreateSqlNullString(&cleanPath),
				DefaultProjectName:      database.CreateSqlNullString(&formData.ProjectName),
				PersistedSpiderSettings: database.CreateSqlNullString(&formString),
			})
			if err != nil {
				app.serverError(w, r, err)
				return
			}
		}
		http.Redirect(w, r, "/edit-settings", http.StatusFound)
	}
}
