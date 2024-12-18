package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	timeout = 20 * time.Minute
)

type census struct {
	Root types.HexBytes `json:"root"`
	URI  string         `json:"uri"`
	Type string         `json:"type"`
	Data []struct {
		Address common.Address `json:"address"`
		Balance string         `json:"balance"`
	} `json:"data"`
}

func main() {
	uri := flag.String("url", "", "url of the census, http(s):// or ipfs://")
	output := flag.String("output", "census.json", "Output file")
	flag.Parse()
	log.Init("debug", "stdout", nil)

	if *uri == "" {
		flag.Usage()
		return
	}

	dataDir, err := os.MkdirTemp("", "census-import")
	if err != nil {
		log.Fatalf("could not create temporary directory: %s", err)
	}
	defer os.RemoveAll(dataDir)

	db, err := metadb.New(db.TypePebble, filepath.Join(dataDir, "db"))
	if err != nil {
		log.Fatalf("could not create database: %s", err)
	}

	// check if the URI prefix is ipfs or http(s)
	var censusData []byte
	switch {
	case strings.HasPrefix(*uri, "ipfs://"):
		log.Infow("importing census from IPFS storage", "uri", *uri)
		// create the IPFS client
		storage := new(ipfs.Handler)
		if err := storage.Init(&types.DataStore{Datadir: filepath.Join(dataDir, "ipfs")}); err != nil {
			log.Fatalf("could not create IPFS storage: %s", err)
		}

		// retrieve the census data from the remote storage
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		censusData, err = storage.Retrieve(ctx, *uri, 0)
		if err != nil {
			log.Fatalf("could not retrieve census data: %s", err)
		}
	case strings.HasPrefix(*uri, "http://"), strings.HasPrefix(*uri, "https://"):
		// retrieve the census data from the HTTP server
		log.Infow("importing census from HTTP endpoint", "url", *uri)
		httpclient := &http.Client{Timeout: timeout}
		resp, err := httpclient.Get(*uri)
		if err != nil {
			log.Fatalf("could not retrieve census data: %s", err)
		}
		defer resp.Body.Close()
		censusData, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("could not read census data: %s", err)
		}
	default:
		log.Fatalf("unsupported URL scheme: %s", *uri)
	}
	log.Infow("census data retrieved", "size", len(censusData))

	// import the census data into the database
	log.Infow("importing census data into the database")
	id := util.RandomBytes(32)
	censusDB := censusdb.NewCensusDB(db)
	if err := censusDB.ImportTree(id, censusData); err != nil {
		log.Fatalf("could not import census data: %s", err)
	}
	log.Infow("census data imported!")
	censusRef, err := censusDB.Load(id, nil)
	if err != nil {
		log.Fatalf("could not load census data: %s", err)
	}
	defer censusDB.UnLoad()

	// print the census data
	root, err := censusRef.Tree().Root()
	if err != nil {
		log.Fatalf("could not get root of census data: %s", err)
	}
	census := census{
		Root: types.HexBytes(root),
		URI:  censusRef.URI,
		Type: models.Census_Type_name[int32(censusRef.CensusType)],
	}

	if err := censusRef.Tree().IterateLeaves(func(key, value []byte) bool {
		balance := arbo.BytesLEToBigInt(value)
		census.Data = append(census.Data, struct {
			Address common.Address `json:"address"`
			Balance string         `json:"balance"`
		}{
			Address: common.BytesToAddress(key),
			Balance: balance.String(),
		})
		return true
	}); err != nil {
		log.Fatalf("could not iterate census data: %s", err)
	}

	censusPrint, err := json.MarshalIndent(census, "", "  ")
	if err != nil {
		log.Fatalf("could not marshal census data: %s", err)
	}

	if err := os.WriteFile(*output, censusPrint, 0o644); err != nil {
		log.Fatalf("could not write census data: %s", err)
	}
	log.Infow("census data exported", "file", *output)
}
