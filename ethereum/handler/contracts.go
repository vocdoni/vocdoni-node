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

// EthereumContractNames is the list of supported smart contracts names
var EthereumContractNames []string = []string{
	"processes",
	"namespaces",
	"erc20",
	"genesis",
	"results",
	"entities",
	"ensRegistry",
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
	case EthereumContractNames[6]:
		if ec.ABI, err = abi.JSON(strings.NewReader(contracts.EnsRegistryWithFallbackABI)); err != nil {
			return fmt.Errorf("cannot read ENS registry contract abi: %w", err)
		}
	}
	return nil
}

// InitContract resolves the contract address given an ENS domain and sets
// the contract ABI creating an artifact that allows to start interacting with
// the contract
func (ec *EthereumContract) InitContract(ctx context.Context, contractName string, ensRegistry common.Address, web3Client *ethclient.Client) error {
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
		return fmt.Errorf("couldn't set contract ABI: %w", err)
	}
	log.Infof("loaded contract %s at address: %s", ec.Domain, ec.Address)
	return nil
}
