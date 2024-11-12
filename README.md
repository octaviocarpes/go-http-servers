# go-http-servers


## Migrations:
```
cd sql/schema

create a file like 0001_my_migration.sql

goose postgres "postgres://postgres:@localhost:5432/chirpy" up
```

## SQLC
 - Create a sql file on `sql/queries` with the query you would like to generate the code to

 - add a comment with the name of the generated function and the amount of results e.g:
```
-- name: GetChirpByID :one
```

 - On the root folder
```
sqlc generate
```