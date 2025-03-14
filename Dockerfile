# Builder
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main .

# Runner
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/main ./main
COPY --from=builder /app/migrations ./migrations
COPY config/regexes.yaml ./config/regexes.yaml
COPY .env .

EXPOSE 8080
CMD ["./main"]
