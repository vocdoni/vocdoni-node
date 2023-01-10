package main

import (
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
)

func testTokenTransactions(
	host,
	treasurerPrivKey string,
	keySigner string,
) {
	treasurerSigner, err := privKeyToSigner(treasurerPrivKey)
	if err != nil {
		log.Fatal(err)
	}
	// create main signer
	mainSigner := &ethereum.SignKeys{}
	if err := mainSigner.Generate(); err != nil {
		log.Fatal(err)
	}

	// create other signer
	otherSigner := &ethereum.SignKeys{}
	if err := otherSigner.Generate(); err != nil {
		log.Fatal(err)
	}

	log.Infof("connecting to main gateway %s", host)
	log.Fatal("wip, not yet implemented",
		treasurerSigner)
}
