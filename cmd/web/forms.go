package main

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/go-playground/form/v4"
	"github.com/go-playground/validator/v10"
)

const formErrorsSessionKey = "form-errors"

type FormErrors map[string]string

func (formErrors FormErrors) Error() string {
	buff := bytes.NewBufferString("")

	for name, msg := range formErrors {
		buff.WriteString(name + ": " + msg)
		buff.WriteString("\n")
	}

	return strings.TrimSpace(buff.String())
}

func (app *application) putFormErrors(r *http.Request, formErrors FormErrors) {
	app.sessionManager.Put(r.Context(), formErrorsSessionKey, formErrors)
}

func (app *application) popFormErrors(r *http.Request) FormErrors {
	exists := app.sessionManager.Exists(r.Context(), formErrorsSessionKey)
	if exists {
		formErrors, ok := app.sessionManager.Pop(r.Context(), formErrorsSessionKey).(FormErrors)
		if ok {
			return formErrors
		}
	}

	return FormErrors{}
}

func (app *application) parseForm(r *http.Request, dst any) error {
	err := r.ParseForm()
	if err != nil {
		return err
	}

	err = app.formDecoder.Decode(dst, r.Form)
	if err != nil {
		var invalidDecoderError *form.InvalidDecoderError
		switch {
		case errors.As(err, &invalidDecoderError):
			panic(err)
		default:
			return err
		}
	}

	err = app.validate.Struct(dst)
	if err != nil {
		var validationErrors validator.ValidationErrors
		switch {
		case errors.As(err, &validationErrors):
			formErrors := make(FormErrors)
			for _, fieldErr := range validationErrors {
				tag := fieldErr.Tag()
				param := fieldErr.Param()

				var msg string
				switch tag {
				case "email":
					msg = "invalid email"
				case "min":
					msg = "minimum length: " + param
				case "max":
					msg = "maximum length: " + param
				default:
					msg = tag
					if param != "" {
						msg += ": " + param
					}
				}

				name := fieldErr.StructField()
				formErrors[name] = msg
			}
			return formErrors
		default:
			return err
		}
	}

	return nil
}
