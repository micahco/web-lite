package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ttl = time.Hour
)

type VerificationModel struct {
	pool *pgxpool.Pool
}

type Verification struct {
	Hash      []byte
	Email     string
	Expiry    time.Time
	CreatedAt time.Time
}

func (v *Verification) IsExpired() bool {
	return time.Now().After(v.Expiry)
}

func scanVerification(row pgx.CollectableRow) (*Verification, error) {
	var v Verification
	err := row.Scan(
		&v.Hash,
		&v.Email,
		&v.Expiry,
		&v.CreatedAt)

	return &v, err
}

// Create new verification token. Store hash in database and return token.
func (m *VerificationModel) New(email string) (string, error) {
	expiry := time.Now().Add(ttl)

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	token := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	// Note: sum is a byte array, while hash is a slice
	sum := sha256.Sum256([]byte(token))
	hash := sum[:]

	sql := `INSERT INTO verification_
		(hash_, email_, expiry_)
		VALUES($1, $2, $3);`

	_, err = m.pool.Exec(context.Background(), sql, hash, email, expiry)

	return token, err
}

func (m *VerificationModel) Get(email string) (*Verification, error) {
	sql := "SELECT * FROM verification_ WHERE email_ = $1;"

	rows, err := m.pool.Query(context.Background(), sql, email)
	if err != nil {
		return nil, err
	}

	v, err := pgx.CollectOneRow(rows, scanVerification)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoRecord
	}

	return v, err
}

func (m *VerificationModel) Verify(token, email string) error {
	sum := sha256.Sum256([]byte(token))
	hash := sum[:]

	sql := `SELECT * FROM verification_ 
		WHERE hash_ = $1 AND email_ = $2;`

	rows, err := m.pool.Query(context.Background(), sql, hash, email)
	if err != nil {
		return err
	}

	v, err := pgx.CollectOneRow(rows, scanVerification)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRecord
	}

	if v.IsExpired() {
		return ErrExpiredVerification
	}

	return nil
}

func (m *VerificationModel) Purge(email string) error {
	sql := "DELETE FROM verification_ WHERE email_ = $1;"

	_, err := m.pool.Exec(context.Background(), sql, email)

	return err
}
