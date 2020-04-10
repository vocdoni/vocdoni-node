package chain

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
	"golang.org/x/net/idna"

	"gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

// VOTING PROCESS WRAPPER

// These methods represent an exportable abstraction over raw contract bindings`
// Use these methods, rather than those present in the contracts folder
type ProcessHandle struct {
	VotingProcess *contracts.VotingProcess
}

// Constructor for proc_transactor on node
func NewVotingProcessHandle(contractAddressHex string, dialEndpoint string) (*ProcessHandle, error) {
	client, err := ethclient.Dial(dialEndpoint)
	if err != nil {
		log.Error(err)
	}
	address := common.HexToAddress(contractAddressHex)

	votingProcess, err := contracts.NewVotingProcess(address, client)
	if err != nil {
		log.Errorf("error constructing contracts handle: %s", err)
		return new(ProcessHandle), err
	}
	PH := new(ProcessHandle)
	PH.VotingProcess = votingProcess
	return PH, nil
}

func (ph *ProcessHandle) ProcessTxArgs(pid [32]byte) (*types.NewProcessTx, error) {
	processMeta, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %s", err)
	}

	processTxArgs := new(types.NewProcessTx)
	processTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	eid, err := hex.DecodeString(util.TrimHex(processMeta.EntityAddress.String()))
	if err != nil {
		return nil, fmt.Errorf("error decoding entity address: %s", err)
	}
	processTxArgs.EntityID = fmt.Sprintf("%x", signature.HashRaw(string(eid)))
	processTxArgs.MkRoot = processMeta.CensusMerkleRoot
	processTxArgs.MkURI = processMeta.CensusMerkleTree
	if processMeta.NumberOfBlocks != nil {
		processTxArgs.NumberOfBlocks = processMeta.NumberOfBlocks.Int64()
	}
	if processMeta.StartBlock != nil {
		processTxArgs.StartBlock = processMeta.StartBlock.Int64()
	}
	processTxArgs.EncryptionPublicKeys = []string{processMeta.VoteEncryptionPrivateKey}
	switch processMeta.ProcessType {
	case "snark-vote", "poll-vote", "petition-sign":
		processTxArgs.ProcessType = processMeta.ProcessType
	}
	processTxArgs.Type = "newProcess"
	return processTxArgs, nil
}

func (ph *ProcessHandle) CancelProcessTxArgs(pid [32]byte) (*types.CancelProcessTx, error) {
	_, err := ph.VotingProcess.Get(nil, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %s", err)
	}
	cancelProcessTxArgs := new(types.CancelProcessTx)
	cancelProcessTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	cancelProcessTxArgs.Type = "cancelProcess"
	return cancelProcessTxArgs, nil
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

// ENS WRAPPER

// ENSCallerHandler contains the contracts and their addresses and an eth client
type ENSCallerHandler struct {
	// Registry public registry contract instance
	Registry *contracts.EnsRegistryWithFallbackCaller
	// Resolver resolver contract instance
	Resolver *contracts.EntityResolverCaller
	// EthEndpoint ethereum web3 endpoint to dial
	EthEndpoint string
	// PublicRegistryAddr public registry contract address
	PublicRegistryAddr string
	// ResolverAddr address resolved by calling Resolve() on Registry contract
	ResolverAddr string
}

// NewENSRegistryWithFallbackHandle connects to a web3 endpoint and creates an ENS public registry read only contact instance
func (e *ENSCallerHandler) NewENSRegistryWithFallbackHandle() {
	ethclient, err := ethclient.Dial(e.EthEndpoint)
	if err != nil {
		log.Warnf("cannot connect to ethereum web3 endpoint: %s", err)
	}
	address := common.HexToAddress(e.PublicRegistryAddr)
	e.Registry, err = contracts.NewEnsRegistryWithFallbackCaller(address, ethclient)
	if err != nil {
		log.Errorf("error constructing contracts handle: %s", err)
	}
}

// NewEntityResolverHandle connects to a web3 endpoint and creates an EntityResolver read only contact instance
func (e *ENSCallerHandler) NewEntityResolverHandle() {
	ethclient, err := ethclient.Dial(e.EthEndpoint)
	if err != nil {
		log.Warnf("cannot connect to ethereum web3 endpoint: %s", err)
	}
	address := common.HexToAddress(e.ResolverAddr)
	e.Resolver, err = contracts.NewEntityResolverCaller(address, ethclient)
	if err != nil {
		log.Errorf("error constructing contracts handle: %s", err)
	}
}

// Resolve if resolvePublicRegistry is set to true it will resolve
// the given namehash on the public registry. If false it will
// resolve the given namehash on a standard resolver
func (e *ENSCallerHandler) Resolve(nameHash [32]byte, resolvePublicRegistry bool) (string, error) {
	var err error
	var resolvedAddr common.Address
	if resolvePublicRegistry {
		resolvedAddr, err = e.Registry.Resolver(nil, nameHash)
	} else {
		resolvedAddr, err = e.Resolver.Addr(nil, nameHash)
	}
	if err != nil {
		return "", err
	}
	return resolvedAddr.String(), nil
}

// VotingProcessAddress gets the Voting process main contract address
func VotingProcessAddress(publicRegistryAddr, domain string, ethEndpoint string) (string, error) {
	// normalize voting process domain name
	nh, err := nameHash(domain)
	if err != nil {
		return "", err
	}
	ensCallerHandler := &ENSCallerHandler{
		EthEndpoint:        ethEndpoint,
		PublicRegistryAddr: publicRegistryAddr,
	}
	// create registry contract instance
	ensCallerHandler.NewENSRegistryWithFallbackHandle()
	// get resolver address from public registry
	ensCallerHandler.ResolverAddr, err = ensCallerHandler.Resolve(nh, true)
	if err != nil {
		return "", err
	}
	// create resolver contract instance
	ensCallerHandler.NewEntityResolverHandle()
	// get voting process addr from resolver
	votingProcessAddr, err := ensCallerHandler.Resolve(nh, false)
	if err != nil {
		return "", err
	}
	return votingProcessAddr, nil
}

// normalize normalizes a name according to the ENS standard
func normalize(input string) (output string, err error) {
	p := idna.New(idna.MapForLookup(), idna.StrictDomainName(false), idna.Transitional(false))
	output, err = p.ToUnicode(input)
	if err != nil {
		return
	}
	// If the name started with a period then ToUnicode() removes it, but we want to keep it
	if strings.HasPrefix(input, ".") && !strings.HasPrefix(output, ".") {
		output = "." + output
	}
	return
}

func nameHashPart(currentHash [32]byte, name string) (hash [32]byte, err error) {
	sha := sha3.NewLegacyKeccak256()
	if _, err = sha.Write(currentHash[:]); err != nil {
		return
	}
	nameSha := sha3.NewLegacyKeccak256()
	if _, err = nameSha.Write([]byte(name)); err != nil {
		return
	}
	nameHash := nameSha.Sum(nil)
	if _, err = sha.Write(nameHash); err != nil {
		return
	}
	sha.Sum(hash[:0])
	return
}

// nameHash generates a hash from a name that can be used to look up the name in ENS
func nameHash(name string) (hash [32]byte, err error) {
	if name == "" {
		return
	}
	normalizedName, err := normalize(name)
	if err != nil {
		return
	}
	parts := strings.Split(normalizedName, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		if hash, err = nameHashPart(hash, parts[i]); err != nil {
			return
		}
	}
	return
}
