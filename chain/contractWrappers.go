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

	"go.vocdoni.io/dvote/chain/contracts"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	models "go.vocdoni.io/proto/build/go/models"
)

// The following methods and structures represent an exportable abstraction over raw contract bindings
// Use these methods, rather than those present in the contracts folder

// VotingHandle wraps the Processes, Namespace and TokenStorageProof contracts and holds a reference to an ethereum client
type VotingHandle struct {
	VotingProcess     *contracts.Processes
	Namespace         *contracts.Namespaces
	TokenStorageProof *contracts.TokenStorageProof
	EthereumClient    *ethclient.Client
}

// Results represents the results received by a call to the processes smart contract getResults function
type Results struct {
	Tally  [][]uint32
	Height uint32
}

// Namespace represents the namespace received by a call to the processes smart contract getNamespace function
type Namespace struct {
	ChainId    string
	Genesis    string
	Validators []string
	Oracles    []common.Address
}

// NewVotingHandle initializes the Processes, Namespace and TokenStorageProof contracts creating a transactor using the ethereum client
// contractsAddress[0] -> Processes contract
// contractsAddress[1] -> Namespace contract
// contractsAddress[2] -> TokenStorageProof contract
func NewVotingHandle(contractsAddress []common.Address, dialEndpoint string) (*VotingHandle, error) {
	var err error
	ph := new(VotingHandle)
	// try connect to the client
	for i := 0; i < types.EthereumDialMaxRetry; i++ {
		ph.EthereumClient, err = ethclient.Dial(dialEndpoint)
		if err != nil || ph.EthereumClient == nil {
			log.Warnf("cannot create a client connection: (%s), trying again (%d of %d)", err, i+1, types.EthereumDialMaxRetry)
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
	if err != nil || ph.EthereumClient == nil {
		return nil, fmt.Errorf("cannot create a client connection: (%w), tried %d times", err, types.EthereumDialMaxRetry)
	}

	if ph.VotingProcess, err = contracts.NewProcesses(contractsAddress[0], ph.EthereumClient); err != nil {
		return new(VotingHandle), fmt.Errorf("error constructing processes contract transactor: %w", err)
	}
	if ph.Namespace, err = contracts.NewNamespaces(contractsAddress[1], ph.EthereumClient); err != nil {
		return nil, fmt.Errorf("error constructing namespace contract transactor: %w", err)
	}
	if ph.TokenStorageProof, err = contracts.NewTokenStorageProof(contractsAddress[2], ph.EthereumClient); err != nil {
		return nil, fmt.Errorf("error constructing token storage proof contract transactor: %w", err)
	}

	return ph, nil
}

// PROCESSES WRAPPER

// NewProcessTxArgs gets the info of a created process on the processes contract and creates a NewProcessTx instance
func (ph *VotingHandle) NewProcessTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte, namespace uint16) (*models.NewProcessTx, error) {
	// TODO: @jordipainan What to do with namespace?
	// get process info from the processes contract
	processMeta, err := ph.VotingProcess.Get(&ethbind.CallOpts{Context: ctx}, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %w", err)
	}
	// create NewProcessTx
	processTxArgs := new(models.NewProcessTx)
	processData := new(models.Process)

	// check status ready or paused
	status := models.ProcessStatus(processMeta.Status + 1) // +1 required to match with solidity enum
	if status != models.ProcessStatus_READY && status != models.ProcessStatus_PAUSED {
		return nil, fmt.Errorf("invalid process status on process creation: %d", status)
	}
	processData.Status = status
	// process id
	processData.ProcessId = pid[:]
	// entity id
	// for evm censuses the entity id is the snapshoted contract address
	if processData.EntityId, err = hex.DecodeString(util.TrimHex(processMeta.EntityAddress.String())); err != nil {
		return nil, fmt.Errorf("error decoding entity address: %w", err)
	}
	// census mkroot
	processData.CensusRoot, err = hex.DecodeString(util.TrimHex(processMeta.MetadataCensusMerkleRootCensusMerkleTree[1]))
	if err != nil {
		return nil, fmt.Errorf("cannot decode merkle root: %w", err)
	}
	// census origin
	cOrigin := processMeta.ModeEnvelopeTypeCensusOrigin[2] + 1 // +1 required to match with solidity enum
	if cOrigin > types.ProcessesContractMaxCensusOrigins {
		return nil, fmt.Errorf("invalid census origin: %d", processMeta.ModeEnvelopeTypeCensusOrigin[2])
	}
	processData.CensusOrigin = models.CensusOrigin(cOrigin)
	evmCensus := processData.CensusOrigin == models.CensusOrigin_OFF_CHAIN_TREE
	// census mkuri, only for off chain censuses
	if evmCensus {
		processMeta.MetadataCensusMerkleRootCensusMerkleTree[2] = util.TrimHex(processMeta.MetadataCensusMerkleRootCensusMerkleTree[2])
		processData.CensusURI = &processMeta.MetadataCensusMerkleRootCensusMerkleTree[2]
	}
	// start and end blocks
	if processMeta.StartBlockBlockCount[0] > types.ProcessesContractMinStartBlock {
		processData.StartBlock = processMeta.StartBlockBlockCount[0]
	}
	if processMeta.StartBlockBlockCount[1] > types.ProcessesContractMinBlockCount {
		processData.BlockCount = processMeta.StartBlockBlockCount[1]
	}
	// supported last 4 chars as the smart contract does
	// process mode
	if processData.Mode, err = extractProcessMode(processMeta.ModeEnvelopeTypeCensusOrigin[0]); err != nil {
		return nil, err
	}
	// envelope type
	if processData.EnvelopeType, err = extractEnvelopeType(processMeta.ModeEnvelopeTypeCensusOrigin[1]); err != nil {
		return nil, err
	}
	// question index
	qIndex := uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[0])
	processData.QuestionIndex = &qIndex
	// question count
	qCount := uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[1])
	processData.QuestionIndex = &qCount
	// max count
	processData.VoteOptions = &models.ProcessVoteOptions{
		// mac count
		MaxCount: uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[2]),
		// max value
		MaxValue: uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[3]),
		// max vote overwrites
		MaxVoteOverwrites: uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[4]),
		// max total cost
		MaxTotalCost: uint32(processMeta.MaxTotalCostCostExponentNamespace[0]),
		// cost exponent
		CostExponent: uint32(processMeta.MaxTotalCostCostExponentNamespace[1]),
	}
	// namespace
	processData.Namespace = uint32(processMeta.MaxTotalCostCostExponentNamespace[2])

	// if EVM census, eth index slot from the ERC20Registry contract
	if processData.CensusOrigin != models.CensusOrigin_OFF_CHAIN_TREE {
		// evm block height not required here, will be fetched by each user when generating the vote
		// index slot
		idxSlot, err := ph.TokenStorageProof.GetBalanceMappingPosition(&ethbind.CallOpts{Context: ctx}, processMeta.EntityAddress)
		if err != nil {
			return nil, fmt.Errorf("error fetching token index slot from Ethereum: %w", err)
		}
		iSlot32 := uint32(idxSlot.Uint64())
		processData.EthIndexSlot = &iSlot32
	}

	// set tx type (outer type used by the vochain tx type)
	processTxArgs.Txtype = models.TxType_NEW_PROCESS
	processTxArgs.Process = processData
	return processTxArgs, nil
}

