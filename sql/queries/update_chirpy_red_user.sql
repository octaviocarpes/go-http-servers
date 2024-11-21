-- name: UpdateChirpyRedUser :one
UPDATE users SET is_chirpy_red = $1
WHERE id = $2
RETURNING *;
