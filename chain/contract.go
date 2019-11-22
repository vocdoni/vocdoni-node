package chain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	votingProcess "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
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
	client, err := ethclient.Dial("http://127.0.0.1:9091")
	if err != nil {
		log.Error(err)
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

func (ph *ProcessHandle) GetProcessMetadata(pid [32]byte) (*ProcessMetadata, error) {
	processInfoStructured := new(ProcessMetadata)
	processMeta, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		return processInfoStructured, err
	}
	processInfo, err := ph.storage.Retrieve(processMeta.Metadata)
	if err != nil {
		return processInfoStructured, err
	}
	censusTree, err := ph.storage.Retrieve(processMeta.CensusMerkleTree)
	if err != nil {
		return processInfoStructured, err
	}
	//json.Unmarshal(processInfo, &processInfoStructured)
	log.Info("Structured Info: %s", processInfo)
	log.Info("Merkle tree: %s", censusTree)
	return processInfoStructured, nil
}

func (ph *ProcessHandle) GetProcessTxArgs(pid [32]byte) (*types.NewProcessTx, error) {
	processMeta, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		log.Errorf("Error fetching process metadata from Ethereum: %s", err)
	}

	processTxArgs := new(types.NewProcessTx)
	processTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	processTxArgs.EntityID = processMeta.EntityAddress.String()
	processTxArgs.MkRoot = processMeta.CensusMerkleRoot
	processTxArgs.NumberOfBlocks = processMeta.NumberOfBlocks.Int64()
	processTxArgs.StartBlock = processMeta.StartBlock.Int64()
	processTxArgs.EncryptionPublicKeys = []string{processMeta.VoteEncryptionPrivateKey}
	if processMeta.ProcessType == "snark-vote" || processMeta.ProcessType == "poll-vote" || processMeta.ProcessType == "petition-sign" {
		processTxArgs.ProcessType = processMeta.ProcessType
	} else {
		processTxArgs.ProcessType = ""
	}
	processTxArgs.Type = "newProcess"
	return processTxArgs, nil
}

func (ph *ProcessHandle) GetProcessIndex(pid [32]byte) (*big.Int, error) {
	return ph.VotingProcess.GetProcessIndex(nil, pid)
}

func (ph *ProcessHandle) GetOracles() ([]string, error) {
	return ph.VotingProcess.GetOracles(nil)
}

func (ph *ProcessHandle) GetValidators() ([]string, error) {
	return ph.VotingProcess.GetValidators(nil)
}

func (ph *ProcessHandle) GetGenesis() (string, error) {
	return ph.VotingProcess.GetGenesis(nil)
}
