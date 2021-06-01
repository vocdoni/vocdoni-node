package apioracle

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/storage-proofs-eth-go/ethstorageproof"
	ethtoken "github.com/vocdoni/storage-proofs-eth-go/token"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	chain "go.vocdoni.io/dvote/ethereum"
	ethereumhandler "go.vocdoni.io/dvote/ethereum/handler"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/oracle"
	"go.vocdoni.io/dvote/router"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	ethQueryTimeOut      = 20 * time.Second
	aclPurgePeriod       = 10 * time.Minute
	aclTimeWindow        = 5 * 24 * time.Hour // 5 days
	maxProposalPerWindow = 3
)

var srcNetworkIds = map[string]models.SourceNetworkId{
	"default":   models.SourceNetworkId_UNKNOWN,
	"mainnet":   models.SourceNetworkId_ETH_MAINNET_SIGNALING,
	"homestead": models.SourceNetworkId_ETH_MAINNET_SIGNALING,
	"rinkeby":   models.SourceNetworkId_ETH_RINKEBY_SIGNALING,
}

type APIoracle struct {
	Namespace        uint32
	oracle           *oracle.Oracle
	router           *router.Router
	eh               *ethereumhandler.EthereumHandler
	chainNames       map[string]bool
	erc20proposalACL *proposalACL
}

func NewAPIoracle(o *oracle.Oracle, r *router.Router) (*APIoracle, error) {
	a := &APIoracle{router: r, oracle: o, chainNames: make(map[string]bool)}
	return a, nil
}

func (a *APIoracle) EnableERC20(chainName, web3Endpoint string) error {
	if chainName == "" || web3Endpoint == "" {
		return fmt.Errorf("no web3 endpoint or chain name provided")
	}
	specs, err := chain.SpecsFor(chainName)
	if err != nil {
		return err
	}
	a.erc20proposalACL = NewProposalACL(aclTimeWindow, maxProposalPerWindow)
	srcNetId, ok := srcNetworkIds[chainName]
	if !ok {
		srcNetId = srcNetworkIds["default"]
	}
	a.eh, err = ethereumhandler.NewEthereumHandler(specs.Contracts, srcNetId, web3Endpoint)
	if err != nil {
		return err
	}
	a.eh.WaitSync()
	for k, v := range srcNetworkIds {
		if v.String() == srcNetId.String() {
			a.chainNames[k] = true
		}
	}
	a.router.RegisterPublic("newERC20process", a.handleNewEthProcess)
	a.router.APIs = append(a.router.APIs, "oracle")
	return nil
}

func (a *APIoracle) handleNewEthProcess(req router.RouterRequest) {
	var response api.MetaResponse
	if req.NewProcess == nil {
		a.router.SendError(req, "newProcess is empty")
		return
	}
	if _, ok := a.chainNames[req.NewProcess.NetworkId]; !ok {
		a.router.SendError(req, fmt.Sprintf("provided chainId does not match ours (%s)",
			req.NewProcess.NetworkId))
		return
	}
	if req.NewProcess.EthIndexSlot == nil {
		a.router.SendError(req, "index slot not provided")
		return
	}
	if req.NewProcess.SourceHeight == nil {
		a.router.SendError(req, "no source height provided")
		return
	}

	pidseed := fmt.Sprintf("%d%d%x%x",
		a.Namespace,
		req.NewProcess.StartBlock,
		req.NewProcess.EntityID,
		util.RandomBytes(32),
	)

	p := &models.Process{
		EntityId:          req.NewProcess.EntityID,
		StartBlock:        req.NewProcess.StartBlock,
		BlockCount:        req.NewProcess.BlockCount,
		CensusRoot:        req.NewProcess.CensusRoot,
		EnvelopeType:      req.NewProcess.EnvelopeType,
		VoteOptions:       req.NewProcess.VoteOptions,
		EthIndexSlot:      req.NewProcess.EthIndexSlot,
		SourceBlockHeight: req.NewProcess.SourceHeight,
		Metadata:          &req.NewProcess.Metadata,
		ProcessId:         ethereum.HashRaw([]byte(pidseed)),
		Status:            models.ProcessStatus_READY,
		Namespace:         a.Namespace,
		CensusOrigin:      models.CensusOrigin_ERC20,
		Mode:              &models.ProcessMode{AutoStart: true},
		SourceNetworkId:   a.eh.SrcNetworkId,
		Owner:             req.GetAddress().Bytes(),
	}

	// Check the ACL
	if err := a.erc20proposalACL.add(p.Owner, p.EntityId); err != nil {
		a.router.SendError(req, err.Error())
		return
	}

	sproof, err := buildETHproof(req.EthProof)
	if err != nil {
		a.router.SendError(req, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), ethQueryTimeOut)
	defer cancel()
	index, err := a.getIndexSlot(ctx, p.EntityId, p.GetSourceBlockHeight(), p.CensusRoot)
	if err != nil {
		a.router.SendError(req, err.Error())
		return
	}
	log.Infof("fetched index slot %d for contract %x", index, p.EntityId)
	if index != p.GetEthIndexSlot() {
		a.router.SendError(req, "index slot does not match")
		return
	}

	slot, err := ethtoken.GetSlot(req.GetAddress().Hex(), int(index))
	if err != nil {
		a.router.SendError(req, fmt.Sprintf("cannot fetch slot: %s", err))
		return
	}

	log.Debugf("ERC20 index slot %d, storage slot %x", index, slot)

	valid, _, err := vochain.CheckProof(sproof,
		models.CensusOrigin_ERC20,
		p.CensusRoot,
		p.ProcessId,
		slot[:],
	)
	if err != nil {
		a.router.SendError(req, err.Error())
		return
	}
	if !valid {
		a.router.SendError(req, "proof is not valid")
		return
	}
	if err := a.oracle.NewProcess(p); err != nil {
		a.router.SendError(req, err.Error())
		return
	}

	response.ProcessID = p.ProcessId
	if err := req.Send(a.router.BuildReply(req, &response)); err != nil {
		log.Warn(err)
	}
}

