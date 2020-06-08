# This first chunk downloads dependencies and builds the binaries, in a way that
# can easily be cached and reused.

FROM golang:1.14.4 AS builder

WORKDIR /src

# We also need the duktape stub for the 'go mod download'. Note that we need two
# COPY lines, since otherwise we do the equivalent of 'cp duktape-stub/* .'.
COPY go.mod go.sum ./
COPY duktape-stub duktape-stub
RUN go mod download

# Build all the binaries at once, so that the final targets don't require having
# Go installed to build each of them.
COPY . .
RUN go build -o=. -ldflags='-w -s' -mod=readonly ./cmd/dvotenode ./cmd/vochaintest

FROM debian:10.4-slim AS test

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/vochaintest ./

FROM debian:10.4-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/dvotenode /src/dockerfiles/dvotenode/files/dvoteStart.sh ./
ENTRYPOINT ["/app/dvoteStart.sh"]
