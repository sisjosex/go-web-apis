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
docker build -t go-web:1 .

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


## Sample apis

```
// Register
curl --location 'http://localhost:8080/api/v1/auth/register' \
--header 'Content-Type: application/json' \
--data-raw '{
    "first_name": "Jhon",
    "last_name": "Smith",
    "email": "jhon-smith@mail.com",
    "password": "$Password2025",
    "phone": "11111111",
    "birthday": "1985-01-01",
    "profile_picture_url": "http:/fotos.com/profile.png",
    "bio": "some bio",
    "website_url": "http:/page.com"
}'
```

```
// Login
curl --location 'http://localhost:8080/api/v1/auth/login' \
--header 'User-Agent: Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:89.0; Device=Laptop) Gecko/20100101 Firefox/89.0' \
--header 'Content-Type: application/json' \
--data-raw '{
    "email": "jhon-smith@mail.com",
    "password": "$Password2025"
}'
```