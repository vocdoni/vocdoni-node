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

	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	"gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

// The following methods and structures represent an exportable abstraction over raw contract bindings
// Use these methods, rather than those present in the contracts folder

// VOTING PROCESS WRAPPER

// ProcessHandle wraps the Processes, Namespace and TokenStorageProof contracts and holds a reference to an ethereum client
type ProcessHandle struct {
	VotingProcess     *contracts.VotingProcess
	Namespace         *contracts.Namespace
	TokenStorageProof *contracts.TokenStorageProof
	EthereumClient    *ethclient.Client
}

// NewProcessHandle initializes the Processes, Namespace and TokenStorageProof contracts creating a transactor using the ethereum client
func NewProcessHandle(votingContractAddressHex, namespaceContractAddressHex, tokenStorageProofAddressHex string, dialEndpoint string) (*ProcessHandle, error) {
	var err error
	PH := new(ProcessHandle)
	// try connect to the client
	for i := 0; i < types.EthereumDialMaxRetry; i++ {
		PH.EthereumClient, err = ethclient.Dial(dialEndpoint)
		if err != nil || PH.EthereumClient == nil {
			log.Warnf("cannot create a client connection: (%s), trying again (%d of %d)", err, i+1, types.EthereumDialMaxRetry)
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
	if err != nil || PH.EthereumClient == nil {
		log.Fatalf("cannot create a client connection: (%s), tried %d times.", err, types.EthereumDialMaxRetry)
	}
	// processes contract transactor
	vAddress := common.HexToAddress(votingContractAddressHex)
	votingProcess, err := contracts.NewVotingProcess(vAddress, PH.EthereumClient)
	if err != nil {
		log.Errorf("error constructing processes contract transactor: %s", err)
		return new(ProcessHandle), err
	}
	// namespace contract transactor
	nAddress := common.HexToAddress(namespaceContractAddressHex)
	namespace, err := contracts.NewNamespace(nAddress, PH.EthereumClient)
	if err != nil {
		log.Errorf("error constructing namespace contract transactor: %s", err)
		return nil, err
	}
	// token storage proof transactor
	tspAddress := common.HexToAddress(tokenStorageProofAddressHex)
	tokenStorageProof, err := contracts.NewTokenStorageProof(tspAddress, PH.EthereumClient)
	if err != nil {
		log.Errorf("error constructing token storage proof contract transactor: %s", err)
		return nil, err
	}

	PH.VotingProcess = votingProcess
	PH.Namespace = namespace
	PH.TokenStorageProof = tokenStorageProof

	return PH, nil
}

// ProcessTxArgs gets the info of a created process on the processes contract and creates a NewProcessTx instance
func (ph *ProcessHandle) ProcessTxArgs(ctx context.Context, pid [32]byte) (*models.NewProcessTx, error) {
	// get process info from the processes contract
	opts := &ethbind.CallOpts{Context: ctx}
	processMeta, err := ph.VotingProcess.Get(opts, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %s", err)
	}
	// create NewProcessTx
	processTxArgs := new(models.NewProcessTx)
	processData := new(models.Process)
	// pid
	processData.ProcessId = pid[:]
	eid, err := hex.DecodeString(util.TrimHex(processMeta.EntityAddress.String()))
	if err != nil {
		return nil, fmt.Errorf("error decoding entity address: %s", err)
	}
	// entity id
	// for evm censuses the entity id is the snapshoted contract address
	processData.EntityId = eid
	// census mkroot
	processData.CensusMkRoot, err = hex.DecodeString(processMeta.MetadataCensusMerkleRootCensusMerkleTree[1])
	if err != nil {
		return nil, fmt.Errorf("cannot decode merkle root: (%s)", err)
	}
	// census mkuri
	processData.CensusMkURI = &processMeta.MetadataCensusMerkleRootCensusMerkleTree[2]
	// start and end blocks
	if processMeta.StartBlockBlockCount[0] > 0 {
		processData.StartBlock = processMeta.StartBlockBlockCount[0]
	}
	if processMeta.StartBlockBlockCount[1] > 0 {
		processData.BlockCount = processMeta.StartBlockBlockCount[1]
	}
	// process mode, envelope type and census origin
	var modeType [2]uint8
	copy(modeType[:], processMeta.ModeEnvelopeTypeCensusOrigin[:2])
	// process mode and envelope type (legacy processType)
	switch modeType {
	case types.SnarkVote:
		processData.ProcessType = types.SnarkVoteStr
		processData.Mode.AutoStart = true
		processData.Mode.Interruptible = true
		processData.EnvelopeType.Anonymous = true
		processData.EnvelopeType.EncryptedVotes = true
	case types.PollVote:
		processData.ProcessType = types.PollVoteStr
		processData.Mode.AutoStart = true
		processData.Mode.Interruptible = true
	case types.PetitionSign:
		processData.ProcessType = types.PetitionSignStr
		processData.Mode.AutoStart = true
		processData.Mode.Interruptible = true
	case types.EncryptedPoll:
		processData.ProcessType = types.EncryptedPollStr
		processData.Mode.AutoStart = true
		processData.Mode.Interruptible = true
		processData.EnvelopeType.EncryptedVotes = true
	default:
		return nil, fmt.Errorf("invalid process type: %v", modeType)
	}
	// census origin
	processData.CensusOrigin = models.CensusOrigin(processMeta.ModeEnvelopeTypeCensusOrigin[2] + 1) // +1 required to match with solidity enum
	// status
	processData.Status = models.ProcessStatus(processMeta.Status)
	// question index
	var qIndex = new(uint32)
	*qIndex = uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[0])
	processData.QuestionIndex = qIndex
	// question count
	var qCount = new(uint32)
	*qCount = uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[1])
	processData.QuestionIndex = qCount
	// max count
	processData.VoteOptions.MaxCount = uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[2])
	// max value
	processData.VoteOptions.MaxValue = uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[3])
	// max vote overwrites
	processData.VoteOptions.MaxVoteOverwrites = uint32(processMeta.QuestionIndexQuestionCountMaxCountMaxValueMaxVoteOverwrites[4])
	// max total cost
	processData.VoteOptions.MaxTotalCost = uint32(processMeta.MaxTotalCostCostExponentNamespace[0])
	// cost exponent
	processData.VoteOptions.CostExponent = uint32(processMeta.MaxTotalCostCostExponentNamespace[1])
	// namespace
	processData.Namespace = uint32(processMeta.MaxTotalCostCostExponentNamespace[2])

	// if EVM census, eth index slot from the ERC20Registry contract
	if processData.CensusOrigin == 1 {
		// evm block height not required here, will be fetched by each user when generating the vote
		// index slot
		opts2 := &ethbind.CallOpts{Context: ctx}
		idxSlot, err := ph.TokenStorageProof.GetBalanceMappingPosition(opts2, processMeta.EntityAddress)
		if err != nil {
			return nil, fmt.Errorf("error fetching token index slot from Ethereum: %s", err)
		}
		var iSlot32 = new(uint32)
		*iSlot32 = uint32(idxSlot.Uint64())
		processData.EthIndexSlot = iSlot32
	}

	// set tx type (outer type used by the vochain tx type)
	processTxArgs.Txtype = models.TxType_NEW_PROCESS
	processTxArgs.Process = processData
	return processTxArgs, nil
}

