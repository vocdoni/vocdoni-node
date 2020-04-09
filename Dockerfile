# This first chunk downloads dependencies and builds the binaries, in a way that
# can easily be cached and reused.

FROM golang:1.14.2 AS builder

WORKDIR /src

# We also need the duktape stub for the 'go mod download'. Note that we need two
# COPY lines, since otherwise we do the equivalent of 'cp duktape-stub/* .'.
COPY go.mod go.sum ./
COPY duktape-stub duktape-stub
RUN go mod download

COPY . .
RUN go build -o=. -ldflags='-w -s' -mod=readonly ./cmd/...

# These multiple targets can be used to obtain each of the images, such as
# --target=miner.

# Note that debian slim images are very minimal, so they don't contain
# ca-certificates. Add them, as it's needed for outbound TLS to work, which is a
# requirement to obtain let's encrypt certificates.

FROM debian:10.3-slim AS gateway
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/gateway /src/dockerfiles/gateway/files/gatewayStart.sh ./
ENTRYPOINT ["/app/gatewayStart.sh"]

FROM debian:10.3-slim AS census
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/censushttp /src/dockerfiles/census/files/censusStart.sh ./
ENTRYPOINT ["/app/censusStart.sh"]

FROM debian:10.3-slim AS miner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/miner /src/dockerfiles/miner/files/minerStart.sh ./
ENTRYPOINT ["/app/minerStart.sh"]

FROM debian:10.3-slim AS oracle
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /src/oracle /src/dockerfiles/oracle/files/oracleStart.sh ./
ENTRYPOINT ["/app/oracleStart.sh"]
