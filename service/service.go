package service

import (
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/keykeeper"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// VocdoniService is the main struct that holds all the services of the Vocdoni node.
type VocdoniService struct {
	Config    *config.VochainCfg
	App       *vochain.BaseApplication
	Router    *httprouter.HTTProuter
	Indexer   *indexer.Indexer
	Stats     *vochaininfo.VochainInfo
	Signer    *ethereum.SignKeys
	KeyKeeper *keykeeper.KeyKeeper
}
