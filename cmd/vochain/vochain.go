package main

import (
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func main() {
	vochain.InitTendermint()
	vochain.StartTendermint()
}
