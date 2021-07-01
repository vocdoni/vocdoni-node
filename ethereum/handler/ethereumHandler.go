package ethereumhandler

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	ethbind "github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/vocdoni/storage-proofs-eth-go/token"
	"golang.org/x/crypto/sha3"
	"golang.org/x/net/idna"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/ethereum/contracts"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
)

// The following methods and structures represent an exportable abstraction
// over raw contract bindings.
// Use these methods, rather than those present in the contracts folder

// EthereumHandler wraps the vocdoni smart contracts
type EthereumHandler struct {
	VotingProcess     *contracts.Processes
	Namespace         *contracts.Namespaces
	TokenStorageProof *contracts.TokenStorageProof
	Genesis           *contracts.Genesis
	Results           *contracts.Results
	EntityResolver    *contracts.EntityResolver
	ENSPublicRegistry *contracts.EnsRegistryWithFallback
	ENSPublicResolver *contracts.EntityResolver
	EthereumClient    *ethclient.Client
	EthereumRPC       *ethrpc.Client
	Endpoints         []string
	SrcNetworkId      models.SourceNetworkId
}

// Genesis wraps the info retrieved for a call to genesis.Get(chainId)
type Genesis struct {
	Genesis    string
	Validators [][]byte
	Oracles    []common.Address
}

type EthSyncInfo struct {
	Height    uint64
	MaxHeight uint64
	Synced    bool
	Peers     int
	Mode      string
}

// NewEthereumHandler initializes contracts creating a transactor using the ethereum client
func NewEthereumHandler(contracts map[string]*EthereumContract, srcNetworkId models.SourceNetworkId,
	dialEndpoints []string) (*EthereumHandler, error) {
	eh := &EthereumHandler{
		SrcNetworkId: srcNetworkId,
		Endpoints:    dialEndpoints,
	}
	if err := eh.Connect(dialEndpoints); err != nil {
		return nil, err
	}
	eh.WaitSync()
	log.Infof("Using ENS Registry at address: %s", contracts[ContractNameENSregistry].Address.Hex())
	ctx, cancel := context.WithTimeout(context.Background(), types.EthereumReadTimeout)
	defer cancel()
	for name, contract := range contracts {
		if err := contract.InitContract(ctx, name, contracts[ContractNameENSregistry].Address, eh.EthereumClient); err != nil {
			return eh, fmt.Errorf("cannot initialize contracts: %w", err)
		}
		if err := eh.SetContractInstance(contract); err != nil {
			log.Errorf("cannot set contract instance: %s", err)
		}
	}
	return eh, nil
}

// Connect creates a new connection to the Ethereum client
func (eh *EthereumHandler) Connect(dialEndpoints []string) error {
	var err error
	for _, endpoint := range dialEndpoints {
		maxtries := 5
		for {
			if maxtries == 0 {
				log.Warnf("could not connect to %s endpoint, trying the next one", endpoint)
				break
			}
			if eh.EthereumRPC, err = ethrpc.Dial(endpoint); err != nil || eh.EthereumRPC == nil {
				log.Warnf("cannot create an ethereum rpc connection with %s: (%v), trying again", endpoint, err)
				time.Sleep(time.Second * 3)
				maxtries--
				continue
			}
			// if RPC connection established, create an ethereum client using the RPC client
			// TODO: @jordipainan this is racy
			eh.EthereumClient = ethclient.NewClient(eh.EthereumRPC)
			log.Infof("connected to %s web3 client", endpoint)
			return nil
		}
	}
	return fmt.Errorf("could not connect to any web3 endpoint")
}

func (eh *EthereumHandler) WaitSync() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if info, err := eh.SyncInfo(ctx); err == nil &&
			info.Synced && info.Peers >= 1 && info.Height > 0 {
			log.Infof("ethereum blockchain synchronized (%+v)", *info)
			cancel()
			break
		}
		if err := eh.Connect(eh.Endpoints); err != nil {
			log.Fatalf("cannot connect to any web3 endpoint: %v", err)
		}
		cancel()
		time.Sleep(time.Second * 5)
	}
}

