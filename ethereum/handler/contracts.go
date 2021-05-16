package ethereumhandler

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/ethereum/contracts"
)

// EthereumContractNames is the list of supported smart contracts names
var EthereumContractNames []string = []string{
	"processes",
	"namespaces",
	"erc20",
	"genesis",
	"results",
	"entities",
}

// EthereumContract wraps basic smartcontract information
type EthereumContract struct {
	ABI             abi.ABI
	Bytecode        []byte
	Domain          string
	Address         common.Address
	ListenForEvents bool
}

// SetABI sets the ethereum contract ABI given a ethereum
// contract name defined at EthereumContractNames
func (ec *EthereumContract) SetABI(contractName string) error {
	var err error
	switch contractName {
	case EthereumContractNames[0]:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.ProcessesABI)); err != nil {
			return fmt.Errorf("cannot read processes contract abi: %w", err)
		}
	case EthereumContractNames[1]:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.NamespacesABI)); err != nil {
			return fmt.Errorf("cannot read namespace contract abi: %w", err)
		}
	case EthereumContractNames[2]:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.TokenStorageProofABI)); err != nil {
			return fmt.Errorf("cannot read token storage proof contract abi: %w", err)
		}
	case EthereumContractNames[3]:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.GenesisABI)); err != nil {
			return fmt.Errorf("cannot read genesis contract abi: %w", err)
		}
	case EthereumContractNames[4]:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.ResultsABI)); err != nil {
			return fmt.Errorf("cannot read results contract abi: %w", err)
		}
	case EthereumContractNames[5]:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.EntityResolverABI)); err != nil {
			return fmt.Errorf("cannot read entity resolver contract abi: %w", err)
		}
	}
	return nil
}
