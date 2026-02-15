# Builder
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy
ARG GO_MAIN=platform
RUN go build -o app ./cmd/${GO_MAIN}

# Runner
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/app ./app
COPY --from=builder /app/modules ./modules
COPY config/regexes.yaml ./config/regexes.yaml

EXPOSE 8080 9080
CMD ["./app"]
