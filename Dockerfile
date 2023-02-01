# syntax=docker/dockerfile:experimental

FROM golang:1.19.5 AS builder

ARG BUILDARGS

# Build all the binaries at once, so that the final targets don't require having
# Go installed to build each of them.
WORKDIR /src
COPY . .
ENV CGO_ENABLED=1
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
	--mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
	go build -trimpath -o=. -ldflags="-w -s -X=go.vocdoni.io/dvote/internal.Version=$(git describe --always --tags --dirty --match='v[0-9]*')" $BUILDARGS \
	./cmd/node ./cmd/vochaintest ./cmd/voconed ./cmd/end2endtest

FROM node:lts-bullseye-slim AS test

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/vochaintest ./
COPY --from=builder /src/end2endtest ./
COPY ./dockerfiles/testsuite/js ./js
RUN cd js && npm install

FROM debian:11.6-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/node ./
COPY --from=builder /src/voconed ./
ENTRYPOINT ["/app/node"]
