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
COPY ./dockerfiles/testsuite/js ./js
RUN cd js && npm install

FROM debian:11.6-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/node ./
COPY --from=builder /src/voconed ./

# Support for go-rapidsnark witness calculator (https://github.com/iden3/go-rapidsnark/tree/main/witness)
COPY --from=builder /go/pkg/mod/github.com/wasmerio/wasmer-go@v1.0.4/wasmer/packaged/lib/linux-amd64/libwasmer.so /go/pkg/mod/github.com/wasmerio/wasmer-go@v1.0.4/wasmer/packaged/lib/linux-amd64/libwasmer.so
# Support for go-rapidsnark prover (https://github.com/iden3/go-rapidsnark/tree/main/prover)
RUN apt-get update && \
	apt-get install -y libc6-dev libomp-dev openmpi-common libgomp1 && \
	apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/app/node"]
