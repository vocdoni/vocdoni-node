package chain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	votingProcess "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

// These methods represent an exportable abstraction over raw contract bindings
// Use these methods, rather than those present in the contracts folder
type ProcessHandle struct {
	VotingProcess *votingProcess.VotingProcess
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
func NewVotingProcessHandle(contractAddressHex string, dialEndpoint string) (*ProcessHandle, error) {
	client, err := ethclient.Dial(dialEndpoint)
	if err != nil {
		log.Error(err)
	}
	address := common.HexToAddress(contractAddressHex)

	votingProcess, err := votingProcess.NewVotingProcess(address, client)
	if err != nil {
		log.Errorf("error constructing votingProcess handle: %s", err)
		return new(ProcessHandle), err
	}
	PH := new(ProcessHandle)
	PH.VotingProcess = votingProcess
	return PH, nil
}

func (ph *ProcessHandle) ProcessTxArgs(pid [32]byte) (*types.NewProcessTx, error) {
	processMeta, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process metadata from Ethereum: %s", err)
	}

	processTxArgs := new(types.NewProcessTx)
	processTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	processTxArgs.EntityID = processMeta.EntityAddress.String()
	processTxArgs.MkRoot = processMeta.CensusMerkleRoot
	processTxArgs.MkURI = processMeta.CensusMerkleTree
	processTxArgs.NumberOfBlocks = processMeta.NumberOfBlocks.Int64()
	processTxArgs.StartBlock = processMeta.StartBlock.Int64()
	processTxArgs.EncryptionPublicKeys = []string{processMeta.VoteEncryptionPrivateKey}
	switch processMeta.ProcessType {
	case "snark-vote", "poll-vote", "petition-sign":
		processTxArgs.ProcessType = processMeta.ProcessType
	}
	processTxArgs.Type = "newProcess"
	return processTxArgs, nil
}

func (ph *ProcessHandle) ProcessIndex(pid [32]byte) (*big.Int, error) {
	return ph.VotingProcess.GetProcessIndex(nil, pid)
}

func (ph *ProcessHandle) Oracles() ([]string, error) {
	return ph.VotingProcess.GetOracles(nil)
}

func (ph *ProcessHandle) Validators() ([]string, error) {
	return ph.VotingProcess.GetValidators(nil)
}

func (ph *ProcessHandle) Genesis() (string, error) {
	return ph.VotingProcess.GetGenesis(nil)
}
