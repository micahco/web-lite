package main

import "net/http"

type FlashMessageType string

const (
	FlashSuccess    = FlashMessageType("success")
	FlashInfo       = FlashMessageType("info")
	FlashError      = FlashMessageType("error")
	flashSessionKey = "flash"
)

type FlashMessage struct {
	Type    FlashMessageType
	Message string
}

func (app *application) putFlash(r *http.Request, f FlashMessage) {
	app.sessionManager.Put(r.Context(), flashSessionKey, f)
}

func (app *application) popFlash(r *http.Request) *FlashMessage {
	exists := app.sessionManager.Exists(r.Context(), flashSessionKey)
	if exists {
		f, ok := app.sessionManager.Pop(r.Context(), flashSessionKey).(FlashMessage)
		if ok {
			return &f
		}
	}

	return nil
}