// SyncInfo returns the height and syncing Ethereum blockchain information
func (eh *EthereumHandler) SyncInfo(ctx context.Context) (*EthSyncInfo, error) {
	info := new(EthSyncInfo)
	info.Mode = "web3"
	info.Synced = false
	info.Peers = 1 // force peers=1 if using external web3
	sp, err := eh.EthereumClient.SyncProgress(ctx)
	if err != nil {
		return nil, err
	}
	if sp != nil {
		info.MaxHeight = sp.HighestBlock
		info.Height = sp.CurrentBlock
	} else {
		header, err := eh.EthereumClient.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, err
		}
		info.Height = uint64(header.Number.Int64())
		info.MaxHeight = info.Height
		info.Synced = info.Height > 0
	}
	return info, nil
}

// PrintInfo prints every N seconds some ethereum information (sync and height). It's blocking!
func (eh *EthereumHandler) PrintInfo(ctx context.Context, seconds time.Duration) {
	var info *EthSyncInfo
	var lastHeight uint64
	var err error
	var syncingInfo string
	for {
		if ctx.Err() != nil {
			return
		}
		time.Sleep(seconds)
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		info, err = eh.SyncInfo(tctx)
		cancel()
		if err != nil {
			log.Warnf("error getting ethereum info: %s", err)
			continue
		}
		if !info.Synced {
			syncingInfo = fmt.Sprintf("syncSpeed:%d b/s", (info.Height-lastHeight)/uint64(seconds.Seconds()))
		} else {
			syncingInfo = ""
		}
		log.Infof("[ethereum info] synced:%t height:%d/%d mode:%s src:%s %s",
			info.Synced, info.Height, info.MaxHeight, info.Mode, eh.SrcNetworkId, syncingInfo)
		lastHeight = info.Height
	}
}

// SetContractInstance creates the given contract transactor and returns
// an object ready to interact with a web3 fashion
func (eh *EthereumHandler) SetContractInstance(ec *EthereumContract) error {
	var err error
	switch {
	case strings.HasPrefix(ec.Domain, ContractNameProcesses):
		if eh.VotingProcess, err = contracts.NewProcesses(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing processes contract transactor: %w", err)
		}
	case strings.HasPrefix(ec.Domain, ContractNameNamespaces):
		if eh.Namespace, err = contracts.NewNamespaces(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing namespace contract transactor: %w", err)
		}
	case strings.HasPrefix(ec.Domain, ContractNameTokenStorageProof):
		if eh.TokenStorageProof, err = contracts.NewTokenStorageProof(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing token storage proof contract transactor: %w", err)
		}
	case strings.HasPrefix(ec.Domain, ContractNameGenesis):
		if eh.Genesis, err = contracts.NewGenesis(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing genesis contract transactor: %w", err)
		}
	case strings.HasPrefix(ec.Domain, ContractNameResults):
		if eh.Results, err = contracts.NewResults(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing results contract transactor: %w", err)
		}
	case strings.HasPrefix(ec.Domain, ContractNameEntities):
		if eh.EntityResolver, err = contracts.NewEntityResolver(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing results contract transactor: %w", err)
		}
	case strings.HasPrefix(ec.Domain, ContractNameENSresolver):
		if eh.ENSPublicResolver, err = contracts.NewEntityResolver(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing results contract transactor: %w", err)
		}
	case strings.HasPrefix(ec.Domain, ContractNameENSregistry):
		if eh.ENSPublicRegistry, err = contracts.NewEnsRegistryWithFallback(ec.Address, eh.EthereumClient); err != nil {
			return fmt.Errorf("error constructing results contract transactor: %w", err)
		}
	}
	return nil
}

// PROCESSES WRAPPER

