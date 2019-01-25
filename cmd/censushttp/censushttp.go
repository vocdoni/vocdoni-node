package main

import (
	"log"
	"os"
	"strconv"

	censusmanager "github.com/vocdoni/dvote-census/service"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: " + os.Args[0] + " <port> [pubKey]")
		os.Exit(2)
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
		os.Exit(2)
	}
	pubK := ""
	if len(os.Args) > 2 {
		log.Print("Public key authentication enabled")
		pubK = os.Args[2]
	}
	log.Print("Starting process HTTP service on port " + os.Args[1])
	censusmanager.T.Init()
	censusmanager.Listen(port, "http", pubK)

}
