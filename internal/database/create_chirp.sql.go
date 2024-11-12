// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: create_chirp.sql

package database

import (
	"context"
)

const createChirp = `-- name: CreateChirp :one
INSERT INTO chirps (id, user_id, body, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    $1,
    $2,
    NOW(),
    NOW()
)
RETURNING id, body, user_id, created_at, updated_at
`

type CreateChirpParams struct {
	UserID string
	Body   string
}

func (q *Queries) CreateChirp(ctx context.Context, arg CreateChirpParams) (Chirp, error) {
	row := q.db.QueryRowContext(ctx, createChirp, arg.UserID, arg.Body)
	var i Chirp
	err := row.Scan(
		&i.ID,
		&i.Body,
		&i.UserID,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}