// NewProcessTxArgs gets the info of a created process on the processes contract and creates a NewProcessTx instance
func (eh *EthereumHandler) NewProcessTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte, namespace uint32) (*models.NewProcessTx, error) {
	// TODO: @jordipainan What to do with namespace?
	// get process info from the processes contract
	processMeta, err := eh.VotingProcess.Get(&ethbind.CallOpts{Context: ctx}, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %w", err)
	}
	processData := new(models.Process)

	// check status ready or paused
	status := models.ProcessStatus(processMeta.Status + 1) // +1 required to match with solidity enum
	if status != models.ProcessStatus_READY && status != models.ProcessStatus_PAUSED {
		return nil, fmt.Errorf("invalid process status on process creation: %d", status)
	}
	processData.Status = status
	processData.ProcessId = pid[:]

	// entity id
	// for evm censuses the entity id is the snapshoted contract address
	if processData.EntityId, err = hex.DecodeString(util.TrimHex(processMeta.EntityAddressOwner[0].String())); err != nil {
		return nil, fmt.Errorf("error decoding entity address: %w", err)
	}

	// process metadata
	processData.Metadata = &processMeta.MetadataCensusRootCensusUri[0]

	// census root
	processData.CensusRoot, err = hex.DecodeString(util.TrimHex(processMeta.MetadataCensusRootCensusUri[1]))
	if err != nil {
		return nil, fmt.Errorf("cannot decode census root: %w", err)
	}

	// census origin
	censusOrigin := models.CensusOrigin(processMeta.ModeEnvelopeTypeCensusOrigin[2])
	if _, ok := vochain.CensusOrigins[censusOrigin]; !ok {
		return nil, fmt.Errorf("census origin: %d not supported", censusOrigin)
	}
	processData.CensusOrigin = censusOrigin

	// census URI
	if vochain.CensusOrigins[censusOrigin].NeedsURI && len(processMeta.MetadataCensusRootCensusUri[2]) == 0 {
		return nil, fmt.Errorf("census %s needs URI, none has been provided", vochain.CensusOrigins[censusOrigin].Name)
	}
	processData.CensusURI = &processMeta.MetadataCensusRootCensusUri[2]

	// start and end blocks
	processData.StartBlock = processMeta.StartBlockBlockCount[0]
	if processMeta.StartBlockBlockCount[1] < types.ProcessesContractMinBlockCount {
		return nil, fmt.Errorf("block count is too low")
	}
	processData.BlockCount = processMeta.StartBlockBlockCount[1]

	// process mode
	if processData.Mode, err = extractProcessMode(processMeta.ModeEnvelopeTypeCensusOrigin[0]); err != nil {
		return nil, fmt.Errorf("cannot extract process mode: %w", err)
	}

	// envelope type
	if processData.EnvelopeType, err = extractEnvelopeType(processMeta.ModeEnvelopeTypeCensusOrigin[1]); err != nil {
		return nil, fmt.Errorf("cannot extract envelope type: %w", err)
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
		MaxTotalCost: uint32(processMeta.MaxTotalCostCostExponent[0]),
		// cost exponent
		CostExponent: uint32(processMeta.MaxTotalCostCostExponent[1]),
	}

	// namespace and sourceNetworkId
	processData.Namespace, err = eh.VotingProcess.NamespaceId(&ethbind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %w", err)
	}
	processData.SourceNetworkId = eh.SrcNetworkId

	// if EVM census, check census root provided and get index slot from the token storage proof contract
	if vochain.CensusOrigins[censusOrigin].NeedsIndexSlot {
		// check valid storage root provided
		// the holder address is chosen randomly
		randSigner := ethereum.NewSignKeys()
		if err := randSigner.Generate(); err != nil {
			return nil, fmt.Errorf("cannot check storage root, cannot generate random Ethereum address: %w", err)
		}
		fetchedRoot, err := eh.getStorageRoot(ctx, randSigner.Address(), processMeta.EntityAddressOwner[0], processMeta.SourceBlockHeight)
		if err != nil {
			return nil, fmt.Errorf("cannot check EVM storage root: %w", err)
		}
		if bytes.Equal(fetchedRoot.Bytes(), common.Hash{}.Bytes()) {
			return nil, fmt.Errorf("invalid storage root obtained from Ethereum: %x", fetchedRoot)
		}
		if !bytes.Equal(fetchedRoot.Bytes(), processData.CensusRoot) {
			return nil, fmt.Errorf("invalid storage root: root fetched must be the same as root provided. "+
				"Got: %x expected: %x", fetchedRoot, processData.CensusRoot)
		}
		// get index slot from the token storage proof contract
		islot, err := eh.GetTokenBalanceMappingPosition(ctx, processMeta.EntityAddressOwner[0])
		if err != nil {
			return nil, fmt.Errorf("cannot get balance mapping position from the contract: %w", err)
		}
		iSlot32 := uint32(islot.Uint64())
		processData.EthIndexSlot = &iSlot32
		// decode owner
		if processData.Owner, err = hex.DecodeString(util.TrimHex(processMeta.EntityAddressOwner[1].String())); err != nil {
			return nil, fmt.Errorf("error decoding owner address: %w", err)
		}
		// save sourceBlockHeight
		sourceBlockHeight64 := uint64(processMeta.SourceBlockHeight.Uint64())
		if sourceBlockHeight64 == 0 {
			return nil, fmt.Errorf("source block height must be > 0: %w", err)
		}
		processData.SourceBlockHeight = &sourceBlockHeight64
	}

	// set tx type (outer type used by the vochain tx type)
	processTxArgs := new(models.NewProcessTx)
	processTxArgs.Txtype = models.TxType_NEW_PROCESS
	processTxArgs.Process = processData
	return processTxArgs, nil
}

