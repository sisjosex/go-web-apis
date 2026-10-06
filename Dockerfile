# Builder
# Matches the `go` directive in go.mod — 1.23 no longer builds this module.
FROM golang:1.25-alpine AS builder
WORKDIR /app
# Modules first (INFRA-004): a source change rebuilds from the compile step on, the download
# layer stays cached across deploys on the server.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# One image serves both servers and the migrate job: ./cmd/server picks platform
# or tenant from -mode, ./cmd/cli carries `migrate` and `tenant -migrate all`.
# Static and stripped: no libc at runtime, no symbol table, no build paths.
# noswagger: no /swagger in production, and ~9 MB less binary (routes/swagger.go).
# noasynqmon: no jobs UI either, ~7 MB less (core/jobs/monitor.go, INFRA-011).
RUN CGO_ENABLED=0 go build -trimpath -tags noswagger,noasynqmon -ldflags="-s -w" -o app ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -tags noswagger,noasynqmon -ldflags="-s -w" -o cli ./cmd/cli
# The only files either binary reads from disk: migrations, translations, e-mail templates.
# The Go sources and the test media under modules/ stay behind.
RUN mkdir /assets \
 && find modules -type f \( -path '*/migrations/*' -o -path '*/lang/*' -o -path '*/templates/*' \) \
  | tar -cf - -T - | tar -xf - -C /assets

# Runner
FROM alpine:3.20
WORKDIR /app
# busybox already provides the wget the compose healthcheck calls; only the CA
# bundle is missing from a bare alpine, and SMTP over TLS needs it.
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/app ./app
COPY --from=builder /app/cli ./cli
COPY --from=builder /assets/modules ./modules
COPY config/regexes.yaml ./config/regexes.yaml

EXPOSE 8080 9080
CMD ["./app"]
