package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os/user"
	"os"
	"time"

	"github.com/spf13/viper"
	flag "github.com/spf13/pflag"

	"gitlab.com/vocdoni/go-dvote/batch"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/db"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/config"
)

var batchSeconds = 6000 //seconds
var batchSize = 10      //votes

var err error
var batchTimer *time.Ticker
var batchSignal chan bool
var signal bool
var transportType net.TransportID
var storageType data.StorageID


func newConfig() (config.RelayCfg, error) {
	var globalCfg config.RelayCfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	defaultDirPath := usr.HomeDir + "/.dvote/relay"
	path := flag.String("cfgpath", defaultDirPath+"/config.yaml", "cfgpath. Specify filepath for relay config")

	flag.String("loglevel", "warn", "Log level must be one of: debug, info, warn, error, dpanic, panic, fatal")
	flag.String("transport", "PSS", "Transport must be one of: PSS, PubSub")
	flag.String("storage", "IPFS", "Transport must be one of: BZZ, IPFS")
	
	flag.Parse()

	viper := viper.New()
	viper.SetDefault("logLevel", "warn")
	viper.SetDefault("transportIDString", "PSS")
	viper.SetDefault("storageIDString", "IPFS")

	viper.SetConfigType("yaml")
	if *path == defaultDirPath+"/config.yaml" { //if path left default, write new cfg file if empty or if file doesn't exist.
		if err = viper.SafeWriteConfigAs(*path); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(defaultDirPath, os.ModePerm)
				if err != nil {
					return globalCfg, err
				}
				err = viper.WriteConfigAs(*path)
				if err != nil {
					return globalCfg, err
				}
			}
		}
	}

	//bind flags to config
	viper.BindPFlag("logLevel", flag.Lookup("loglevel"))
	viper.BindPFlag("transportIDString", flag.Lookup("transport"))
	viper.BindPFlag("storageIDString", flag.Lookup("storage"))
	
	
	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}


func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	dataDir := usr.HomeDir + "/.dvote"
	dbPath := dataDir + "/relay.db"

	db, err := db.NewLevelDbStorage(dbPath, false)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	batch.Setup(db)


	transportType = net.TransportIDFromString(globalCfg.TransportIDString)
	storageType = data.StorageIDFromString(globalCfg.StorageIDString)

	batchTimer = time.NewTicker(time.Second * time.Duration(batchSeconds))
	batchSignal = make(chan bool)

	batch.BatchSignal = batchSignal
	batch.BatchSize = batchSize

	transportOutput := make(chan types.Message)

	log.Infof("Initializing transport")
	transport, err := net.InitDefault(transportType)
	if err != nil {
		log.Fatal(err)
	}

	var storageConfig data.StorageConfig
	log.Infof("Initializing storage")
	switch globalCfg.StorageIDString {
	case "IPFS":
		storageConfig = data.IPFSNewConfig()
	case "BZZ":
		storageConfig = data.BZZNewConfig()
	default:
		log.Panic(errors.New("Storage not supported"))
	}

	storage, err := data.InitDefault(storageType, storageConfig)
	if err != nil {
		log.Fatal(err)
	}
	_ = storage

	/*
		log.Println("Initializing websockets")
		ws, err := net.InitDefault(net.TransportIDFromString("Websocket"))
		if err != nil {
			log.Fatal(err)
		}
	*/

	go transport.Listen(transportOutput)
	go batch.Recieve(transportOutput)
	//	go ws.Listen(listenerOutput)
	//	go net.Route(listenerOutput, storage, ws)

	log.Infof("Entering main loop")
	for {
		select {
		case <-batchTimer.C:
			//fmt.Println("Timer triggered")
			//			fmt.Println(batch.Create())
			//replace with chain link
			continue
		case signal := <-batchSignal:
			if signal == true {
				_, bs := batch.Fetch()
				buf := &bytes.Buffer{}
				gob.NewEncoder(buf).Encode(bs)
				bb := buf.Bytes()
				cid, err := storage.Publish(bb)
				if err != nil {
					log.Errorf("Storage error: %s", err)
				}
				log.Infof("Batch published at: %s \n", cid)
				// add to chain
				// announce to pubsub
				//log.Infof("Batch info: Nullifiers: %v, Batch: %v", n, b)
				//batch.Compact(ns)
			}
		default:
			continue
		}
	}
}
