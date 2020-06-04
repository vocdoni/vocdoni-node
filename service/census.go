package service

import (
	"os"
	"time"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
)

func Census(datadir string, ma *metrics.Agent) (*census.Manager, error) {
	log.Info("creating census service")
	var censusManager census.Manager
	if _, err := os.Stat(datadir + "/census"); os.IsNotExist(err) {
		if err := os.MkdirAll(datadir+"/census", os.ModePerm); err != nil {
			return nil, err
		}
	}
	if err := censusManager.Init(datadir+"/census", ""); err != nil {
		return nil, err
	}

	// Collect metrics for prometheus
	go censusManager.CollectMetrics(ma)

	// Print log info
	go func() {
		var local, imported, loaded int
		for {
			time.Sleep(time.Second * 20)
			local, imported, loaded = censusManager.Count()
			log.Infof("[census info] local:%d imported:%d loaded:%d queue:%d/%d toRetry:%d", local, imported,
				loaded, censusManager.ImportQueueSize(), census.ImportQueueRoutines, censusManager.ImportFailedQueueSize())
		}
	}()

	return &censusManager, nil
}