func buildETHproof(proof *ethstorageproof.StorageResult) (*models.Proof, error) {
	if proof == nil {
		return nil, fmt.Errorf("storage proof is nil")
	}
	if len(proof.Proof) < 1 {
		return nil, fmt.Errorf("storage proof siblings missing")
	}
	if proof.Value == nil {
		return nil, fmt.Errorf("storage proof value missing")
	}
	siblings := [][]byte{}
	for _, sib := range proof.Proof {
		sibb, err := hex.DecodeString(util.TrimHex(sib))
		if err != nil {
			return nil, err
		}
		siblings = append(siblings, sibb)
	}
	key, err := hex.DecodeString(util.TrimHex(proof.Key))
	if err != nil {
		return nil, fmt.Errorf("cannot decode key: %w", err)
	}
	return &models.Proof{Payload: &models.Proof_EthereumStorage{
		EthereumStorage: &models.ProofEthereumStorage{
			Key:      key,
			Value:    proof.Value.ToInt().Bytes(),
			Siblings: siblings,
		},
	}}, nil
}

func (a *APIoracle) getIndexSlot(ctx context.Context, contractAddr []byte,
	evmBlockHeight uint64, rootHash []byte) (uint32, error) {
	// check valid storage root provided
	if len(contractAddr) != common.AddressLength {
		return 0, fmt.Errorf("contractAddress length is not correct")
	}
	addr := common.Address{}
	copy(addr[:], contractAddr[:])
	fetchedRoot, err := a.getStorageRoot(ctx, addr, evmBlockHeight)
	if err != nil {
		return 0, fmt.Errorf("cannot check EVM storage root: %w", err)
	}
	if bytes.Equal(fetchedRoot.Bytes(), common.Hash{}.Bytes()) {
		return 0, fmt.Errorf("invalid storage root obtained from Ethereum: %x", fetchedRoot)
	}
	if !bytes.Equal(fetchedRoot.Bytes(), rootHash) {
		return 0, fmt.Errorf("invalid storage root, got: %x expected: %x",
			fetchedRoot, rootHash)
	}
	// get index slot from the token storage proof contract
	islot, err := a.eh.GetTokenBalanceMappingPosition(ctx, addr)
	if err != nil {
		return 0, fmt.Errorf("cannot get balance mapping position from the contract: %w", err)
	}
	return uint32(islot.Uint64()), nil
}

// getStorageRoot returns the storage Root Hash of the Ethereum Patricia trie for
// a contract address and a block height
func (a *APIoracle) getStorageRoot(ctx context.Context, contractAddr common.Address,
	blockNum uint64) (hash common.Hash, err error) {
	// create token storage proof artifact
	ts := ethtoken.ERC20Token{
		RPCcli: a.eh.EthereumRPC,
		Ethcli: a.eh.EthereumClient,
	}
	if err := ts.Init(ctx, "", contractAddr.String()); err != nil {
		return common.Hash{}, err
	}

	// get block
	blk, err := ts.GetBlock(ctx, new(big.Int).SetUint64(blockNum))
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot get block: %w", err)
	}

	// get proof, we use a random token holder and a dummy index slot
	holder := ethereum.NewSignKeys()
	if err := holder.Generate(); err != nil {
		return common.Hash{},
			fmt.Errorf("cannot check storage root, cannot generate random Ethereum address: %w", err)
	}

	log.Debugf("get EVM storage root for address %s and block %d", holder.Address(), blockNum)
	sproof, err := ts.GetProofWithIndexSlot(ctx, holder.Address(), blk, 1)
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot get storage root: %w", err)
	}

	// return the storage root hash
	return sproof.StorageHash, nil
}
