package main

import (
	"fmt"

	swarm "github.com/vocdoni/go-dvote/net/swarm"
)

func main() {
	sn := new(swarm.SwarmNet)
	err := sn.Init()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	sn.Test()
}
