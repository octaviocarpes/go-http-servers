// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package database

import (
	"database/sql"
	"time"
)

type Chirp struct {
	ID        string
	Body      string
	UserID    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RefreshToken struct {
	Token     string
	UserID    string
	RevokedAt sql.NullTime
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Email          string
	HashedPassword string
	IsChirpyRed    sql.NullBool
}
