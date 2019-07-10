# go-dvote

This repository contains a set of libraries and tools for the Vocdoni's backend infrastrucutre, as described [in the documentation](http://vocdoni.io/docs/#/).

The list of components that are implemented by `go-dvote` are

+ Voting relay
+ Gateway
+ Bootnode
+ Census service

## Gateway

Gateways provide an entry point to the P2P networks. 
They allow clients to reach decentralized services (census, relays, blockchain, etc.) through a WebSocket or an HTTP API interface.

```
git clone https://gitlab.com/vocdoni/go-dvote.git
cd go-dvote
unset GOPATH
go build cmd/gatewat/gateway.go
./gateway --help
```

---

If you run `go get ./...` it will update dependencies and `go.mod` file. Unless you are sure what you are doing, it's better to not update it. 
Go modules is still in early stage of adoption and dependencies might easy break.
