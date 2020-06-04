package testcommon

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"

	amino "github.com/tendermint/go-amino"
	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/privval"
	voclient "github.com/tendermint/tendermint/rpc/client"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

var (
	OracleListHardcoded = []string{
		"0fA7A3FdB5C7C611646a535BDDe669Db64DC03d2",
		"00192Fb10dF37c9FB26829eb2CC623cd1BF599E8",
		"237B54D0163Aa131254fA260Fc12DB0E6DC76FC7",
		"F904848ea36c46817096E94f932A9901E377C8a5",
		"06d0d2c41f4560f8ffea1285f44ce0ffa2e19ef0",
	}

	ProcessHardcoded = &types.Process{
		EntityID:             "180dd5765d9f7ecef810b565a2e5bd14a3ccd536c442b3de74867df552855e85",
		MkRoot:               "0a975f5cf517899e6116000fd366dc0feb34a2ea1b64e9b213278442dd9852fe",
		NumberOfBlocks:       1000,
		EncryptionPublicKeys: OracleListHardcoded, // reusing oracle keys as encryption pub keys
		Type:                 types.PetitionSign,
	}

	VoteHardcoded = &types.Vote{
		ProcessID:   "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105",
		Proof:       "0x00030000000000000000000000000000000000000000000000000000000000070ab34471caaefc9bb249cb178335f367988c159f3907530ef7daa1e1bf0c9c7a218f981be7c0c46ffa345d291abb36a17c22722814fb0110240b8640fd1484a6268dc2f0fc2152bf83c06566fbf155f38b8293033d4779a63bba6c7157fd10c8",
		Nullifier:   "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0", // nullifier and nonce are the same here
		VotePackage: "eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19",
		Nonce:       "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0",
		Signature:   "8ee76647eb9a5639c776aff4e0452410edc50fe5b3d0a6d619383effc02daa4b2f00e74105d84eb016bf424a0e4bfcee1045db13b97ae2c4d484c8fdff541bce1b",
	}

	// privKey e0aa6db5a833531da4d259fb5df210bae481b276dc4c2ab6ab9771569375aed5 for address 06d0d2c41f4560f8ffea1285f44ce0ffa2e19ef0
	HardcodedNewProcessTx = &types.NewProcessTx{
		EntityID:       "180dd5765d9f7ecef810b565a2e5bd14a3ccd536c442b3de74867df552855e85",
		MkRoot:         "0x0a975f5cf517899e6116000fd366dc0feb34a2ea1b64e9b213278442dd9852fe",
		NumberOfBlocks: 1000,
		ProcessID:      "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105",
		ProcessType:    types.PetitionSign,
		Signature:      "845289a58a4d5292f8353cc29055f5f5ca92579ec4afc5c589107023f8c0d87055f75e2b91a47a15e7d4cc8294de422cf229a5b5731d989a7b439fffa2ee141b01",
		Type:           "newProcess",
	}

	HardcodedCancelProcessTx = &types.CancelProcessTx{
		ProcessID: "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105",
		Signature: "685707c15ed3c0ab71bdf75157a724f340ffaba716cc9d7d9a317acd7f62e9fa739c201f270cced59c405d18dc945360a498a8a84132f9920b83ba08673996e01c",
		Type:      "cancelProcess",
	}

	HardcodedNewVoteTx = &types.VoteTx{
		Nonce:       "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0",
		Nullifier:   "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0",
		ProcessID:   "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105",
		Proof:       "00030000000000000000000000000000000000000000000000000000000000070ab34471caaefc9bb249cb178335f367988c159f3907530ef7daa1e1bf0c9c7a218f981be7c0c46ffa345d291abb36a17c22722814fb0110240b8640fd1484a6268dc2f0fc2152bf83c06566fbf155f38b8293033d4779a63bba6c7157fd10c8",
		Signature:   "773de3c55da3e355337ab0632ebd3da0b0eecc3dfa01149460b18df46b2a3a7e1ac8168e2db134e2e6abcb1dd3c328cabfdbd047aa602187992250128d24397e1b",
		Type:        "vote",
		VotePackage: "eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19",
	}

	HardcodedAdminTxAddOracle = &types.AdminTx{
		Address:   "39106af1fF18bD60a38a296fd81B1f28f315852B", // oracle address or pubkey validator
		Nonce:     "0x1",
		Signature: "11ccdaacd6b6c2c832ea51b4dc695ce9f3c31b7fecd81a2509e7daf183a126e974f1b68060dd406c83ea2db1147d7a56fd6033e8cf7834ce0cf5ec504f09f2ee1b",
		Type:      "addOracle",
	}

	HardcodedAdminTxRemoveOracle = &types.AdminTx{
		Address:   "00192Fb10dF37c9FB26829eb2CC623cd1BF599E8",
		Nonce:     "0x1",
		Signature: "70f89a73f2b7a712e1281e49758ea7fa32769666b38773eeff5a3a0f0e20b6c46b5bb05d9257c9156bf7e7b7334b0af9cb38bc0ae19c70d4f64633529a49585d1b",
		Type:      "removeOracle",
	}

	HardcodedAdminTxAddValidator = &types.AdminTx{
		Address:   "GyZfKNK3lT5AQXQ4pwrVdgG3rRisx9tS4bM9EIZ0zYY=",
		Nonce:     "0x1",
		Power:     10,
		Signature: "0c869d72bd7d8d9df538f68d506c93ff47e7e6bd7f3c6462e45d4926b061501f23b6631bfc3c89c5e5b02dd6f0bf19f576ab982a8065b18ca961868097daf61f1c",
		Type:      "addValidator",
	}

	HardcodedAdminTxRemoveValidator = &types.AdminTx{
		Address:   "5DC922017285EC24415F3E7ECD045665EADA8B5A",
		Nonce:     "0x1",
		Signature: "777fde5f25337e70c463815513f3cf4f2d78aaf00d5f02ac7371cf419387569952ac98e3f7be8b9ce0911508ae73547a0cf3d3a443602f13c3e0a7009b4dce581c",
		Type:      "removeValidator",
	}
)

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
	s, err := vochain.NewState(TempDir(tb, "vochain-db"), c)
	if err != nil {
		tb.Fatal(err)
	}
	oraclesBytes, err := s.Codec.MarshalBinaryBare(OracleListHardcoded)
	if err != nil {
		tb.Fatal(err)
	}
	s.AppTree.Set([]byte("oracle"), oraclesBytes)
	return s
}

