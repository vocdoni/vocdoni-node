package chain

import (
	"encoding/json"
	"fmt"

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

type VoteOption struct {
	Title map[string]string `json:"title"`
	Value string            `json:"value"`
}

type QuestionMetadata struct {
	Type        string            `json:"type"`
	Question    map[string]string `json:"question"`
	Description map[string]string `json:"description"`
	VoteOptions []VoteOption      `json:"voteOptions"`
}

type ProcessMetadata struct {
	Version        string `json:"version"`
	Type           string `json:"type"`
	StartBlock     int64  `json:"startBlock"`
	NumberOfBlocks int64  `json:"numberOfBlocks"`
	Census         struct {
		MerkleRoot string `json:"merkleRoot"`
		MerkleTree string `json:"merkleTree"`
	} `json:"census"`
	Details struct {
		EntityID            string             `json:"entityID"`
		EncryptionPublicKey string             `json:"encryptionPublicKey"`
		Title               map[string]string  `json:"title"`
		Description         map[string]string  `json:"description"`
		HeaderImage         string             `json:"headerImage"`
		Questions           []QuestionMetadata `json:"questions"`
	}
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

func (ph *ProcessHandle) GetProcessMetadata(pid [32]byte) (*vochain.NewProcessTxArgs, error) {
	processMeta, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		log.Errorf("Error fetching process metadata from Ethereum: %s", err)
	}
	processInfo, err := ph.storage.Retrieve(processMeta.Metadata)
	if err != nil {
		log.Errorf("Error fetching process info from IFPS: %s", err)
	}

	processInfoStructured := new(ProcessMetadata)
	json.Unmarshal(processInfo, &processInfoStructured)
	log.Info("PROCESSINFOSTRUCTURED: %v", processInfoStructured)
	processTxArgs := new(vochain.NewProcessTxArgs)
	processTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	processTxArgs.EntityID = processMeta.EntityAddress.String()
	processTxArgs.MkRoot = processInfoStructured.Census.MerkleRoot
	processTxArgs.NumberOfBlocks = processInfoStructured.NumberOfBlocks
	processTxArgs.StartBlock = processInfoStructured.StartBlock
	processTxArgs.EncryptionPublicKey = processInfoStructured.Details.EncryptionPublicKey

	return processTxArgs, nil
}

// node.proc_transactor.do_this()

// node.proc_transactor.do_that()

// Contructor for entity_transactor on node ?>

// node.entity_transactor.do_this()

// node.entity_transactor.do_that()