func extractEnvelopeType(envelopeType uint8) (*models.EnvelopeType, error) {
	if envelopeType > types.ProcessesContractMaxEnvelopeType {
		return nil, fmt.Errorf("invalid process mode: (%d)", envelopeType)
	}
	return &models.EnvelopeType{
		Serial:         envelopeType&byte(0b00000001) > 0,
		Anonymous:      envelopeType&byte(0b00000010) > 0,
		EncryptedVotes: envelopeType&byte(0b00000100) > 0,
		UniqueValues:   envelopeType&byte(0b00001000) > 0,
	}, nil
}

func extractProcessMode(processMode uint8) (*models.ProcessMode, error) {
	if processMode > types.ProcessesContractMaxProcessMode {
		return nil, fmt.Errorf("invalid process mode: (%d)", processMode)
	}
	return &models.ProcessMode{
		AutoStart:         processMode&byte(0b00000001) > 0,
		Interruptible:     processMode&byte(0b00000010) > 0,
		DynamicCensus:     processMode&byte(0b00000100) > 0,
		EncryptedMetaData: processMode&byte(0b00001000) > 0,
	}, nil
}

// SetStatusTxArgs returns a SetProcessTx instance
func (ph *VotingHandle) SetStatusTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte, namespace uint16, status uint8) (*models.SetProcessTx, error) {
	status++ // +1 for matching with ethevent uint8
	processData, err := ph.VotingProcess.Get(&ethbind.CallOpts{Context: ctx}, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %w", err)
	}
	if processData.Status == status {
		return nil, fmt.Errorf("status should differ: %w", err)
	}
	// create setProcessTx
	setprocessTxArgs := new(models.SetProcessTx)
	// process id
	setprocessTxArgs.ProcessId = pid[:]
	// process status
	processStatus := models.ProcessStatus(uint32(status))
	setprocessTxArgs.Status = &processStatus
	// TODO: @jordipainan namespace not used
	setprocessTxArgs.Txtype = models.TxType_SET_PROCESS_STATUS

	return setprocessTxArgs, nil
}

