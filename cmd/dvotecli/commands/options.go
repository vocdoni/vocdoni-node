package commands

import (
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
)

type options struct {
	digested bool
	colorize bool
	host     string
	privKey  string
	signKey  *ethereum.SignKeys
}

func (o options) checkSignKey() error {
	o.signKey = ethereum.NewSignKeys()
	if o.privKey == "" {
		return fmt.Errorf("a private key is needed to sign this command")
	} else {
		return o.signKey.AddHexKey(o.privKey)
	}
}
