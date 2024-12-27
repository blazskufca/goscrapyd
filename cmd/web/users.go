package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/password"
	"github.com/blazskufca/goscrapyd/internal/request"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"github.com/google/uuid"
	"net/http"
)

type userAddEditForm struct {
	Username           string              `form:"username"`
	Password           string              `form:"password"`
	PasswordConfirm    string              `form:"password_confirm"`
	HasAdminPrivileges bool                `form:"grant_admin"`
	Validator          validator.Validator `form:"-"`
}

func (app *application) listsUsers(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancelFunc := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancelFunc()
	users, err := app.DB.queries.GetAllUsers(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	templateData := app.newTemplateData(r)
	templateData["Users"] = users
	app.render(w, r, http.StatusOK, usersListPage, nil, templateData)
}

func (app *application) addNewUser(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancelFunc := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancelFunc()
	var form userAddEditForm
	switch r.Method {
	case http.MethodGet:
		data := app.newTemplateData(r)
		data["Form"] = form
		app.render(w, r, http.StatusOK, addUserFormPage, nil, data)
	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}
		form.Validator.CheckField(form.Username != "", "Username", "Username is required")
		form.Validator.CheckField(len(form.Username) <= 20, "Username", "Username must not be more than 20 characters")
		form.Validator.CheckField(form.Password != "", "Password", "Password is required")
		form.Validator.CheckField(len(form.Password) >= 8, "Password", "Password is too short")
		form.Validator.CheckField(len(form.Password) <= 72, "Password", "Password is too long")
		form.Validator.CheckField(validator.NotIn(form.Password, password.CommonPasswords...), "Password", "Password is too common")
		form.Validator.CheckField(form.Password != "", "PasswordConfirm", "You need to confirm the selected password")
		form.Validator.CheckField(form.PasswordConfirm == form.Password, "PasswordConfirm", "Passwords do not match")

		if form.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = form
			app.render(w, r, http.StatusUnprocessableEntity, addUserFormPage, nil, data)
			return
		}
		hashedPassword, err := password.Hash(form.Password)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		userUUID, err := uuid.NewRandom()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		_, err = app.DB.queries.CreateNewUser(ctxwt, database.CreateNewUserParams{
			ID:                 userUUID,
			Username:           form.Username,
			HashedPassword:     hashedPassword,
			HasAdminPrivileges: form.HasAdminPrivileges,
		})
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		http.Redirect(w, r, "/list-users", http.StatusSeeOther)
	}
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancelFunc := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancelFunc()
	var form struct {
		Username  string              `form:"username"`
		Password  string              `form:"password"`
		Validator validator.Validator `form:"-"`
	}
	switch r.Method {
	case http.MethodGet:
		data := app.newTemplateData(r)
		data["Form"] = form
		app.render(w, r, http.StatusOK, loginPage, nil, data)
	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}
		var found bool
		user, userDbQueryErr := app.DB.queries.GetUserByUsername(ctxwt, form.Username)
		if userDbQueryErr != nil {
			if errors.Is(userDbQueryErr, sql.ErrNoRows) {
				found = false
			} else {
				app.reportServerError(r, err)
				return
			}
		} else {
			found = true
		}
		form.Validator.CheckField(form.Username != "", "Username", "Username is required")
		form.Validator.CheckField(found, "Username", "User by that username could not be found")
		if found {
			passwordMatches, err := password.Matches(form.Password, user.HashedPassword)
			if err != nil {
				app.reportServerError(r, err)
				return
			}
			form.Validator.CheckField(form.Password != "", "Password", "Password is required")
			form.Validator.CheckField(passwordMatches, "Password", "Password is incorrect")
		}
		if form.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = form
			app.render(w, r, http.StatusUnprocessableEntity, loginPage, nil, data)
			return
		}

		session, err := app.sessionStore.Get(r, "session")
		if err != nil {
			app.reportServerError(r, err)
			return
		}

		session.Values["userID"] = user.ID

		redirectPath, ok := session.Values["redirectPathAfterLogin"].(string)
		if ok {
			delete(session.Values, "redirectPathAfterLogin")
		} else {
			redirectPath = "/list-nodes"
		}

		err = session.Save(r, w)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		http.Redirect(w, r, redirectPath, http.StatusSeeOther)
	}
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	session, err := app.sessionStore.Get(r, "session")
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	delete(session.Values, "userID")

	err = session.Save(r, w)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *application) deleteUser(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancelFunc := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancelFunc()
	parsedUUID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		app.reportServerError(r, err)
		return
	}
	err = app.DB.queries.DeleteUserByUUID(ctxwt, parsedUUID)
	if err != nil {
		app.reportServerError(r, err)
		return
	}
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (app *application) updateUser(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancelFunc := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancelFunc()
	var FormData userAddEditForm
	userUUID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	user, err := app.DB.queries.GetUserWithID(ctxwt, userUUID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		FormData.Username = user.Username
		templateData := app.newTemplateData(r)
		templateData["Form"] = FormData
		templateData["IsAdmin"] = user.HasAdminPrivileges
		templateData["ID"] = user.ID
		app.render(w, r, http.StatusOK, editUserPage, nil, templateData)
	case http.MethodPost:
		var userPassword string
		err = request.DecodePostForm(r, &FormData)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}
		FormData.Validator.CheckField(FormData.Username != "", "Username", "Username is required")
		FormData.Validator.CheckField(len(FormData.Username) <= 20, "Username", "Username must not be more than 20 characters")
		if FormData.Password != "" || FormData.PasswordConfirm != "" {
			FormData.Validator.CheckField(FormData.Password != "", "Password", "Password is required")
			FormData.Validator.CheckField(len(FormData.Password) >= 8, "Password", "Password is too short")
			FormData.Validator.CheckField(len(FormData.Password) <= 72, "Password", "Password is too long")
			FormData.Validator.CheckField(validator.NotIn(FormData.Password, password.CommonPasswords...), "Password", "Password is too common")
			FormData.Validator.CheckField(FormData.Password != "", "PasswordConfirm", "You need to confirm the selected password")
			FormData.Validator.CheckField(FormData.PasswordConfirm == FormData.Password, "PasswordConfirm", "Passwords do not match")
		}
		if FormData.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = FormData
			data["IsAdmin"] = FormData.HasAdminPrivileges
			data["ID"] = user.ID
			app.render(w, r, http.StatusUnprocessableEntity, editUserPage, nil, data)
			return
		}
		if FormData.Password != "" {
			userPassword, err = password.Hash(FormData.Password)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
		} else {
			userPassword = user.HashedPassword
		}
		err = app.DB.queries.UpdateUserWhereUUID(ctxwt, database.UpdateUserWhereUUIDParams{
			Username:           FormData.Username,
			HashedPassword:     userPassword,
			HasAdminPrivileges: FormData.HasAdminPrivileges,
			ID:                 user.ID,
		})
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		http.Redirect(w, r, "/list-users", http.StatusSeeOther)
	}
}
