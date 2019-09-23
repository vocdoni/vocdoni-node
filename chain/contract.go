package chain

import (
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

/*
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
*/

// Constructor for proc_transactor on node
func NewVotingProcessHandle(contractAddressHex string) (*ProcessHandle, error) {
	client, err := ethclient.Dial("https://gwdev1.vocdoni.net/web3")
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

	return PH, nil
}

func (ph *ProcessHandle) GetProcessData(pid [32]byte) (*vochain.NewProcessTxArgs, error) {
	processMeta, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		log.Errorf("Error fetching process metadata from Ethereum: %s", err)
	}
	log.Debugf("PROCESS META: %+v", processMeta)

	processTxArgs := new(vochain.NewProcessTxArgs)
	processTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	processTxArgs.EntityAddress = processMeta.EntityAddress.String()
	processTxArgs.MkRoot = processMeta.CensusMerkleRoot
	processTxArgs.NumberOfBlocks = processMeta.NumberOfBlocks
	processTxArgs.StartBlock = processMeta.StartBlock
	processTxArgs.EncryptionPrivateKey = processMeta.VoteEncryptionPrivateKey

	return processTxArgs, nil
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
