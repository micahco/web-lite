package main

import (
	"errors"
	"net/http"

	"github.com/micahco/web-lite/internal/models"
)

type contextKey string

const (
	authenticatedUserIDSessionKey = "authenticatedUserID"
	isAuthenticatedContextKey     = contextKey("isAuthenticated")
)

func (app *application) login(r *http.Request, userID int) error {
	err := app.sessionManager.RenewToken(r.Context())
	if err != nil {
		return err
	}

	app.sessionManager.Put(r.Context(), authenticatedUserIDSessionKey, userID)

	return nil
}

func (app *application) logout(r *http.Request) error {
	err := app.sessionManager.RenewToken(r.Context())
	if err != nil {
		return err
	}

	app.sessionManager.Remove(r.Context(), authenticatedUserIDSessionKey)

	return nil
}

// Check the auth context set by the authenticate middleware
func (app *application) isAuthenticated(r *http.Request) bool {
	isAuthenticated, ok := r.Context().Value(isAuthenticatedContextKey).(bool)
	if !ok {
		return false
	}

	return isAuthenticated
}

func (app *application) getSessionUserID(r *http.Request) (int, error) {
	id := app.sessionManager.GetInt(r.Context(), authenticatedUserIDSessionKey)

	return id, nil
}

func (app *application) handleAuthLoginGet(w http.ResponseWriter, r *http.Request) error {
	return app.render(w, r, http.StatusOK, "login.tmpl", nil)
}

func (app *application) handleAuthLoginPost(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return app.renderError(w, r, http.StatusBadRequest, errors.New("already authenticated"))
	}

	var form struct {
		Username string `form:"username" validate:"required"`
		Password string `form:"password" validate:"required"`
	}

	err := app.parseForm(r, &form)
	if err != nil {
		return err
	}

	user, err := app.models.User.GetForCredentials(form.Username, form.Password)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrInvalidCredentials):
			return app.renderError(w, r, http.StatusUnauthorized, nil)
		default:
			return err
		}
	}

	err = app.login(r, user.ID)
	if err != nil {
		return err
	}

	// Redirect to homepage after authenticating the user.
	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}

func (app *application) handleAuthLogoutPost(w http.ResponseWriter, r *http.Request) error {
	err := app.logout(r)
	if err != nil {
		return err
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}

func (app *application) handleAuthSignupPost(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return app.renderError(w, r, http.StatusBadRequest, errors.New("already authenticated"))
	}

	var form struct {
		Username string `form:"username" validate:"required,max=254"`
		Password string `form:"password" validate:"required,min=8,max=72"`
	}

	err := app.parseForm(r, &form)
	if err != nil {
		return err
	}

	user, err := app.models.User.New(form.Username, form.Password)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateUsername) {
			return app.renderError(w, r, http.StatusUnauthorized, nil)
		}

		return err
	}

	// Login user
	app.sessionManager.Clear(r.Context())
	err = app.login(r, user.ID)
	if err != nil {
		return err
	}

	f := FlashMessage{
		Type:    FlashSuccess,
		Message: "Successfully created account. Welcome!",
	}
	app.putFlash(r, f)
	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}
