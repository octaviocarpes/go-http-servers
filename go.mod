module github.com/octaviocarpes/go-http-servers

go 1.23.2

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/octaviocarpes/go-http-servers/utils v0.0.0
	github.com/octaviocarpes/go-http-servers/server v0.0.0
)

replace github.com/octaviocarpes/go-http-servers/utils v0.0.0 => ./utils
replace github.com/octaviocarpes/go-http-servers/server v0.0.0 => ./server
