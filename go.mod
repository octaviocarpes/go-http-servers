module github.com/octaviocarpes/go-http-servers

go 1.23.2

require (
	github.com/octaviocarpes/go-http-servers/internal/auth v0.0.0
	github.com/octaviocarpes/go-http-servers/server v0.0.0
	github.com/octaviocarpes/go-http-servers/utils v0.0.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/lib/pq v1.10.9 // indirect
	golang.org/x/crypto v0.29.0 // indirect
)

replace github.com/octaviocarpes/go-http-servers/internal/auth v0.0.0 => ./internal/auth

replace github.com/octaviocarpes/go-http-servers/utils v0.0.0 => ./utils

replace github.com/octaviocarpes/go-http-servers/server v0.0.0 => ./server
