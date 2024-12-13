package models

import (
	"context"
	"errors"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserModel struct {
	pool *pgxpool.Pool
}

type User struct {
	ID           uuid.UUID
	CreatedAt    time.Time
	Email        string
	PasswordHash []byte
}

func (u User) Validate() error {
	return nil
}

func (u *User) SetPasswordHash(password string) error {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return err
	}

	u.PasswordHash = []byte(hash)

	return nil
}

func (m *UserModel) New(email, password string) (*User, error) {
	user := &User{Email: email}

	err := user.SetPasswordHash(password)
	if err != nil {
		return nil, err
	}

	err = m.Insert(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (m *UserModel) Insert(user *User) error {
	err := user.Validate()
	if err != nil {
		return err
	}

	sql := `
		INSERT INTO user_ (email_, password_hash_)
		VALUES($1, $2)
		RETURNING id_, created_at_;`

	args := []any{user.Email, user.PasswordHash}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err = m.pool.QueryRow(ctx, sql, args...).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		switch {
		case pgErrCode(err) == pgerrcode.UniqueViolation:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m *UserModel) GetWithID(id uuid.UUID) (*User, error) {
	var u User

	sql := `
		SELECT id_, created_at_, email_, password_hash_
		FROM user_ WHERE id_ = $1;`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, id).Scan(
		&u.ID,
		&u.CreatedAt,
		&u.Email,
		&u.PasswordHash,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrInvalidCredentials
		default:
			return nil, err
		}
	}

	return &u, nil
}

func (m *UserModel) GetWithEmail(email string) (*User, error) {
	var u User

	sql := `
		SELECT id_, created_at_, email_, password_hash_
		FROM user_ WHERE email_ = $1;`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, email).Scan(
		&u.ID,
		&u.CreatedAt,
		&u.Email,
		&u.PasswordHash,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrInvalidCredentials
		default:
			return nil, err
		}
	}

	return &u, nil
}

func (m *UserModel) GetForCredentials(email, password string) (*User, error) {
	u, err := m.GetWithEmail(email)
	if err != nil {
		return nil, err
	}

	match, err := argon2id.ComparePasswordAndHash(password, string(u.PasswordHash))
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, ErrInvalidCredentials
	}

	return u, nil
}

func (m *UserModel) Exists(id uuid.UUID) (bool, error) {
	var exists bool

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM user_
			WHERE id_ = $1
		);`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, id).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (m *UserModel) ExistsWithEmail(email string) (bool, error) {
	var exists bool

	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM user_
			WHERE email_ = $1
		);`

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	err := m.pool.QueryRow(ctx, sql, email).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (m UserModel) Update(user *User) error {
	err := user.Validate()
	if err != nil {
		return err
	}

	sql := `
		UPDATE user_ 
        SET email_ = $1, password_hash_ = $2
        WHERE id_ = $3;`

	args := []any{
		user.Email,
		user.PasswordHash,
		user.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	_, err = m.pool.Exec(ctx, sql, args...)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		case pgErrCode(err) == pgerrcode.UniqueViolation:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}
