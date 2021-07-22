# syntax=docker/dockerfile:experimental

FROM golang:1.16.5 AS builder

ARG BUILDARGS

# Build all the binaries at once, so that the final targets don't require having
# Go installed to build each of them.
WORKDIR /src
COPY . .
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
	--mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
	go build -trimpath -tags=badgerdb -o=. -ldflags="-w -s -X=go.vocdoni.io/dvote/internal.Version=$(git describe --always --tags --dirty --match='v[0-9]*')" -mod=readonly $BUILDARGS \
	./cmd/dvotenode ./cmd/vochaintest

FROM debian:10.10-slim AS test

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
RUN apt update && apt install -y curl
COPY --from=builder /src/vochaintest ./

FROM debian:10.10-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/dvotenode ./
ENTRYPOINT ["/app/dvotenode"]
