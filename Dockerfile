# Builder
# Matches the `go` directive in go.mod — 1.23 no longer builds this module.
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
# One image serves both servers and the migrate job: ./cmd/server picks platform
# or tenant from -mode, ./cmd/cli carries `migrate` and `tenant -migrate all`.
RUN go build -o app ./cmd/server && go build -o cli ./cmd/cli

# Runner
FROM alpine:3.20
WORKDIR /app
# busybox already provides the wget the compose healthcheck calls; only the CA
# bundle is missing from a bare alpine, and SMTP over TLS needs it.
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/app ./app
COPY --from=builder /app/cli ./cli
COPY --from=builder /app/modules ./modules
COPY config/regexes.yaml ./config/regexes.yaml

EXPOSE 8080 9080
CMD ["./app"]
