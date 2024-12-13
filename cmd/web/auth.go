package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid/v5"
	"github.com/micahco/web/internal/models"
)

type contextKey string

const (
	authenticatedUserIDSessionKey = "authenticatedUserID"
	verificationEmailSessionKey   = "verificationEmail"
	verificationTokenSessionKey   = "verificationToken"
	resetEmailSessionKey          = "resetEmail"
	resetTokenSessionKey          = "resetToken"
	isAuthenticatedContextKey     = contextKey("isAuthenticated")
)

func (app *application) login(r *http.Request, userID uuid.UUID) error {
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

func (app *application) getSessionUserID(r *http.Request) (uuid.UUID, error) {
	id, ok := app.sessionManager.Get(r.Context(), authenticatedUserIDSessionKey).(uuid.UUID)
	if !ok {
		return uuid.UUID{}, fmt.Errorf("unable to parse session id as int")
	}

	return id, nil
}
func (app *application) handleAuthLoginPost(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return app.renderError(w, r, http.StatusBadRequest, errors.New("already authenticated"))
	}

	var form struct {
		Email    string `form:"email" validate:"required,email"`
		Password string `form:"password" validate:"required"`
	}

	err := app.parseForm(r, &form)
	if err != nil {
		return err
	}

	user, err := app.models.User.GetForCredentials(form.Email, form.Password)
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
		Email string `form:"email" validate:"required,email"`
	}

	err := app.parseForm(r, &form)
	if err != nil {
		return err
	}

	// Consistent flash message
	f := FlashMessage{
		Type:    FlashInfo,
		Message: "A link to activate your account has been sent to the email address provided. Please check your junk folder.",
	}

	// Check if user with email already exists
	exists, err := app.models.User.ExistsWithEmail(form.Email)
	if err != nil {
		return err
	}

	// If user does exist, do nothing but send flash message
	if exists {
		app.putFlash(r, f)
		app.refresh(w, r)

		return nil
	}

	// Check if link verification has already been created
	v, err := app.models.Verification.Get(form.Email)
	if err != nil && err != models.ErrNoRecord {
		return err
	}

	// Don't send a new link if less than 5 minutes since last
	if v != nil {
		if time.Since(v.CreatedAt) < 5*time.Minute {
			app.putFlash(r, f)
			app.refresh(w, r)

			return nil
		}
	}

	token, err := app.models.Verification.New(form.Email)
	if err != nil {
		return fmt.Errorf("signup create token: %w", err)
	}

	// Create link with token
	ref, err := url.Parse("/auth/register")
	if err != nil {
		return err
	}
	q := ref.Query()
	q.Set("token", token)
	ref.RawQuery = q.Encode()
	link := app.baseURL.ResolveReference(ref)

	// Send mail in background routine
	if !app.config.dev {
		app.background(func() {
			err = app.mailer.Send(form.Email, "email-verification.tmpl", link)
			if err != nil {
				app.logger.Error("mailer", slog.Any("err", err))
			}
		})
	}
	app.logger.Debug("mailed", slog.String("link", link.String()))

	// Clear all session data and add form email to session. That way,
	// when the user goes to register, won't have to re-enter email.
	app.sessionManager.Clear(r.Context())
	app.sessionManager.RenewToken(r.Context())
	app.sessionManager.Put(r.Context(), verificationEmailSessionKey, form.Email)

	app.putFlash(r, f)
	app.refresh(w, r)

	return nil
}

func (app *application) handleAuthRegisterGet(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return app.renderError(w, r, http.StatusBadRequest, errors.New("already authenticated"))
	}

	queryToken := r.URL.Query().Get("token")
	if queryToken == "" {
		return app.renderError(w, r, http.StatusBadRequest, errors.New("missing verification token"))
	}

	app.sessionManager.Put(r.Context(), verificationTokenSessionKey, queryToken)

	var data struct {
		HasSessionEmail bool
	}

	// If session email exists, don't show email input in form.
	data.HasSessionEmail = app.sessionManager.Exists(r.Context(), verificationEmailSessionKey)

	return app.render(w, r, http.StatusOK, "auth-register.tmpl", data)
}

var ExpiredTokenFlash = FlashMessage{
	Type:    FlashError,
	Message: "Expired verification token.",
}

