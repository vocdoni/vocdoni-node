package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	censusmanager "github.com/vocdoni/dvote-census/service"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: " + os.Args[0] +
			" <port> <namespace>[:pubKey] [<namespace>[:pubKey]]...")
		os.Exit(2)
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
		os.Exit(2)
	}
	for i := 2; i < len(os.Args); i++ {
		s := strings.Split(os.Args[i], ":")
		ns := s[0]
		pubK := ""
		if len(s) > 1 {
			pubK = s[1]
			log.Printf("Public Key authentication enabled on namespace %s\n", ns)
		}
		censusmanager.AddNamespace(ns, pubK)
		log.Printf("Starting process HTTP service on port %d for namespace %s\n",
			port, ns)
	}
	censusmanager.Listen(port, "http")

}
