package chain

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	votingProcess "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	vochain "gitlab.com/vocdoni/go-dvote/vochain/types"
)

//These methods represent an exportable abstraction over raw contract bindings
//Use these methods, rather than those present in the contracts folder
type ProcessHandle struct {
	VotingProcess *votingProcess.VotingProcess
	storage       data.Storage
}

// Constructor for proc_transactor on node
func NewVotingProcessHandle(contractAddressHex string, storage data.Storage) (*ProcessHandle, error) {
	client, err := ethclient.Dial("https://gwdev1.vocdoni.net/web3")
	if err != nil {
		log.Fatal(err)
	}
	address := common.HexToAddress(contractAddressHex)

	votingProcess, err := votingProcess.NewVotingProcess(address, client)
	if err != nil {
		log.Errorf("Error constructing votingProcess handle: %s", err)
		return new(ProcessHandle), err
	}
	PH := new(ProcessHandle)
	PH.VotingProcess = votingProcess

	PH.storage = storage
	return PH, nil
}

func (ph *ProcessHandle) Get(pid [32]byte) (*vochain.NewProcessTxArgs, error) {
	processMeta, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		log.Errorf("Error fetching process metadata from Ethereum: %s", err)
	}
	processInfo, err := ph.storage.Retrieve(processMeta.Metadata)
	if err != nil {
		log.Errorf("Error fetching process info from IFPS: %s", err)
	}

	processInfoStructured := new(vochain.NewProcessTxArgs)
	json.Unmarshal(processInfo, &processInfoStructured)
	log.Info("PROCESSINFOSTRUCTURED: %v", processInfoStructured)
	return processInfoStructured, nil
}

// node.proc_transactor.do_this()

// node.proc_transactor.do_that()

// Contructor for entity_transactor on node ?>

// node.entity_transactor.do_this()

// node.entity_transactor.do_that()
