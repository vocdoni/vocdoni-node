# syntax=docker/dockerfile:experimental

FROM golang:1.26 AS builder

ARG BUILDARGS

# Build all the binaries at once, so that the final targets don't require having
# Go installed to build each of them.
WORKDIR /src
ENV CGO_ENABLED=1
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
	--mount=type=bind,source=go.sum,target=go.sum \
	--mount=type=bind,source=go.mod,target=go.mod \
	go mod download -x
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
	--mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
	--mount=type=bind,target=. \
	go build -trimpath -o=/bin -ldflags="-w -s -X=go.vocdoni.io/dvote/internal.Version=$(git describe --always --tags --dirty --match='v[0-9]*')" $BUILDARGS \
	./cmd/node ./cmd/voconed ./cmd/end2endtest

FROM debian:bookworm-slim as base
WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

RUN apt-get update && \
	apt-get install --no-install-recommends -y curl && \
	apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

FROM base as test
COPY --from=builder /bin/end2endtest /app/

FROM base
COPY --from=builder /bin/node /bin/voconed /app/

ENTRYPOINT ["/app/node"]
