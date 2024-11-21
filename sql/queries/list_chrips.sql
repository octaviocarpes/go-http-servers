-- name: ListChirps :many
SELECT *
FROM chirps
WHERE user_id = COALESCE(sqlc.narg('author_id'), user_id)
ORDER BY
    CASE WHEN $1 THEN created_at END ASC,
    CASE WHEN $1 = FALSE THEN created_at END DESC;