func (app *application) handleAuthRegisterPost(w http.ResponseWriter, r *http.Request) error {
	if app.isAuthenticated(r) {
		return app.renderError(w, r, http.StatusBadRequest, errors.New("already authenticated"))
	}

	var form struct {
		Email    string `form:"email" validate:"required,email,max=254"`
		Password string `form:"password" validate:"required,min=8,max=72"`
	}

	form.Email = app.sessionManager.GetString(r.Context(), verificationEmailSessionKey)
	err := app.parseForm(r, &form)
	if err != nil {
		return err
	}

	token := app.sessionManager.GetString(r.Context(), verificationTokenSessionKey)
	if token == "" {
		return app.renderError(w, r, http.StatusUnauthorized, nil)
	}

	err = app.models.Verification.Verify(token, form.Email)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			return app.renderError(w, r, http.StatusUnauthorized, nil)
		}
		if errors.Is(err, models.ErrExpiredVerification) {
			app.putFlash(r, ExpiredTokenFlash)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return nil
		}

		return err
	}

	// Upon registration, purge db of all verifications with email.
	err = app.models.Verification.Purge(form.Email)
	if err != nil {
		return err
	}

	user, err := app.models.User.New(form.Email, form.Password)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateEmail) {
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

func (app *application) handleAuthResetGet(w http.ResponseWriter, r *http.Request) error {
	return app.render(w, r, http.StatusOK, "auth-reset.tmpl", nil)
}

func (app *application) handleAuthResetPost(w http.ResponseWriter, r *http.Request) error {
	var email string

	// Get users email if already authenticated.
	if app.isAuthenticated(r) {
		suid, err := app.getSessionUserID(r)
		if err != nil {
			return err
		}

		user, err := app.models.User.GetWithID(suid)
		if err != nil {
			return err
		}

		email = user.Email
	} else {
		// If not authenticated, parse form and validate email address
		var form struct {
			Email string `form:"email" validate:"required,email"`
		}

		err := app.parseForm(r, &form)
		if err != nil {
			var validationErrors validator.ValidationErrors
			switch {
			case errors.As(err, &validationErrors):
				return app.renderError(w, r, http.StatusUnprocessableEntity, validationErrors)
			default:
				return err
			}
		}

		email = form.Email
	}

	exists, err := app.models.User.ExistsWithEmail(email)
	if err != nil {
		return err
	}

	f := FlashMessage{
		Type:    FlashInfo,
		Message: "A link to reset your password has been sent to the email address provided. Please check your junk folder.",
	}

	// If user does not exist, respond with consistent flash message
	if !exists {
		app.putFlash(r, f)
		app.refresh(w, r)

		return nil
	}

	// Check if link verification has already been created
	v, err := app.models.Verification.Get(email)
	if err != nil && err != models.ErrNoRecord {
		return err
	}

	// Don't send a new link if less than 5 minutes since last
	if v != nil {
		if time.Since(v.CreatedAt) < 5*time.Minute {
			app.putFlash(r, f)
			app.refresh(w, r)

			return nil
		}
	}

	token, err := app.models.Verification.New(email)
	if err != nil {
		return err
	}

	// Create link with token
	ref, err := url.Parse("/auth/reset/update")
	if err != nil {
		return err
	}
	q := ref.Query()
	q.Set("token", token)
	ref.RawQuery = q.Encode()
	link := app.baseURL.ResolveReference(ref)

	// Send mail in background routine
	if !app.config.dev {
		app.background(func() {
			err = app.mailer.Send(email, "email-verification.tmpl", link)
			if err != nil {
				app.logger.Error("mailer", slog.Any("err", err))
			}
		})
	}
	app.logger.Debug("mailed", slog.String("link", link.String()))

	app.sessionManager.RenewToken(r.Context())
	app.sessionManager.Put(r.Context(), resetEmailSessionKey, email)

	app.putFlash(r, f)
	app.refresh(w, r)

	return nil
}

func (app *application) handleAuthResetUpdateGet(w http.ResponseWriter, r *http.Request) error {
	queryToken := r.URL.Query().Get("token")
	if queryToken == "" {
		return app.renderError(w, r, http.StatusUnauthorized, nil)
	}

	app.sessionManager.Put(r.Context(), resetTokenSessionKey, queryToken)

	var data struct {
		HasSessionEmail bool
	}
	data.HasSessionEmail = app.sessionManager.Exists(r.Context(), resetEmailSessionKey)

	return app.render(w, r, http.StatusOK, "auth-reset-update.tmpl", data)
}

func (app *application) handleAuthResetUpdatePost(w http.ResponseWriter, r *http.Request) error {
	var form struct {
		Email    string `form:"email" validate:"required,email,max=254"`
		Password string `form:"password" validate:"required,min=8,max=72"`
	}

	form.Email = app.sessionManager.GetString(r.Context(), resetEmailSessionKey)
	err := app.parseForm(r, &form)
	if err != nil {
		return err
	}

	token := app.sessionManager.GetString(r.Context(), resetTokenSessionKey)
	if token == "" {
		return app.renderError(w, r, http.StatusUnauthorized, nil)
	}

	err = app.models.Verification.Verify(token, form.Email)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			return app.renderError(w, r, http.StatusUnauthorized, nil)
		}
		if errors.Is(err, models.ErrExpiredVerification) {
			app.putFlash(r, ExpiredTokenFlash)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return nil
		}

		return err
	}

	user, err := app.models.User.GetWithEmail(form.Email)
	if err != nil {
		return err
	}

	err = user.SetPasswordHash(form.Password)
	if err != nil {
		return err
	}

	err = app.models.User.Update(user)
	if err != nil {
		return err
	}

	err = app.models.Verification.Purge(form.Email)
	if err != nil {
		return err
	}

	app.sessionManager.Clear(r.Context())

	f := FlashMessage{
		Type:    FlashSuccess,
		Message: "Successfully updated password. Please login.",
	}
	app.putFlash(r, f)

	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}