func NewVochainStateWithValidators(tb testing.TB) *vochain.State {
	c := amino.NewCodec()
	cryptoAmino.RegisterAmino(c)
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
	s.AppTree.Set([]byte("validator"), validatorsBytes)
	oraclesBytes, err := s.Codec.MarshalBinaryBare(OracleListHardcoded)
	if err != nil {
		tb.Fatal(err)
	}
	s.AppTree.Set([]byte("oracle"), oraclesBytes)
	return s
}

func NewVochainStateWithProcess(tb testing.TB) *vochain.State {
	c := amino.NewCodec()
	s, err := vochain.NewState(TempDir(tb, "vochain-db"), c)
	if err != nil {
		tb.Fatal(err)
	}
	// add process
	processBytes, err := s.Codec.MarshalBinaryBare(ProcessHardcoded)
	if err != nil {
		tb.Fatal(err)
	}
	s.ProcessTree.Set([]byte("e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), processBytes)
	return s
}

func NewMockVochainNode(tb testing.TB, d *DvoteAPIServer) *vochain.BaseApplication {
	cdc := amino.NewCodec()
	cryptoAmino.RegisterAmino(cdc)
	// start vochain node
	// create config
	d.VochainCfg = new(config.VochainCfg)
	d.VochainCfg.DataDir = TempDir(tb, "vochain-data")
	// create genesis file
	consensusParams := tmtypes.DefaultConsensusParams()
	// TO-DO instead of creating a pv file, just create a random 64 bytes key and use it for the genesis file
	validator := privval.GenFilePV(d.VochainCfg.DataDir+"/config/priv_validator_key.json", d.VochainCfg.DataDir+"/data/priv_validator_state.json")
	oracles := []string{d.Signer.EthAddrString()}
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
	// create vochain rpc conection
	d.VochainRPCClient, err = voclient.NewHTTP("tcp://"+d.VochainCfg.RPCListen, "/websocket")
	if err != nil {
		tb.Fatal(err)
	}
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
		s[i] = new(ethereum.SignKeys)
		if err := s[i].Generate(); err != nil {
			tb.Fatal(err)
		}
	}
	return s
}