// SetCensusTxArgs
func (ph *VotingHandle) SetCensusTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte) (*models.SetProcessTx, error) {
	return nil, fmt.Errorf("not implemented")
}

// SetResultsTxArgs
func (ph *VotingHandle) SetResultsTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte) (*models.SetProcessTx, error) {
	return nil, fmt.Errorf("not implemented")
}

// IncrementQuestionIndexTxArgs
func (ph *VotingHandle) IncrementQuestionIndexTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte) (*models.SetProcessTx, error) {
	return nil, fmt.Errorf("not implemented")
}

// SetNamespaceAddressTxArgs
func (ph *VotingHandle) SetNamespaceAddressTxArgs(ctx context.Context) (*models.AdminTx, error) {
	return nil, fmt.Errorf("not implemented")
}

// EntityProcessCount returns the entity process count given an entity address
func (ph *VotingHandle) EntityProcessCount(ctx context.Context, eid common.Address) (entityProcessCount *big.Int, err error) {
	if entityProcessCount, err = ph.VotingProcess.GetEntityProcessCount(&ethbind.CallOpts{Context: ctx}, eid); err != nil {
		err = fmt.Errorf("cannot get entity process count: %w", err)
	}
	return
}

// EntityNextProcessID returns the next process id of a given entity address
func (ph *VotingHandle) EntityNextProcessID(ctx context.Context, eid common.Address, namespace uint16) (entityNextProcessID [types.EntityIDsizeV2]byte, err error) {
	if entityNextProcessID, err = ph.VotingProcess.GetNextProcessId(&ethbind.CallOpts{Context: ctx}, eid, namespace); err != nil {
		err = fmt.Errorf("cannot get entity next process id: %w", err)
	}
	return
}

// ProcessParamsSignature returns the signature of the process parameters
func (ph *VotingHandle) ProcessParamsSignature(ctx context.Context, pid [types.ProcessIDsize]byte) (processParamsSignature [types.ProcessesParamsSignatureSize]byte, err error) {
	if processParamsSignature, err = ph.VotingProcess.GetParamsSignature(&ethbind.CallOpts{Context: ctx}, pid); err != nil {
		err = fmt.Errorf("cannot get process params signature: %w", err)
	}
	return
}

// ProcessResults returns the results for a given process
func (ph *VotingHandle) ProcessResults(ctx context.Context, pid [types.ProcessIDsize]byte) (processResults Results, err error) {
	if processResults, err = ph.VotingProcess.GetResults(&ethbind.CallOpts{Context: ctx}, pid); err != nil {
		err = fmt.Errorf("cannot get process results: %w", err)
	}
	return
}

// ProcessCreationInstance returns the address of the processes contract instance where the process was created
func (ph *VotingHandle) ProcessCreationInstance(ctx context.Context, pid [types.ProcessIDsize]byte) (processCreationInstance common.Address, err error) {
	if processCreationInstance, err = ph.VotingProcess.GetCreationInstance(&ethbind.CallOpts{Context: ctx}, pid); err != nil {
		err = fmt.Errorf("cannot get process creation instance: %w", err)
	}
	return
}

// NAMESPACE WRAPPER

// GetNamespace returns the chainID, genesis, validators and oracles of a given namespace
func (ph *VotingHandle) GetNamespace(ctx context.Context, namespace uint16) (*Namespace, error) {
	ns, err := ph.Namespace.GetNamespace(&ethbind.CallOpts{Context: ctx}, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get namespace: %w", err)
	}
	return &Namespace{
		ChainId:    ns.ChainId,
		Genesis:    ns.Genesis,
		Validators: ns.Validators,
		Oracles:    ns.Oracles,
	}, nil
}

// TOKEN STORAGE PROOF WRAPPER

