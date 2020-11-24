# syntax=docker/dockerfile:experimental

FROM golang:1.15.2 AS builder

# Build all the binaries at once, so that the final targets don't require having
# Go installed to build each of them.
WORKDIR /src
COPY . .
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
	--mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
	go build -o=. -ldflags="-w -s -X=gitlab.com/vocdoni/go-dvote/internal.Version=$(git describe --always --tags --dirty --match='v[0-9]*')" -mod=readonly \
	./cmd/dvotenode ./cmd/vochaintest

FROM debian:10.5-slim AS test

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
RUN apt update && apt install -y curl
COPY --from=builder /src/vochaintest ./

FROM debian:10.5-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/dvotenode ./
ENTRYPOINT ["/app/dvotenode"]
