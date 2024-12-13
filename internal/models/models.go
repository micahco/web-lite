package models

import (
	"database/sql"
	"errors"
	"time"
)

const ctxTimeout = 3 * time.Second

type Models struct {
	User *UserModel
}

func New(db *sql.DB) Models {
	return Models{
		User: &UserModel{db},
	}
}

var (
	ErrNoRecord           = errors.New("models: no matching record found")
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	ErrDuplicateUsername  = errors.New("models: duplicate username")
	ErrEditConflict       = errors.New("models: edit conflict")
)