// IsTokenRegistered returns true if a token represented by the given address is registered on the token storage proof contract
func (ph *VotingHandle) IsTokenRegistered(ctx context.Context, address common.Address) (bool, error) {
	return ph.TokenStorageProof.IsRegistered(&ethbind.CallOpts{Context: ctx}, address)
}

// GetTokenBalanceMappingPosition returns the balance mapping position given a token address
func (ph *VotingHandle) GetTokenBalanceMappingPosition(ctx context.Context, address common.Address) (*big.Int, error) {
	return ph.TokenStorageProof.GetBalanceMappingPosition(&ethbind.CallOpts{Context: ctx}, address)
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
		return fmt.Errorf("cannot create ENS Registry contract instance: %w", err)
	}
	return nil
}

// NewEntityResolverHandle connects to a web3 endpoint and creates an EntityResolver read only contact instance
func (e *ENSCallerHandler) NewEntityResolverHandle() (err error) {
	address := common.HexToAddress(e.ResolverAddr)
	if e.Resolver, err = contracts.NewEntityResolverCaller(address, e.EthereumClient); err != nil {
		log.Errorf("error constructing contracts handle: %s", err)
		return fmt.Errorf("cannot create ENS Resolver contract instance: %w", err)
	}
	return nil
}

// Resolve if resolvePublicRegistry is set to true it will resolve
// the given namehash on the public registry. If false it will
// resolve the given namehash on a standard resolver
func (e *ENSCallerHandler) Resolve(ctx context.Context, nameHash [32]byte, resolvePublicRegistry bool) (string, error) {
	var err error
	var resolvedAddr common.Address
	tctx, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	if resolvePublicRegistry {
		resolvedAddr, err = e.Registry.Resolver(&ethbind.CallOpts{Context: tctx}, nameHash)
	} else {
		resolvedAddr, err = e.Resolver.Addr(&ethbind.CallOpts{Context: tctx}, nameHash)

	}
	if err != nil {
		return "", fmt.Errorf("cannot resolve contract address: %w", err)
	}
	return resolvedAddr.String(), nil
}