func (ph *ProcessHandle) CancelProcessTxArgs(ctx context.Context, pid [32]byte) (*models.CancelProcessTx, error) {
	opts := &ethbind.CallOpts{Context: ctx}
	_, err := ph.VotingProcess.Get(opts, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %s", err)
	}
	cancelProcessTxArgs := new(models.CancelProcessTx)
	cancelProcessTxArgs.ProcessId = pid[:]
	cancelProcessTxArgs.Txtype = models.TxType_CANCEL_PROCESS
	return cancelProcessTxArgs, nil
}

func (ph *ProcessHandle) ProcessIndex(ctx context.Context, pid [32]byte) (*big.Int, error) {
	opts := &ethbind.CallOpts{Context: ctx}
	return ph.VotingProcess.GetProcessIndex(opts, pid)
}

func (ph *ProcessHandle) Oracles(ctx context.Context) ([]string, error) {
	opts := &ethbind.CallOpts{Context: ctx}
	oracles, err := ph.VotingProcess.GetOracles(opts)
	if err != nil {
		return []string{}, err
	}
	oraclesStr := []string{}
	for _, oracle := range oracles {
		oraclesStr = append(oraclesStr, oracle.Hex())
	}
	return oraclesStr, nil
}

func (ph *ProcessHandle) Validators(ctx context.Context) ([]string, error) {
	opts := &ethbind.CallOpts{Context: ctx}
	return ph.VotingProcess.GetValidators(opts)
}

func (ph *ProcessHandle) Genesis(ctx context.Context) (string, error) {
	opts := &ethbind.CallOpts{Context: ctx}
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
	tctx, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: tctx}
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
	var client *ethclient.Client
	for i := 0; i < types.EthereumDialMaxRetry; i++ {
		client, err = ethclient.Dial(ethEndpoint)
		if err != nil || client == nil {
			log.Warnf("cannot create a client connection: (%s), trying again (%d of %d)", err, i+1, types.EthereumDialMaxRetry)
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
	if err != nil || client == nil {
		log.Fatalf("cannot create a client connection: (%s), tried %d times.", err, types.EthereumDialMaxRetry)
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
	var client *ethclient.Client
	for i := 0; i < types.EthereumDialMaxRetry; i++ {
		client, err = ethclient.Dial(ethEndpoint)
		if err != nil || client == nil {
			log.Warnf("cannot create a client connection: (%s), trying again (%d of %d)", err, i+1, types.EthereumDialMaxRetry)
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}
	if err != nil || client == nil {
		log.Fatalf("cannot create a client connection: (%s), tried %d times.", err, types.EthereumDialMaxRetry)
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
	tctx, cancel := context.WithTimeout(ctx, types.EthereumWriteTimeout)
	defer cancel()
	opts := &ethbind.CallOpts{Context: tctx}
	metaURL, err := ensCallerHandler.Resolver.Text(opts, eIDBytes32, types.EntityMetaKey)
	if err != nil {
		return "", err
	}
	return metaURL, nil
}
