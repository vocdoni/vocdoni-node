# vocone

Vocone is for testing purposes, executes the vocdoni protocol logic in a single program.

Start with:
```
docker compose build
docker compose up -d
```

Ports 80 and 9090 are now serving the voconi API.

For running a voting test:
```
go run ./cmd/end2endtest --host=http://localhost/v2 --logLevel=debug --votes=100 --faucet=http://localhost/v2/faucet/dev/
```
