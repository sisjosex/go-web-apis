# Go lang optimized backend

Optimized backend using Gin with Go, and postgresql database pool based for multiple connections

## Requirements

* Golang installed
* Postgres (for local environment)

## Libraries/Db

* Go
* Gin
* PostgreSQL
* Docker compose

## How to run locally

```
go mod tidy
go run .
```

## Building

```
go build -t go-web:1 .

docker run -p 8080:8080 -e DATABASE_URL=postgres://postgres:dbpassword@<server-ip>:5432/web?sslmode=disable go-web:1
```


## Build images
```
docker compose --env-file .env.docker build

docker compose --env-file .env.docker up

// or

docker compose --env-file .env.docker up --build
```

## Error codes


