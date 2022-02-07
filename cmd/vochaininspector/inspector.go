package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"time"

	flag "github.com/spf13/pflag"
	tmcfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/store"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func main() {
	var dataDir, chain, action, logLevel, pid string
	var blockHeight int
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot get user home directory with error: %v", err)
	}
	dataDir = filepath.Join(home, ".dvote", chain, "vochain")
	flag.StringVar(&dataDir, "dataDir", dataDir, "vochain data directory (absolute path)")
	flag.StringVar(&logLevel, "logLevel", "info", "log level [error,warn,info,debug]")
	flag.StringVar(&chain, "chain", "stage", "chain [stage,dev,prod]")
	flag.StringVar(&action, "action", "sync", `action to execute: 
	sync = synchronize the blockchain
	block = print a block from the blockstore
	blockList = list blocks and numfer of transactions
	listProcess = list voting processes from the state at specific height
	listVotes = list votes from the state at specific height
	listBlockVotes = list existing votes from a block (with nullifier)
	stateGraph = prints the graphViz of the state main tree`)
	flag.IntVar(&blockHeight, "height", 0, "height block to inspect")
	flag.StringVar(&pid, "processId", "", "processId as hexadecimal string")

	flag.Parse()
	log.Init(logLevel, "stdout")

	switch action {
	case "listProcess":
		if blockHeight == 0 {
			log.Fatal("listProcess requires a heigh value")
		}
		path := filepath.Join(dataDir, "data", "vcstate")
		log.Infof("opening state database path %s", path)
		listStateProcesses(int64(blockHeight), path)

	case "listVotes":
		if blockHeight == 0 {
			log.Fatal("listVotes requires a heigh value")
		}
		path := filepath.Join(dataDir, "data", "vcstate")
		log.Infof("opening state database path %s", path)
		listStateVotes(pid, int64(blockHeight), path)

	case "listBlockVotes":
		if blockHeight == 0 {
			log.Fatal("listBlockVotes requires a heigh value")
		}
		listBlockVotes(int64(blockHeight), dataDir)

	case "stateGraph":
		if blockHeight == 0 {
			log.Fatal("stateGraph requires a heigh value")
		}
		path := filepath.Join(dataDir, "data", "vcstate")
		graphVizMainTree(int64(blockHeight), path)

	case "sync":
		vnode := newVochain(chain, dataDir)
		vi := vochaininfo.NewVochainInfo(vnode)
		go vi.Start(10)
		go service.VochainPrintInfo(20, vi)

		defer func() {
			vnode.Node.Stop()
			vnode.Node.Wait()
		}()

		// Wait for Vochain to be ready
		var h, hPrev uint32
		for vnode.Node == nil {
			hPrev = h
			time.Sleep(time.Second * 10)
			h = vnode.Height()
			log.Infof("[vochain info] replaying height %d at %d blocks/s",
				h, (h-hPrev)/5)
		}
		select {}

	case "blockList":
		vnode := newVochain(chain, dataDir)
		vnode.Node.Stop()
		height, err := vnode.State.LastHeight()
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("last height is %d", height)
		for i := uint32(1); i <= height; i++ {
			blk := vnode.GetBlockByHeight(int64(i))
			if blk == nil {
				log.Fatalf("block %d does not exist", blockHeight)
			}
			fmt.Printf("block %d AppHash:%s BlkHash:%s txs:%d\n", i, blk.AppHash, blk.Hash(), len(blk.Txs))
		}

	case "block":
		vnode := newVochain(chain, dataDir)
		vnode.Node.Stop()
		blk := vnode.GetBlockByHeight(int64(blockHeight))
		if blk == nil {
			log.Fatalf("block %d does not exist", blockHeight)
		}
		blkData, err := json.MarshalIndent(blk, " ", " ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nBlock %d:\n%s\n", blockHeight, blkData)
		for i := 0; i < len(blk.Txs); i++ {
			stx, err := vnode.GetTx(uint32(blockHeight), int32(i))
			if err != nil {
				log.Fatal(err)
			}
			tx := &models.Tx{}
			if err := proto.Unmarshal(stx.Tx, tx); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("-------------%d--------------\n", i)
			switch tx.Payload.(type) {
			case *models.Tx_Vote:
				vote := tx.GetVote()
				fmt.Printf("Tx %d: vote to process %x\n", i, vote.GetProcessId())
			case *models.Tx_SetProcess:
				setp := tx.GetSetProcess()
				fmt.Printf("Tx %d: setProcess to process %x\n", i, setp.GetProcessId())
			case *models.Tx_NewProcess:
				newp := tx.GetNewProcess()
				fmt.Printf("Tx %d: newProcess %x\n", i, newp.Process.ProcessId)
			default:
				fmt.Printf("Tx %d is of type: %s", i, reflect.TypeOf(tx))
			}
			fmt.Println(log.FormatProto(tx))
			fmt.Println("-------------END--------------")
		}
	default:
		log.Fatalf("Action %s not recognized", action)
	}
}

func newVochain(chain, dataDir string) *vochain.BaseApplication {
	cfg := &config.VochainCfg{
		Chain:     chain,
		Dev:       true,
		LogLevel:  "error",
		DataDir:   dataDir,
		P2PListen: "0.0.0.0:21500",
		DBType:    "pebble",
	}

	ip, err := util.PublicIP(4)
	if err != nil {
		log.Warn(err)
	} else {
		_, port, err := net.SplitHostPort(cfg.P2PListen)
		if err != nil {
			log.Fatal(err)
		}
		cfg.PublicAddr = net.JoinHostPort(ip.String(), port)
	}
	log.Infof("external ip address %s", cfg.PublicAddr)
	// Create the vochain node
	genesisBytes := []byte(vochain.Genesis[chain].Genesis)
	return vochain.NewVochain(cfg, genesisBytes)
}

