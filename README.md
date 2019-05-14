# go-dvote

This repository contains a set of libraries and tools for the Vocdoni's backend infrastrucutre, as described [in the documentation](http://vocdoni.io/docs/#/).

The list of components that are implemented by `go-dvote` are

+ Voting relay
+ Gateway
+ Bootnode
+ Census service

### Compile

```
git clone https://github.com/vocdoni/go-dvote.git
cd go-dvote
unset GOPATH
go build cmd/generator/generator.go
```

If you run `go get ./...` it will update dependencies and `go.mod` file. Unless you are sure what you are doing, it's better to not update it. 
Go modules is still in early stage of adoption and dependencies might easy break.
