package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	ethbind "github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
	"golang.org/x/net/idna"

	"gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

// VOTING PROCESS WRAPPER

// These methods represent an exportable abstraction over raw contract bindings`
// Use these methods, rather than those present in the contracts folder
type ProcessHandle struct {
	VotingProcess  *contracts.VotingProcess
	EthereumClient *ethclient.Client
}

// Constructor for proc_transactor on node
func NewVotingProcessHandle(contractAddressHex string, dialEndpoint string) (*ProcessHandle, error) {
	var err error
	PH := new(ProcessHandle)

	PH.EthereumClient, err = ethclient.Dial(dialEndpoint)
	if err != nil {
		log.Error(err)
	}
	address := common.HexToAddress(contractAddressHex)

	votingProcess, err := contracts.NewVotingProcess(address, PH.EthereumClient)
	if err != nil {
		log.Errorf("error constructing contracts handle: %s", err)
		return new(ProcessHandle), err
	}
	PH.VotingProcess = votingProcess
	return PH, nil
}

func (ph *ProcessHandle) ProcessTxArgs(ctx context.Context, pid [32]byte) (*types.NewProcessTx, error) {
	timeout, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: timeout}
	processMeta, err := ph.VotingProcess.Get(opts, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %s", err)
	}

	processTxArgs := new(types.NewProcessTx)
	processTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	eid, err := hex.DecodeString(util.TrimHex(processMeta.EntityAddress.String()))
	if err != nil {
		return nil, fmt.Errorf("error decoding entity address: %s", err)
	}
	processTxArgs.EntityID = fmt.Sprintf("%x", ethereum.HashRaw(eid))
	processTxArgs.MkRoot = processMeta.CensusMerkleRoot
	processTxArgs.MkURI = processMeta.CensusMerkleTree
	if processMeta.NumberOfBlocks != nil {
		processTxArgs.NumberOfBlocks = processMeta.NumberOfBlocks.Int64()
	}
	if processMeta.StartBlock != nil {
		processTxArgs.StartBlock = processMeta.StartBlock.Int64()
	}
	switch processMeta.ProcessType {
	case types.SnarkVote, types.PollVote, types.PetitionSign, types.EncryptedPoll:
		processTxArgs.ProcessType = processMeta.ProcessType
	}
	processTxArgs.Type = "newProcess"
	return processTxArgs, nil
}

func (ph *ProcessHandle) CancelProcessTxArgs(ctx context.Context, pid [32]byte) (*types.CancelProcessTx, error) {
	ctx, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: ctx}
	_, err := ph.VotingProcess.Get(opts, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %s", err)
	}
	cancelProcessTxArgs := new(types.CancelProcessTx)
	cancelProcessTxArgs.ProcessID = fmt.Sprintf("%x", pid)
	cancelProcessTxArgs.Type = "cancelProcess"
	return cancelProcessTxArgs, nil
}

func (ph *ProcessHandle) ProcessIndex(ctx context.Context, pid [32]byte) (*big.Int, error) {
	timeout, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: timeout}
	return ph.VotingProcess.GetProcessIndex(opts, pid)
}

func (ph *ProcessHandle) Oracles(ctx context.Context) ([]string, error) {
	timeout, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: timeout}
	return ph.VotingProcess.GetOracles(opts)
}

func (ph *ProcessHandle) Validators(ctx context.Context) ([]string, error) {
	timeout, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: timeout}
	return ph.VotingProcess.GetValidators(opts)
}

func (ph *ProcessHandle) Genesis(ctx context.Context) (string, error) {
	timeout, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: timeout}
	return ph.VotingProcess.GetGenesis(opts)
}

// ENS WRAPPER

// ENSCallerHandler contains the contracts and their addresses and an eth client
type ENSCallerHandler struct {
	// Registry public registry contract instance
	Registry *contracts.EnsRegistryWithFallbackCaller
	// Resolver resolver contract instance
	Resolver *contracts.EntityResolverCaller
	// EthereumClient is the client interacting with the ethereum node
	EthereumClient *ethclient.Client
	// PublicRegistryAddr public registry contract address
	PublicRegistryAddr string
	// ResolverAddr address resolved by calling Resolve() on Registry contract
	ResolverAddr string
}

func (e *ENSCallerHandler) close() {
	e.EthereumClient.Close()
}

// NewENSRegistryWithFallbackHandle connects to a web3 endpoint and creates an ENS public registry read only contact instance
func (e *ENSCallerHandler) NewENSRegistryWithFallbackHandle() (err error) {
	address := common.HexToAddress(e.PublicRegistryAddr)
	if e.Registry, err = contracts.NewEnsRegistryWithFallbackCaller(address, e.EthereumClient); err != nil {
		log.Errorf("error constructing contracts handle: %s", err)
		return err
	}
	return nil
}

