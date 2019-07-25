package main

import (
	"path/filepath"

	"gitlab.com/vocdoni/go-dvote/vochain/mock"
)

func main() {
	ctx := mock.NewDefaultContext()
	mock.StartInProcess(ctx, "counterApp", filepath.Join(ctx.Config.RootDir, "tendermint_data"))
	//_, err := mock.startInProcess(ctx)
	//app := mock.NewCounterApplication(false)
	//initc := app.InitChain(types.RequestInitChain{})
	//log.Infof("%+v", initc)
	/*
		for i := 0; i < 10; i++ {
			app.Commit()
		}
	*/
	//for {
	//app.Commit()
	//}
	//log.Infof("%+v", app.Info(types.RequestInfo{}))
}
