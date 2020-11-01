package testcommon

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"git.sr.ht/~sircmpwn/go-bare"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

var (
	SignerPrivKey = "e0aa6db5a833531da4d259fb5df210bae481b276dc4c2ab6ab9771569375aed5"

	OracleListHardcoded = []string{
		"0fA7A3FdB5C7C611646a535BDDe669Db64DC03d2",
		"00192Fb10dF37c9FB26829eb2CC623cd1BF599E8",
		"237B54D0163Aa131254fA260Fc12DB0E6DC76FC7",
		"F904848ea36c46817096E94f932A9901E377C8a5",
		"06d0d2c41f4560f8ffea1285f44ce0ffa2e19ef0",
	}

	// VoteHardcoded needs to be a constructor, since multiple tests will
	// modify its value. We need a different pointer for each test.
	VoteHardcoded = func() *types.Vote {
		return &types.Vote{
			ProcessID:   util.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
			Nullifier:   util.Hex2byte(nil, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), // nullifier and nonce are the same here
			VotePackage: "eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19",
		}
	}

	ProcessHardcoded = &types.Process{
		EntityID:             util.Hex2byte(nil, "180dd5765d9f7ecef810b565a2e5bd14a3ccd536c442b3de74867df552855e85"),
		MkRoot:               "0a975f5cf517899e6116000fd366dc0feb34a2ea1b64e9b213278442dd9852fe",
		NumberOfBlocks:       1000,
		EncryptionPublicKeys: OracleListHardcoded, // reusing oracle keys as encryption pub keys
		Type:                 types.PetitionSign,
	}

	// privKey e0aa6db5a833531da4d259fb5df210bae481b276dc4c2ab6ab9771569375aed5 for address 06d0d2c41f4560f8ffea1285f44ce0ffa2e19ef0
	HardcodedNewProcessTx = &types.NewProcessTx{
		EntityID:       "180dd5765d9f7ecef810b565a2e5bd14a3ccd536c442b3de74867df552855e85",
		MkRoot:         "0x0a975f5cf517899e6116000fd366dc0feb34a2ea1b64e9b213278442dd9852fe",
		NumberOfBlocks: 1000,
		ProcessID:      "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105",
		ProcessType:    types.PetitionSign,
		Type:           "newProcess",
	}

	HardcodedCancelProcessTx = &types.CancelProcessTx{
		ProcessID: "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105",
		Type:      "cancelProcess",
	}

	HardcodedNewVoteTx = &types.VoteTx{
		Nonce:       "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0",
		Nullifier:   "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0",
		ProcessID:   "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105",
		Proof:       "00030000000000000000000000000000000000000000000000000000000000070ab34471caaefc9bb249cb178335f367988c159f3907530ef7daa1e1bf0c9c7a218f981be7c0c46ffa345d291abb36a17c22722814fb0110240b8640fd1484a6268dc2f0fc2152bf83c06566fbf155f38b8293033d4779a63bba6c7157fd10c8",
		Type:        "vote",
		VotePackage: "eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19",
	}

	HardcodedAdminTxAddOracle = &types.AdminTx{
		Address: "39106af1fF18bD60a38a296fd81B1f28f315852B", // oracle address or pubkey validator
		Nonce:   "0x1",
		Type:    "addOracle",
	}

	HardcodedAdminTxRemoveOracle = &types.AdminTx{
		Address: "00192Fb10dF37c9FB26829eb2CC623cd1BF599E8",
		Nonce:   "0x1",
		Type:    "removeOracle",
	}

	HardcodedAdminTxAddValidator = &types.AdminTx{
		Address: "GyZfKNK3lT5AQXQ4pwrVdgG3rRisx9tS4bM9EIZ0zYY=",
		Nonce:   "0x1",
		Power:   10,
		Type:    "addValidator",
	}

	HardcodedAdminTxRemoveValidator = &types.AdminTx{
		Address: "5DC922017285EC24415F3E7ECD045665EADA8B5A",
		Nonce:   "0x1",
		Type:    "removeValidator",
	}
)

func EncodeTx(tx interface{}) ([]byte, error) {
	return bare.Marshal(tx)
}

func SignTx(tx interface{}) (string, error) {
	signer := ethereum.SignKeys{}
	signer.AddHexKey(SignerPrivKey)
	txb, err := bare.Marshal(tx)
	if err != nil {
		return "", err
	}
	return signer.Sign(txb)
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
	oraclesBytes, err := s.Codec.MarshalBinaryBare(OracleListHardcoded)
	if err != nil {
		tb.Fatal(err)
	}
	s.Store.Tree(vochain.AppTree).Add([]byte("oracle"), oraclesBytes)
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
