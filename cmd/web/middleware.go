package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/justinas/nosurf"
)

func (app *application) recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")

				app.logger.Error("recovered from panic", slog.Any("err", err))

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; frame-ancestors 'self'; form-action 'self';")
		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-XSS-Protection", "0")

		next.ServeHTTP(w, r)
	})
}

func (app *application) noSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	csrfHandler.SetFailureHandler(app.csrfFailureHandler())

	return csrfHandler
}

func (app *application) csrfFailureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.logger.Error("csrf failure handler",
			slog.String("method", r.Method),
			slog.String("uri", r.URL.RequestURI()),
		)

		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	})
}

// Reads session authenticated user id key and checks if that user exists.
// If all systems check, then set authenticated context to the request.
func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := app.sessionManager.Get(r.Context(), authenticatedUserIDSessionKey).(uuid.UUID)
		if !ok {
			next.ServeHTTP(w, r)

			return
		}

		exists, err := app.models.User.Exists(id)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			app.logger.Error("middleware authenticate", slog.Any("err", err))

			return
		}

		if exists {
			ctx := context.WithValue(r.Context(), isAuthenticatedContextKey, true)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.isAuthenticated(r) {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		// Prevent pages that require authentication from being cached
		w.Header().Add("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}
