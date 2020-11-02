package testcommon

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/protobuf/proto"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/types"
	models "gitlab.com/vocdoni/go-dvote/types/proto"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

var (
	SignerPrivKey = "e0aa6db5a833531da4d259fb5df210bae481b276dc4c2ab6ab9771569375aed5"

	OracleListHardcoded = []common.Address{
		common.HexToAddress("0fA7A3FdB5C7C611646a535BDDe669Db64DC03d2"),
		common.HexToAddress("00192Fb10dF37c9FB26829eb2CC623cd1BF599E8"),
		common.HexToAddress("237B54D0163Aa131254fA260Fc12DB0E6DC76FC7"),
		common.HexToAddress("F904848ea36c46817096E94f932A9901E377C8a5"),
		common.HexToAddress("06d0d2c41f4560f8ffea1285f44ce0ffa2e19ef0"),
	}

	// VoteHardcoded needs to be a constructor, since multiple tests will
	// modify its value. We need a different pointer for each test.
	VoteHardcoded = func() *types.Vote {
		vp, _ := base64.StdEncoding.DecodeString("eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19")
		return &types.Vote{
			ProcessID:   util.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
			Nullifier:   util.Hex2byte(nil, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), // nullifier and nonce are the same here
			VotePackage: vp,
		}
	}

	ProcessHardcoded = &types.Process{
		EntityID:       util.Hex2byte(nil, "180dd5765d9f7ecef810b565a2e5bd14a3ccd536c442b3de74867df552855e85"),
		MkRoot:         "0a975f5cf517899e6116000fd366dc0feb34a2ea1b64e9b213278442dd9852fe",
		NumberOfBlocks: 1000,
		Type:           types.PetitionSign,
	}

	// privKey e0aa6db5a833531da4d259fb5df210bae481b276dc4c2ab6ab9771569375aed5 for address 06d0d2c41f4560f8ffea1285f44ce0ffa2e19ef0
	HardcodedNewProcessTx = &models.NewProcessTx{
		Txtype:         models.TxType_NEWPROCESS,
		EntityId:       util.Hex2byte(nil, "180dd5765d9f7ecef810b565a2e5bd14a3ccd536c442b3de74867df552855e85"),
		MkRoot:         util.Hex2byte(nil, "0a975f5cf517899e6116000fd366dc0feb34a2ea1b64e9b213278442dd9852fe"),
		ProcessId:      util.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		ProcessType:    types.PetitionSign,
		NumberOfBlocks: 1000,
	}

	HardcodedCancelProcessTx = &models.CancelProcessTx{
		Txtype:    models.TxType_CANCELPROCESS,
		ProcessId: util.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
	}

	HardcodedNewVoteTx = &models.VoteEnvelope{
		Nonce:     "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0",
		Nullifier: util.Hex2byte(nil, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"),
		ProcessId: util.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		Proof: &models.Proof{Proof: &models.Proof_Graviton{Graviton: &models.ProofGraviton{
			Siblings: util.Hex2byte(nil, "00030000000000000000000000000000000000000000000000000000000000070ab34471caaefc9bb249cb178335f367988c159f3907530ef7daa1e1bf0c9c7a218f981be7c0c46ffa345d291abb36a17c22722814fb0110240b8640fd1484a6268dc2f0fc2152bf83c06566fbf155f38b8293033d4779a63bba6c7157fd10c8"),
		}}},
		VotePackage: util.B642byte(nil, "eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19"),
	}

	HardcodedAdminTxAddOracle = &models.AdminTx{
		Txtype:  models.TxType_ADDORACLE,
		Address: util.Hex2byte(nil, "39106af1fF18bD60a38a296fd81B1f28f315852B"), // oracle address or pubkey validator
		Nonce:   "0x1",
	}

	HardcodedAdminTxRemoveOracle = &models.AdminTx{
		Txtype:  models.TxType_REMOVEORACLE,
		Address: util.Hex2byte(nil, "00192Fb10dF37c9FB26829eb2CC623cd1BF599E8"),
		Nonce:   "0x1",
	}
	power                        = uint64(10)
	HardcodedAdminTxAddValidator = &models.AdminTx{
		Txtype:  models.TxType_ADDVALIDATOR,
		Address: util.Hex2byte(nil, "5DC922017285EC24415F3E7ECD045665EADA8B5A"),
		Nonce:   "0x1",
		Power:   &power,
	}

	HardcodedAdminTxRemoveValidator = &models.AdminTx{
		Txtype:  models.TxType_REMOVEVALIDATOR,
		Address: util.Hex2byte(nil, "5DC922017285EC24415F3E7ECD045665EADA8B5A"),
		Nonce:   "0x1",
	}
)

func SignAndPrepareTx(vtx *models.Tx) error {
	// Signature
	signer := ethereum.SignKeys{}
	signer.AddHexKey(SignerPrivKey)
	var err error
	txb := []byte{}
	switch vtx.Tx.(type) {
	case *models.Tx_Vote:
		tx := vtx.GetVote()
		txb, err = proto.Marshal(tx)
	case *models.Tx_Admin:
		tx := vtx.GetAdmin()
		txb, err = proto.Marshal(tx)
	case *models.Tx_NewProcess:
		tx := vtx.GetNewProcess()
		txb, err = proto.Marshal(tx)
	case *models.Tx_CancelProcess:
		tx := vtx.GetCancelProcess()
		txb, err = proto.Marshal(tx)
	default:
		err = fmt.Errorf("transaction type unknown")
	}
	if err != nil {
		return err
	}
	hexsign, err := signer.Sign(txb)
	if err != nil {
		return err
	}
	vtx.Signature, err = hex.DecodeString(hexsign)
	return err
}

func TempDir(tb testing.TB, name string) string {
	tb.Helper()
	dir, err := ioutil.TempDir("", name)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func NewVochainStateWithOracles(tb testing.TB) *vochain.State {
	c := amino.NewCodec()
	vochain.RegisterAmino(c)
	s, err := vochain.NewState(TempDir(tb, "vochain-db"), c)
	if err != nil {
		tb.Fatal(err)
	}
	for _, o := range OracleListHardcoded {
		if err := s.AddOracle(o); err != nil {
			tb.Fatal(err)
		}
	}
	return s
}

func NewVochainStateWithValidators(tb testing.TB) *vochain.State {
	c := amino.NewCodec()
	vochain.RegisterAmino(c)
	s, err := vochain.NewState(TempDir(tb, "vochain-db"), c)
	if err != nil {
		tb.Fatal(err)
	}
	vals := make([]privval.FilePV, 2)
	rint := rand.Int()
	vals[0] = *privval.GenFilePV("/tmp/"+strconv.Itoa(rint), "/tmp/"+strconv.Itoa(rint))
	rint = rand.Int()
	vals[1] = *privval.GenFilePV("/tmp/"+strconv.Itoa(rint), "/tmp/"+strconv.Itoa(rint))
	validatorsBytes, err := c.MarshalJSON(vals)
	if err != nil {
		tb.Fatal(err)
	}
	if err = s.Store.Tree(vochain.AppTree).Add([]byte("validator"), validatorsBytes); err != nil {
		tb.Fatal(err)
	}
	oraclesBytes, err := s.Codec.MarshalBinaryBare(OracleListHardcoded)
	if err != nil {
		tb.Fatal(err)
	}
	if err = s.Store.Tree(vochain.AppTree).Add([]byte("oracle"), oraclesBytes); err != nil {
		tb.Fatal(err)
	}
	return s
}

func NewVochainStateWithProcess(tb testing.TB) *vochain.State {
	c := amino.NewCodec()
	vochain.RegisterAmino(c)
	s, err := vochain.NewState(TempDir(tb, "vochain-db"), c)
	if err != nil {
		tb.Fatal(err)
	}
	// add process
	processBytes, err := s.Codec.MarshalBinaryBare(ProcessHardcoded)
	if err != nil {
		tb.Fatal(err)
	}
	if err = s.Store.Tree(vochain.ProcessTree).Add(util.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), processBytes); err != nil {
		tb.Fatal(err)
	}
	return s
}

func NewMockVochainNode(tb testing.TB, d *DvoteAPIServer) *vochain.BaseApplication {
	cdc := amino.NewCodec()
	vochain.RegisterAmino(cdc)
	// start vochain node
	// create config
	d.VochainCfg = new(config.VochainCfg)
	d.VochainCfg.DataDir = TempDir(tb, "vochain-data")

	// create genesis file
	tmConsensusParams := tmtypes.DefaultConsensusParams()
	consensusParams := &types.ConsensusParams{
		Block:     types.BlockParams(tmConsensusParams.Block),
		Evidence:  types.EvidenceParams(tmConsensusParams.Evidence),
		Validator: types.ValidatorParams(tmConsensusParams.Validator),
	}

	// TO-DO instead of creating a pv file, just create a random 64 bytes key and use it for the genesis file
	validator := privval.GenFilePV(d.VochainCfg.DataDir+"/config/priv_validator_key.json", d.VochainCfg.DataDir+"/data/priv_validator_state.json")
	oracles := []string{d.Signer.AddressString()}
	genBytes, err := vochain.NewGenesis(d.VochainCfg, strconv.Itoa(rand.Int()), consensusParams, []privval.FilePV{*validator}, oracles)
	if err != nil {
		tb.Fatal(err)
	}
	// creating node
	d.VochainCfg.LogLevel = "error"
	d.VochainCfg.P2PListen = "0.0.0.0:26656"
	d.VochainCfg.PublicAddr = "0.0.0.0:26656"
	d.VochainCfg.RPCListen = "0.0.0.0:26657"
	// run node
	d.VochainCfg.MinerKey = fmt.Sprintf("%x", validator.Key.PrivKey)
	vnode := vochain.NewVochain(d.VochainCfg, genBytes)
	tb.Cleanup(func() {
		if err := vnode.Node.Stop(); err != nil {
			tb.Error(err)
		}
		vnode.Node.Wait()
	})
	return vnode
}

func NewMockScrutinizer(tb testing.TB, d *DvoteAPIServer, vnode *vochain.BaseApplication) *scrutinizer.Scrutinizer {
	tb.Log("starting vochain scrutinizer")
	d.ScrutinizerDir = TempDir(tb, "scrutinizer")
	sc, err := scrutinizer.NewScrutinizer(d.ScrutinizerDir, vnode.State)
	if err != nil {
		tb.Fatal(err)
	}
	return sc
}

// CreateEthRandomKeysBatch creates a set of eth random signing keys
func CreateEthRandomKeysBatch(tb testing.TB, n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = ethereum.NewSignKeys()
		if err := s[i].Generate(); err != nil {
			tb.Fatal(err)
		}
	}
	return s
}