// ENSAddress gets a smart contract address trough the ENS given a public regitry address and its domain
func ENSAddress(ctx context.Context, publicRegistryAddr, domain, ethEndpoint string) (string, error) {
	// normalize voting process domain name
	nh, err := NameHash(domain)
	if err != nil {
		return "", fmt.Errorf("cannot get ENS address of the given domain: %w", err)
	}
	var client *ethclient.Client
	for i := 0; i < types.EthereumDialMaxRetry; i++ {
		client, err = ethclient.Dial(ethEndpoint)
		if err != nil || client == nil {
			log.Warnf("cannot create a client connection: %s, trying again... %d of %d", err, i+1, types.EthereumDialMaxRetry)
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
	if err != nil || client == nil {
		log.Warnf("cannot create a client connection: %s, tried %d times.", err, types.EthereumDialMaxRetry)
		return "", fmt.Errorf("cannot create client connection: %w", err)
	}

	ensCallerHandler := &ENSCallerHandler{
		PublicRegistryAddr: publicRegistryAddr,
		EthereumClient:     client,
	}
	defer ensCallerHandler.close()
	// create registry contract instance
	if err := ensCallerHandler.NewENSRegistryWithFallbackHandle(); err != nil {
		return "", fmt.Errorf("cannot get ENS address of the given domain: %w", err)
	}
	// get resolver address from public registry
	ensCallerHandler.ResolverAddr, err = ensCallerHandler.Resolve(ctx, nh, true)
	if err != nil {
		return "", fmt.Errorf("cannot get ENS address of the given domain: %w", err)
	}
	// create resolver contract instance
	if err := ensCallerHandler.NewEntityResolverHandle(); err != nil {
		return "", fmt.Errorf("cannot get ENS address of the given domain: %w", err)
	}
	// get voting process addr from resolver
	contractAddr, err := ensCallerHandler.Resolve(ctx, nh, false)
	if err != nil {
		return "", fmt.Errorf("cannot get ENS address of the given domain: %w", err)
	}
	return contractAddr, nil
}

// Normalize normalizes a name according to the ENS standard
func Normalize(input string) (output string, err error) {
	p := idna.New(idna.MapForLookup(), idna.StrictDomainName(false), idna.Transitional(false))
	output, err = p.ToUnicode(input)
	if err != nil {
		err = fmt.Errorf("cannot convert input to Unicode: %w", err)
	}
	// If the name started with a period then ToUnicode() removes it, but we want to keep it
	if strings.HasPrefix(input, ".") && !strings.HasPrefix(output, ".") {
		output = "." + output
	}
	return
}

// NameHashPart returns a unique hash generated for any valid domain name
func NameHashPart(currentHash [32]byte, name string) (hash [32]byte, err error) {
	sha := sha3.NewLegacyKeccak256()
	if _, err = sha.Write(currentHash[:]); err != nil {
		err = fmt.Errorf("nameHashPart: cannot generate sha3 of the given hash: %w", err)
		return
	}
	nameSha := sha3.NewLegacyKeccak256()
	if _, err = nameSha.Write([]byte(name)); err != nil {
		err = fmt.Errorf("nameHashPart: cannot generate sha3 of the given name: %w", err)
		return
	}
	nameHash := nameSha.Sum(nil)
	if _, err = sha.Write(nameHash); err != nil {
		err = fmt.Errorf("nameHashPart: cannot generate sha3 of the computed namehash: %w", err)
		return
	}
	sha.Sum(hash[:0])
	return
}

// NameHash generates a hash from a name that can be used to look up the name in ENS
func NameHash(name string) (hash [32]byte, err error) {
	if name == "" {
		err = fmt.Errorf("nameHash: cannot create namehash of the given name")
		return
	}
	normalizedName, err := Normalize(name)
	if err != nil {
		err = fmt.Errorf("nameHash: cannot normalize the given name: %w", err)
		return
	}
	parts := strings.Split(normalizedName, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		if hash, err = NameHashPart(hash, parts[i]); err != nil {
			err = fmt.Errorf("nameHash: cannot generate name hash part: %w", err)
			return
		}
	}
	return
}

const maxRetries = 30

// EnsResolve resolves smart contract addresses through the stardard ENS
func EnsResolve(ctx context.Context, ensRegistryAddr, ethDomain, w3uri string) (contractAddr string, err error) {
	for i := 0; i < maxRetries; i++ {
		contractAddr, err = ENSAddress(ctx, ensRegistryAddr, ethDomain, w3uri)
		if err != nil {
			if strings.Contains(err.Error(), "no suitable peers available") {
				time.Sleep(time.Second)
				continue
			}
			err = fmt.Errorf("cannot get contract address: %w", err)
			return
		}
		log.Infof("loaded contract at address: %s", contractAddr)
		break
	}
	return
}

// ResolveEntityMetadataURL returns the metadata URL given an entityID
func ResolveEntityMetadataURL(ctx context.Context, ensRegistryAddr, entityID, ethEndpoint string) (string, error) {
	// normalize entity resolver domain name
	nh, err := NameHash(types.EntityResolverDomain)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	var client *ethclient.Client
	for i := 0; i < types.EthereumDialMaxRetry; i++ {
		client, err = ethclient.Dial(ethEndpoint)
		if err != nil || client == nil {
			log.Warnf("cannot create a client connection: %s, trying again... %d of %d", err, i+1, types.EthereumDialMaxRetry)
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
	if err != nil || client == nil {
		log.Errorf("cannot create a client connection: %s, tried %d times.", err, types.EthereumDialMaxRetry)
		return "", fmt.Errorf("cannot resolve entity metadata URL, cannot create a client connection: %w", err)
	}
	ensCallerHandler := &ENSCallerHandler{
		PublicRegistryAddr: ensRegistryAddr,
		EthereumClient:     client,
	}
	defer ensCallerHandler.close()
	// create registry contract instance
	if err := ensCallerHandler.NewENSRegistryWithFallbackHandle(); err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	// get resolver address from public registry
	ensCallerHandler.ResolverAddr, err = ensCallerHandler.Resolve(ctx, nh, true)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	// create resolver contract instance
	if err := ensCallerHandler.NewEntityResolverHandle(); err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	// get entity metadata url from resolver
	eIDBytes, err := hex.DecodeString(entityID)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	var eIDBytes32 [32]byte
	copy(eIDBytes32[:], eIDBytes)
	tctx, cancel := context.WithTimeout(ctx, types.EthereumWriteTimeout)
	defer cancel()
	metaURL, err := ensCallerHandler.Resolver.Text(&ethbind.CallOpts{Context: tctx}, eIDBytes32, types.EntityMetaKey)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	return metaURL, nil
}
