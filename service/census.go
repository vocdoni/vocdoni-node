package service

import (
	"os"
	"path"
	"time"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
)

func Census(datadir string, ma *metrics.Agent) (*census.Manager, error) {
	log.Info("creating census service")
	var censusManager census.Manager
	stdir := path.Join(datadir, "census")
	if _, err := os.Stat(stdir); os.IsNotExist(err) {
		if err := os.MkdirAll(stdir, os.ModePerm); err != nil {
			return nil, err
		}
	}
	if err := censusManager.Init(stdir, ""); err != nil {
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