// NewEntityResolverHandle connects to a web3 endpoint and creates an EntityResolver read only contact instance
func (e *ENSCallerHandler) NewEntityResolverHandle() (err error) {
	address := common.HexToAddress(e.ResolverAddr)
	if e.Resolver, err = contracts.NewEntityResolverCaller(address, e.EthereumClient); err != nil {
		log.Errorf("error constructing contracts handle: %s", err)
		return err
	}
	return nil
}

// Resolve if resolvePublicRegistry is set to true it will resolve
// the given namehash on the public registry. If false it will
// resolve the given namehash on a standard resolver
func (e *ENSCallerHandler) Resolve(ctx context.Context, nameHash [32]byte, resolvePublicRegistry bool) (string, error) {
	var err error
	var resolvedAddr common.Address
	timeout, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: timeout}
	if resolvePublicRegistry {
		resolvedAddr, err = e.Registry.Resolver(opts, nameHash)
	} else {
		resolvedAddr, err = e.Resolver.Addr(opts, nameHash)
	}
	if err != nil {
		return "", err
	}
	return resolvedAddr.String(), nil
}

// VotingProcessAddress gets the Voting process main contract address
func VotingProcessAddress(ctx context.Context, publicRegistryAddr, domain, ethEndpoint string) (string, error) {
	// normalize voting process domain name
	nh, err := NameHash(domain)
	if err != nil {
		return "", err
	}
	client, err := ethclient.Dial(ethEndpoint)
	if err != nil {
		log.Error(err)
	}
	ensCallerHandler := &ENSCallerHandler{
		PublicRegistryAddr: publicRegistryAddr,
		EthereumClient:     client,
	}
	defer ensCallerHandler.close()
	// create registry contract instance
	if err := ensCallerHandler.NewENSRegistryWithFallbackHandle(); err != nil {
		return "", err
	}
	// get resolver address from public registry
	ensCallerHandler.ResolverAddr, err = ensCallerHandler.Resolve(ctx, nh, true)
	if err != nil {
		return "", err
	}
	// create resolver contract instance
	if err := ensCallerHandler.NewEntityResolverHandle(); err != nil {
		return "", err
	}
	// get voting process addr from resolver
	votingProcessAddr, err := ensCallerHandler.Resolve(ctx, nh, false)
	if err != nil {
		return "", err
	}
	return votingProcessAddr, nil
}

// normalize normalizes a name according to the ENS standard
func Normalize(input string) (output string, err error) {
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

func NameHashPart(currentHash [32]byte, name string) (hash [32]byte, err error) {
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
func NameHash(name string) (hash [32]byte, err error) {
	if name == "" {
		return
	}
	normalizedName, err := Normalize(name)
	if err != nil {
		return
	}
	parts := strings.Split(normalizedName, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		if hash, err = NameHashPart(hash, parts[i]); err != nil {
			return
		}
	}
	return
}

const maxRetries = 30

// EnsResolve resolves the voting process contract address through the stardard ENS
func EnsResolve(ctx context.Context, ensRegistryAddr, ethDomain, w3uri string) (contractAddr string, err error) {
	for i := 0; i < maxRetries; i++ {
		contractAddr, err = VotingProcessAddress(ctx, ensRegistryAddr, ethDomain, w3uri)
		if err != nil {
			if strings.Contains(err.Error(), "no suitable peers available") {
				time.Sleep(time.Second)
				continue
			}
			err = fmt.Errorf("cannot get voting process contract: %s", err)
			return
		}
		log.Infof("loaded voting contract at address: %s", contractAddr)
		break
	}
	return
}

// ResolveEntityMetadataURL returns the metadata URL given an entityID
func ResolveEntityMetadataURL(ctx context.Context, ensRegistryAddr, entityID, ethEndpoint string) (string, error) {
	// normalize entity resolver domain name
	nh, err := NameHash(types.EntityResolverDomain)
	if err != nil {
		return "", err
	}
	client, err := ethclient.Dial(ethEndpoint)
	if err != nil {
		log.Error(err)
	}
	ensCallerHandler := &ENSCallerHandler{
		PublicRegistryAddr: ensRegistryAddr,
		EthereumClient:     client,
	}
	defer ensCallerHandler.close()
	// create registry contract instance
	if err := ensCallerHandler.NewENSRegistryWithFallbackHandle(); err != nil {
		return "", err
	}
	// get resolver address from public registry
	ensCallerHandler.ResolverAddr, err = ensCallerHandler.Resolve(ctx, nh, true)
	if err != nil {
		return "", err
	}
	// create resolver contract instance
	if err := ensCallerHandler.NewEntityResolverHandle(); err != nil {
		return "", err
	}
	// get entity metadata url from resolver
	eIDBytes, err := hex.DecodeString(entityID)
	if err != nil {
		return "", err
	}
	var eIDBytes32 [32]byte
	copy(eIDBytes32[:], eIDBytes)
	timeout, cancel := context.WithTimeout(ctx, types.EthereumWriteTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: timeout}
	metaURL, err := ensCallerHandler.Resolver.Text(opts, eIDBytes32, types.EntityMetaKey)
	if err != nil {
		return "", err
	}
	return metaURL, nil
}
