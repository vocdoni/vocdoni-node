package ethereumhandler

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.vocdoni.io/dvote/ethereum/contracts"
	"go.vocdoni.io/dvote/log"
)

const (
	ContractNameENSregistry       = "ensRegistry"
	ContractNameENSresolver       = "ensResolver"
	ContractNameProcesses         = "processes"
	ContractNameNamespaces        = "namespaces"
	ContractNameTokenStorageProof = "erc20"
	ContractNameGenesis           = "genesis"
	ContractNameResults           = "results"
	ContractNameEntities          = "entities"
)

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
	case ContractNameProcesses:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.ProcessesABI)); err != nil {
			return fmt.Errorf("cannot read processes contract abi: %w", err)
		}
	case ContractNameNamespaces:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.NamespacesABI)); err != nil {
			return fmt.Errorf("cannot read namespace contract abi: %w", err)
		}
	case ContractNameTokenStorageProof:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.TokenStorageProofABI)); err != nil {
			return fmt.Errorf("cannot read token storage proof contract abi: %w", err)
		}
	case ContractNameGenesis:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.GenesisABI)); err != nil {
			return fmt.Errorf("cannot read genesis contract abi: %w", err)
		}
	case ContractNameResults:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.ResultsABI)); err != nil {
			return fmt.Errorf("cannot read results contract abi: %w", err)
		}
	case ContractNameEntities:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.EntityResolverABI)); err != nil {
			return fmt.Errorf("cannot read entity resolver contract abi: %w", err)
		}
	case ContractNameENSregistry:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.EnsRegistryWithFallbackABI)); err != nil {
			return fmt.Errorf("cannot read ENS registry contract abi: %w", err)
		}
	// entity resolver and public resolver have the same ABI
	case ContractNameENSresolver:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.EntityResolverABI)); err != nil {
			return fmt.Errorf("cannot read public resolver contract abi: %w", err)
		}
	}
	return nil
}

// InitContract resolves the contract address given an ENS domain and sets
// the contract ABI creating an artifact that allows to start interacting with
// the contract
func (ec *EthereumContract) InitContract(ctx context.Context, contractName string, ensRegistry common.Address, web3Client *ethclient.Client) error {
	// avoid resolve registry contract, this is the entry point for the ENS
	// and does not have a domain name
	if contractName == ContractNameENSregistry || contractName == ContractNameENSresolver {
		if err := ec.SetABI(contractName); err != nil {
			return fmt.Errorf("couldn't set contract %s ABI: %w", contractName, err)
		}
		return nil
	}
	var addr string
	var err error
	addr, err = EnsResolve(ctx, ensRegistry.Hex(), ec.Domain, web3Client)
	if err != nil {
		return fmt.Errorf("cannot resolve domain: %s, error: %w, trying again", ec.Domain, err)
	}
	if addr == "" {
		return fmt.Errorf("cannot resolve domain contract addresses")
	}
	ec.Address = common.HexToAddress(addr)
	if err := ec.SetABI(contractName); err != nil {
		return fmt.Errorf("couldn't set contract %s ABI: %w", contractName, err)
	}
	log.Infof("loaded contract %s at address: %s", ec.Domain, ec.Address)
	return nil
}