func (eh *EthereumHandler) getStorageRoot(ctx context.Context, holder common.Address, contractAddr common.Address, blockNum *big.Int) (hash common.Hash, err error) {
	// create token storage proof artifact
	ts := token.ERC20Token{
		RPCcli: eh.EthereumRPC,
		Ethcli: eh.EthereumClient,
	}
	ts.Init(ctx, "", contractAddr.String())
	// get block
	blk, err := ts.GetBlock(ctx, blockNum)
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot get block: %w", err)
	}
	// get proof
	log.Debugf("get EVM storage root for address %s and block %d", holder.String(), blk.NumberU64())
	sproof, err := ts.GetProofWithIndexSlot(ctx, holder, blk, 1)
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot get storage root: %w", err)
	}
	// return the storage root hash
	return sproof.StorageHash, nil
}

func extractEnvelopeType(envelopeType uint8) (*models.EnvelopeType, error) {
	if envelopeType > types.ProcessesContractMaxEnvelopeType {
		return nil, fmt.Errorf("invalid envelope type: (%d)", envelopeType)
	}
	return &models.EnvelopeType{
		Serial:         envelopeType&byte(0b00000001) > 0,
		Anonymous:      envelopeType&byte(0b00000010) > 0,
		EncryptedVotes: envelopeType&byte(0b00000100) > 0,
		UniqueValues:   envelopeType&byte(0b00001000) > 0,
		CostFromWeight: envelopeType&byte(0b00010000) > 0,
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

// SetStatusTxArgs returns a SetProcessTx instance with the processStatus set
func (eh *EthereumHandler) SetStatusTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte, namespace uint32, status uint8) (*models.SetProcessTx, error) {
	status++ // +1 for matching with ethevent uint8
	processData, err := eh.VotingProcess.Get(&ethbind.CallOpts{Context: ctx}, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %w", err)
	}
	if processData.Status == status {
		return nil, fmt.Errorf("status should be changed: %w", err)
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

// SetCensusTxArgs returns a SetProcess tx instance with census censusRoot and census censusURI set
func (eh *EthereumHandler) SetCensusTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte, namespace uint32) (*models.SetProcessTx, error) {
	processData, err := eh.VotingProcess.Get(&ethbind.CallOpts{Context: ctx}, pid)
	if err != nil {
		return nil, fmt.Errorf("error fetching process from Ethereum: %w", err)
	}
	// create setProcessTx
	setprocessTxArgs := new(models.SetProcessTx)
	// process id
	setprocessTxArgs.ProcessId = pid[:]
	// process censusRoot
	if setprocessTxArgs.CensusRoot, err = hex.DecodeString(util.TrimHex(processData.MetadataCensusRootCensusUri[1])); err != nil {
		return nil, fmt.Errorf("invalid census root: %w", err)
	}

	censusOrigin := models.CensusOrigin(processData.ModeEnvelopeTypeCensusOrigin[2])
	if !vochain.CensusOrigins[censusOrigin].AllowCensusUpdate {
		return nil, fmt.Errorf("cannot update census, invalid census origin")
	}
	if vochain.CensusOrigins[censusOrigin].NeedsURI && len(processData.MetadataCensusRootCensusUri[2]) == 0 {
		return nil, fmt.Errorf("census %s needs URI but an empty census URI has been provided",
			censusOrigin.String())
	}
	setprocessTxArgs.CensusURI = &processData.MetadataCensusRootCensusUri[2]

	// TODO: @jordipainan namespace not used
	setprocessTxArgs.Txtype = models.TxType_SET_PROCESS_CENSUS

	return setprocessTxArgs, nil
}

// IncrementQuestionIndexTxArgs
func (eh *EthereumHandler) IncrementQuestionIndexTxArgs(ctx context.Context, pid [types.ProcessIDsize]byte) (*models.SetProcessTx, error) {
	return nil, fmt.Errorf("not implemented")
}

// EntityProcessCount returns the entity process count given an entity address
func (eh *EthereumHandler) EntityProcessCount(ctx context.Context, eid common.Address) (entityProcessCount *big.Int, err error) {
	if entityProcessCount, err = eh.VotingProcess.GetEntityProcessCount(&ethbind.CallOpts{Context: ctx}, eid); err != nil {
		err = fmt.Errorf("cannot get entity process count: %w", err)
	}
	return
}

// EntityNextProcessID returns the next process id of a given entity address
func (eh *EthereumHandler) EntityNextProcessID(ctx context.Context, eid common.Address) (entityNextProcessID [32]byte, err error) {
	if entityNextProcessID, err = eh.VotingProcess.GetNextProcessId(&ethbind.CallOpts{Context: ctx}, eid); err != nil {
		err = fmt.Errorf("cannot get entity's next process id: %w", err)
	}
	return
}

// ProcessParamsSignature returns the signature of the process parameters
func (eh *EthereumHandler) ProcessParamsSignature(ctx context.Context, pid [types.ProcessIDsize]byte) (processParamsSignature [types.ProcessesParamsSignatureSize]byte, err error) {
	if processParamsSignature, err = eh.VotingProcess.GetParamsSignature(&ethbind.CallOpts{Context: ctx}, pid); err != nil {
		err = fmt.Errorf("cannot get process params signature: %w", err)
	}
	return
}

// ProcessCreationInstance returns the address of the processes contract instance where the process was created
func (eh *EthereumHandler) ProcessCreationInstance(ctx context.Context, pid [types.ProcessIDsize]byte) (processCreationInstance common.Address, err error) {
	if processCreationInstance, err = eh.VotingProcess.GetCreationInstance(&ethbind.CallOpts{Context: ctx}, pid); err != nil {
		err = fmt.Errorf("cannot get process creation instance: %w", err)
	}
	return
}

// NAMESPACE WRAPPER

// TOKEN STORAGE PROOF WRAPPER

// IsTokenRegistered returns true if a token represented by the given address is registered on the token storage proof contract
func (eh *EthereumHandler) IsTokenRegistered(ctx context.Context, address common.Address) (bool, error) {
	return eh.TokenStorageProof.IsRegistered(&ethbind.CallOpts{Context: ctx}, address)
}

// GetTokenBalanceMappingPosition returns the balance mapping position given a token address
func (eh *EthereumHandler) GetTokenBalanceMappingPosition(ctx context.Context, address common.Address) (*big.Int, error) {
	tokenInfo, err := eh.TokenStorageProof.Tokens(&ethbind.CallOpts{Context: ctx}, address)
	if err != nil {
		return nil, err
	}
	return tokenInfo.BalanceMappingPosition, nil
}

// GENESIS WRAPPER

// AddOracleTxArgs returns an Admin tx instance with the oracle address to add
func (eh *EthereumHandler) AddOracleTxArgs(ctx context.Context, oracleAddress common.Address, chainId uint32) (tx *models.AdminTx, err error) {
	genesis, err := eh.Genesis.Get(&ethbind.CallOpts{Context: ctx}, chainId)
	if err != nil {
		return nil, fmt.Errorf("cannot get genesis %d: %w", chainId, err)
	}
	var found bool
	for _, oracle := range genesis.Oracles {
		if oracle == oracleAddress {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("cannot fetch added oracle from ethereum")
	}
	addOracleTxArgs := &models.AdminTx{
		Address: oracleAddress.Bytes(),
		Txtype:  models.TxType_ADD_ORACLE,
	}
	return addOracleTxArgs, nil
}

// RemoveOracleTxArgs returns an Admin tx instance with the oracle address to remove
func (eh *EthereumHandler) RemoveOracleTxArgs(ctx context.Context, oracleAddress common.Address, chainId uint32) (tx *models.AdminTx, err error) {
	genesis, err := eh.Genesis.Get(&ethbind.CallOpts{Context: ctx}, chainId)
	if err != nil {
		return nil, fmt.Errorf("cannot get genesis %d: %w", chainId, err)
	}
	for _, oracle := range genesis.Oracles {
		if oracle == oracleAddress {
			return nil, fmt.Errorf("cannot remove oracle: the oracle should not be on ethereum")
		}
	}
	removeOracleTxArgs := &models.AdminTx{
		Address: oracleAddress.Bytes(),
		Txtype:  models.TxType_REMOVE_ORACLE,
	}
	return removeOracleTxArgs, nil
}

// RESULTS WRAPPER

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
func ENSAddress(ctx context.Context, publicRegistryAddr, domain string, web3Client *ethclient.Client) (string, error) {
	// normalize voting process domain name
	nh, err := NameHash(domain)
	if err != nil {
		return "", fmt.Errorf("cannot get ENS address of the given domain: %w", err)
	}
	ensCallerHandler := &ENSCallerHandler{
		PublicRegistryAddr: publicRegistryAddr,
		EthereumClient:     web3Client,
	}
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
func EnsResolve(ctx context.Context, ensRegistryAddr, ethDomain string, web3Client *ethclient.Client) (contractAddr string, err error) {
	for i := 0; i < maxRetries; i++ {
		contractAddr, err = ENSAddress(ctx, ensRegistryAddr, ethDomain, web3Client)
		if err != nil {
			if strings.Contains(err.Error(), "no suitable peers available") {
				time.Sleep(time.Second * 2)
				continue
			}
			err = fmt.Errorf("cannot get contract address: %w", err)
			return
		}
		break
	}
	return
}

// ResolveEntityMetadataURL returns the metadata URL given an entityID
func ResolveEntityMetadataURL(ctx context.Context, ensRegistryAddr, entityResolverDomain string, entityID, ethEndpoint string) (string, error) {
	// normalize entity resolver domain name
	nh, err := NameHash(entityResolverDomain)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	var client *ethclient.Client
	for i := 0; i < types.EthereumDialMaxRetry; i++ {
		client, err = ethclient.Dial(ethEndpoint)
		if err != nil || client == nil {
			log.Warnf("cannot create a client connection: %s, trying again... %d of %d",
				err, i+1, types.EthereumDialMaxRetry)
			time.Sleep(time.Second * 10)
			continue
		}
		break
	}
	if err != nil || client == nil {
		log.Errorf("cannot create a client connection: %s, tried %d times.",
			err, types.EthereumDialMaxRetry)
		return "", fmt.Errorf("cannot resolve entity metadata URL, cannot create a client connection: %w",
			err)
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
	// resolve entity resolver addr
	tctx, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	rAddr, err := ensCallerHandler.Resolver.Addr(&ethbind.CallOpts{Context: tctx}, nh)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	// assign entity resolver to ensCallerHandler resolver
	ensCallerHandler.ResolverAddr = rAddr.String()
	if err := ensCallerHandler.NewEntityResolverHandle(); err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	// get entity metadata url from resolver
	eIDBytes, err := hex.DecodeString(entityID)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	// keccak256 entity addr
	eIDBytes = ethereum.HashRaw(eIDBytes)
	var eIDBytes32 [32]byte
	copy(eIDBytes32[:], eIDBytes)
	tctx, cancel = context.WithTimeout(ctx, types.EthereumWriteTimeout)
	defer cancel()
	// get stored text
	metaURL, err := ensCallerHandler.Resolver.Text(&ethbind.CallOpts{Context: tctx}, eIDBytes32, types.EntityMetaKey)
	if err != nil {
		return "", fmt.Errorf("cannot resolve entity metadata URL: %w", err)
	}
	return metaURL, nil
}
