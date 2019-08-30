package chain

import (
		"gitlab.com/vocdoni/go-dvote/log"
		"gitlab.com/vocdoni/go-dvote/chain/contracts"
		"github.com/ethereum/go-ethereum/accounts/abi/bind"
		"github.com/ethereum/go-ethereum/common"
)

//These methods represent an exportable abstraction over raw contract bindings
//Use these methods, rather than those present in the contracts folder

// Constructor for proc_transactor on node
func (e *EthChainContext) NewVotingProcessHandle(contractAddressHex string) error {
	var contractBackend = new(bind.ContractBackend)
	address := common.HexToAddress(contractAddressHex)

	votingProcess, err := votingProcess.NewVotingProcess(address, *contractBackend)	
	if err != nil {
		log.Errorf("Error constructing votingProcess handle: %s", err)
	}
	//assign vp to something in e?
	_ = votingProcess
	return nil
}

// node.proc_transactor.do_this()

// node.proc_transactor.do_that()


// Contructor for entity_transactor on node ?>

// node.entity_transactor.do_this()

// node.entity_transactor.do_that()