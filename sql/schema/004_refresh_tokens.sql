-- +goose Up
CREATE TABLE refresh_tokens(
    token TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    revoked_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    CONSTRAINT fk_user_refresh_token
    FOREIGN KEY (user_id)
    REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE refresh_tokens;