func voteID(pid, nullifier []byte) ([]byte, error) {
	if len(pid) != types.ProcessIDsize {
		return nil, fmt.Errorf("wrong processID size %d", len(pid))
	}
	if len(nullifier) != types.VoteNullifierSize {
		return nil, fmt.Errorf("wrong nullifier size %d", len(nullifier))
	}
	vid := sha256.New()
	vid.Write(pid)
	vid.Write(nullifier)
	return vid.Sum(nil), nil
}

func listBlockVotes(height int64, blockStoreDir string) {
	cfg := tmcfg.DefaultConfig()
	cfg.RootDir = blockStoreDir
	blockStoreDB, err := node.DefaultDBProvider(&node.DBContext{ID: "blockstore", Config: cfg})
	if err != nil {
		log.Fatal("Can't open blockstore")
	}
	blockStore := store.NewBlockStore(blockStoreDB)
	if blockStore == nil {
		log.Fatal("Blockstore is nil")
	}
	block := blockStore.LoadBlock(height)
	if block == nil {
		log.Fatal("Block is nil")
	}
	fmt.Printf("Block txs: %v\n", len(block.Data.Txs))
	for i, blockTx := range block.Data.Txs {
		tx := new(vochain.VochainTx)
		if err := tx.Unmarshal(blockTx, ""); err != nil {
			log.Error(err)
			continue
		}

		//	fmt.Printf("%+v\n", tx)
		vote := tx.Tx.GetVote()
		if vote == nil {
			continue
		}
		pubKey, err := ethereum.PubKeyFromSignature(tx.SignedBody, tx.Signature)
		if err != nil {
			log.Fatalf("cannot extract public key from signature: %v", err)
		}
		addr, err := ethereum.AddrFromPublicKey(pubKey)
		if err != nil {
			log.Fatalf("cannot extract address from public key: %v", err)
		}
		vote.Nullifier = vochain.GenerateNullifier(addr, vote.ProcessId)
		// fmt.Printf("%+v\n", vote)
		vid, err := voteID(vote.ProcessId, vote.Nullifier)
		if err != nil {
			log.Fatalf("cannot get voteID: %v", err)
		}

		fmt.Printf("vote %d pid:%x vid:%x proofType:%s\n", i, vote.ProcessId, vid, reflect.TypeOf(vote.Proof.Payload))
	}
}

func openStateAtHeight(height int64, stateDir string) *statedb.TreeView {
	database, err := metadb.New(db.TypePebble, stateDir)
	if err != nil {
		log.Fatalf("Can't open DB: %v", err)
	}
	sdb := statedb.NewStateDB(database)
	lastHeight, err := sdb.Version()
	if err != nil {
		log.Fatal("Can't get last height")
	}
	log.Infof("Last height found on state is %d", lastHeight)
	if height > int64(lastHeight) {
		log.Fatal("Height cannot be greather than lastHeight")
	}
	root, err := sdb.VersionRoot(uint32(height))
	if err != nil {
		log.Fatalf("Can't get VersionRoot: %v", err)
	}
	fmt.Printf("height: %v, root: %x\n", height, root)
	snapshot, err := sdb.TreeView(root)
	if err != nil {
		log.Fatalf("Can't get TreeView at root %x: %v", root, err)
	}
	return snapshot
}

func graphVizMainTree(height int64, stateDir string) {
	snapshot := openStateAtHeight(height, stateDir)
	fmt.Println("--- mainTree ---")
	if err := snapshot.PrintGraphviz(); err != nil {
		log.Fatalf("Can't PrintGraphviz: %v", err)
	}
}

func listStateProcesses(height int64, stateDir string) {
	log.Infof("listing state processes for height %d", height)
	snapshot := openStateAtHeight(height, stateDir)
	processes, err := snapshot.DeepSubTree(vochain.ProcessesCfg)
	if err != nil {
		log.Fatalf("Can't get Processes: %v", err)
	}
	processes.Iterate(func(pid, processBytes []byte) bool {
		var process models.StateDBProcess
		if err := proto.Unmarshal(processBytes, &process); err != nil {
			log.Fatalf("Cannot unmarshal process (%s): %w", pid, err)
		}
		votes := countVotes(snapshot, pid)
		fmt.Printf("eid: %x, pid: %x, census: %x, votesHash: %x, votes: %d\n",
			process.Process.EntityId, pid, process.Process.CensusRoot, process.VotesRoot, votes)
		return false
	})
}

func countVotes(snapshot *statedb.TreeView, processID []byte) int64 {
	votes, err := snapshot.DeepSubTree(vochain.ProcessesCfg, vochain.VotesCfg.WithKey(processID))
	if err != nil {
		if errors.Is(err, statedb.ErrEmptyTree) {
			return 0
		}
		log.Fatalf("Can't get Votes for processID %x: %v", processID, err)
	}
	count := int64(0)
	votes.Iterate(func(voteID, voteBytes []byte) bool {
		count++
		return false
	})
	return count
}

func listStateVotes(processID string, height int64, stateDir string) {
	snapshot := openStateAtHeight(height, stateDir)
	pid, err := hex.DecodeString(util.TrimHex(processID))
	if err != nil {
		panic(err)
	}
	votes, err := snapshot.DeepSubTree(vochain.ProcessesCfg, vochain.VotesCfg.WithKey(pid))
	if err != nil {
		log.Fatalf("Can't get Votes: %v", err)
	}
	count := 0
	votes.Iterate(func(voteID, voteBytes []byte) bool {
		count++
		fmt.Printf("%x %x\n", voteID, voteBytes)
		return false
	})
	fmt.Printf("Votes: %v\n", count)
}
