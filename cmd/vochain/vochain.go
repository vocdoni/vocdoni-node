package main

import (
	"github.com/tendermint/tendermint/abci/types"
	log "gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/vochain/mock"
)

func main() {
	app := mock.NewCounterApplication(false)
	initc := app.InitChain(types.RequestInitChain{})
	log.Infof("%+v", initc)
	/*
		for i := 0; i < 10; i++ {
			app.Commit()
		}
	*/
	for {
		app.Commit()
	}
	log.Infof("%+v", app.Info(types.RequestInfo{}))
}
