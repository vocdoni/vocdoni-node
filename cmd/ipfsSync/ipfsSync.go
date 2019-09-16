package main

import (
	"os/user"
	"time"

	flag "github.com/spf13/pflag"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/ipfssync"
	"gitlab.com/vocdoni/go-dvote/log"
)

func main() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	userDir := usr.HomeDir + "/.dvote"
	logLevel := flag.String("logLevel", "info", "log level")
	dataDir := flag.String("dataDir", userDir, "directory for storing data")
	key := flag.String("key", "", "symetric key of the sync ipfs cluster")
	port := flag.Int16("port", 4171, "port for the sync network")
	helloTime := flag.Int("helloTime", 40, "period in seconds for sending hello messages")
	updateTime := flag.Int("updateTime", 20, "period in seconds for sending update messages")
	flag.Parse()
	log.InitLoggerAtLevel(*logLevel)

	ipfsStore := data.IPFSNewConfig(*dataDir + "/.ipfs")
	storage, err := data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		log.Fatal(err.Error())
	}

	is := ipfssync.NewIPFSsync(*dataDir, *key, storage)
	is.HelloTime = *helloTime
	is.UpdateTime = *updateTime
	is.Port = *port
	is.Start()
	for {
		time.Sleep(1 * time.Second)
	}
}
