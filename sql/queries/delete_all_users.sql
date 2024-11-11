-- name: DeleteAllUsers :exec
DELETE FROM users WHERE id IN (SELECT id FROM users)
RETURNING *;
