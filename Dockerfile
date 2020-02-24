# This first chunk downloads dependencies and builds the binaries, in a way that
# can easily be cached and reused.

FROM golang:1.13.7 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o=. -ldflags='-w -s' -mod=readonly ./cmd/...

# These multiple targets can be used to obtain each of the images, such as
# --target=miner.

FROM debian:10.2-slim AS gateway
WORKDIR /app
COPY --from=builder /src/gateway /src/dockerfiles/gateway/files/gatewayStart.sh ./
ENTRYPOINT ["/app/gatewayStart.sh"]

FROM debian:10.2-slim AS census
WORKDIR /app
COPY --from=builder /src/censushttp /src/dockerfiles/census/files/censusStart.sh ./
ENTRYPOINT ["/app/censusStart.sh"]

FROM debian:10.2-slim AS miner
WORKDIR /app
COPY --from=builder /src/miner /src/dockerfiles/miner/files/minerStart.sh ./
ENTRYPOINT ["/app/minerStart.sh"]

FROM debian:10.2-slim AS oracle
WORKDIR /app
COPY --from=builder /src/oracle /src/dockerfiles/oracle/files/oracleStart.sh ./
ENTRYPOINT ["/app/oracleStart.sh"]
