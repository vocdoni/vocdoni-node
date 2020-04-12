package service

import (
	"os"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/log"
)

func Census(datadir string) (*census.Manager, error) {
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

	go func() {
		var imported, local int
		for {
			time.Sleep(time.Second * 60)
			imported = 0
			local = 0
			for _, n := range censusManager.Census.Namespaces {
				if strings.Contains(n.Name, "/") {
					local++
				} else {
					imported++
				}
			}
			log.Infof("[census info] local:%d imported:%d", local, imported)
		}
	}()

	return &censusManager, nil
}
