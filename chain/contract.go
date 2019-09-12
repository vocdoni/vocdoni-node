package chain

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	votingProcess "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/log"
)

//These methods represent an exportable abstraction over raw contract bindings
//Use these methods, rather than those present in the contracts folder
type ProcessHandle struct {
	VotingProcess *votingProcess.VotingProcess
}

type ProcessMetadata struct {
	EntityAddress            common.Address
	Metadata                 string
	CensusMerkleRoot         string
	CensusMerkleTree         string
	VoteEncryptionPrivateKey string
	Canceled                 bool
}

// Constructor for proc_transactor on node
func NewVotingProcessHandle(contractAddressHex string) (*ProcessHandle, error) {
	var contractBackend = new(bind.ContractBackend)
	address := common.HexToAddress(contractAddressHex)

	votingProcess, err := votingProcess.NewVotingProcess(address, *contractBackend)
	if err != nil {
		log.Errorf("Error constructing votingProcess handle: %s", err)
		return new(ProcessHandle), err
	}
	PH := new(ProcessHandle)
	PH.VotingProcess = votingProcess
	//assign vp to something in e?
	return PH, nil
}

func (ph *ProcessHandle) Get(pid [32]byte) (ProcessMetadata, error) {
	return ph.VotingProcess.Get(nil, pid)
}

// node.proc_transactor.do_this()

// node.proc_transactor.do_that()

// Contructor for entity_transactor on node ?>

// node.entity_transactor.do_this()

// node.entity_transactor.do_that()
