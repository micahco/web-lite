package main

import "net/http"

func (app *application) handleDashboardGet(w http.ResponseWriter, r *http.Request) error {
	suid, err := app.getSessionUserID(r)
	if err != nil {
		return err
	}

	u, err := app.models.User.GetWithID(suid)
	if err != nil {
		return err
	}

	var data struct {
		Username string
	}

	data.Username = u.Username

	return app.render(w, r, http.StatusOK, "dashboard.tmpl", data)
}
