package service

import (
	"time"

	"github.com/nsqio/go-diskqueue"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/events"
	"go.vocdoni.io/dvote/vochain"
)

const (
	maxBytesPerFile = 5000000 // 5 MB
	maxMsgSize      = 100000  // 0.1 MB
	minMsgSize      = 1       // 1 Byte
	syncTimeout     = time.Duration(time.Second * 5)
	syncEvery       = 100
)

// Events creates the dvote events service
func Events(cfg *config.EventsCfg, signer *ethereum.SignKeys, vochain *vochain.BaseApplication) {

	// TODO: @jordipainan integrate with our logger
	l := func(lvl diskqueue.LogLevel, f string, args ...interface{}) {}

	// create and start dispatcher
	dispatcher := events.NewDispatcher(signer, vochain, cfg.Datadir, maxBytesPerFile, minMsgSize, maxMsgSize, syncEvery, syncTimeout, l)
	dispatcher.StartDispatcher()
}
