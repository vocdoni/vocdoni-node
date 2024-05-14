package genesis

import (
	"encoding/json"
	"time"

	comettypes "github.com/cometbft/cometbft/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
)

// genesis is a map containing the default GenesisDoc for each network
var genesis = map[string]comettypes.GenesisDoc{
	// testsuite test network
	"test": testGenesis,

	// Development network
	"dev": devGenesis,

	// Staging network
	"stage": stageGenesis,

	// LTS production network
	"lts": ltsGenesis,
}

var testGenesis = comettypes.GenesisDoc{
	GenesisTime: time.Date(2024, time.May, 7, 1, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni/TEST/1",
	ConsensusParams: &comettypes.ConsensusParams{
		Block:     DefaultBlockParams(),
		Evidence:  comettypes.DefaultEvidenceParams(),
		Validator: DefaultValidatorParams(),
		Version: comettypes.VersionParams{
			App: 1,
		},
	},
	AppState: jsonRawMessage(AppState{
		MaxElectionSize: 1000000,
		NetworkCapacity: 10000,
		Validators: []AppStateValidators{
			{ // 0
				Address:  ethereumAddrFromPubKey("038faa051e8a726597549bb057f1d296947bb54378443ec8fce030001ece678e14"),
				PubKey:   types.HexStringToHexBytes("038faa051e8a726597549bb057f1d296947bb54378443ec8fce030001ece678e14"),
				Power:    10,
				Name:     "validator1",
				KeyIndex: 1,
			},
		},
		Accounts: []Account{
			{ // faucet
				Address: types.HexStringToHexBytes("0x88a499cEf9D1330111b41360173967c9C1bf703f"),
				Balance: 1000000000000,
			},
		},
		TxCost: TransactionCosts{
			SetProcessStatus:        1,
			SetProcessCensus:        1,
			SetProcessQuestionIndex: 1,
			RegisterKey:             1,
			NewProcess:              10,
			SendTokens:              2,
			SetAccountInfoURI:       2,
			CreateAccount:           2,
			AddDelegateForAccount:   2,
			DelDelegateForAccount:   2,
			CollectFaucet:           1,
			SetAccountSIK:           1,
			DelAccountSIK:           1,
			SetAccountValidator:     100,
		},
	}),
}

var devGenesis = comettypes.GenesisDoc{
	GenesisTime: time.Date(2024, time.April, 4, 1, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni/DEV/33",
	ConsensusParams: &comettypes.ConsensusParams{
		Block:     DefaultBlockParams(),
		Evidence:  comettypes.DefaultEvidenceParams(),
		Validator: DefaultValidatorParams(),
		Version: comettypes.VersionParams{
			App: 1,
		},
	},
	AppState: jsonRawMessage(AppState{
		MaxElectionSize: 100000,
		NetworkCapacity: 20000,
		Validators: []AppStateValidators{
			{ // 0
				Address:  ethereumAddrFromPubKey("03c61c8399828b0c5644455e43c946979272dc3ca0859267f798268802303015f7"),
				PubKey:   types.HexStringToHexBytes("03c61c8399828b0c5644455e43c946979272dc3ca0859267f798268802303015f7"),
				Power:    10,
				Name:     "",
				KeyIndex: 1,
			},
			{ // 1
				Address:  ethereumAddrFromPubKey("0383fe95c5fddee9932ef0f77c180c3c5d0357dba566f2ee77de666a64d9d8c2a6"),
				PubKey:   types.HexStringToHexBytes("0383fe95c5fddee9932ef0f77c180c3c5d0357dba566f2ee77de666a64d9d8c2a6"),
				Power:    10,
				Name:     "",
				KeyIndex: 2,
			},
			{ // 2
				Address:  ethereumAddrFromPubKey("03503c0872bdcd804b1635cf187577ca1caddbbb14ec8eb3af68579fe4bedcf071"),
				PubKey:   types.HexStringToHexBytes("03503c0872bdcd804b1635cf187577ca1caddbbb14ec8eb3af68579fe4bedcf071"),
				Power:    10,
				Name:     "",
				KeyIndex: 3,
			},
			{ // 3
				Address:  ethereumAddrFromPubKey("02159b8dd9b1cea02cd0ff78ae26dc8aa4efc65f46511537d8550fe1ce407100c3"),
				PubKey:   types.HexStringToHexBytes("02159b8dd9b1cea02cd0ff78ae26dc8aa4efc65f46511537d8550fe1ce407100c3"),
				Power:    10,
				Name:     "",
				KeyIndex: 4,
			},
		},
		Accounts: []Account{
			{ // faucet
				Address: types.HexStringToHexBytes("0x7C3a4A5f142ed27C07966b7C7C3F085521154b40"),
				Balance: 1000000000,
			},
			{ // faucet2
				Address: types.HexStringToHexBytes("0x536Da9ecd65Fc0248625b0BBDbB305d0DD841893"),
				Balance: 100000000,
			},
			{ // faucet3
				Address: types.HexStringToHexBytes("0x15A052aA90350BA95038A89A470117A9b9c35960"),
				Balance: 100000000,
			},
		},
		TxCost: TransactionCosts{
			SetProcessStatus:        2,
			SetProcessCensus:        2,
			SetProcessQuestionIndex: 1,
			RegisterKey:             1,
			NewProcess:              5,
			SendTokens:              1,
			SetAccountInfoURI:       1,
			CreateAccount:           1,
			AddDelegateForAccount:   1,
			DelDelegateForAccount:   1,
			CollectFaucet:           1,
			SetAccountSIK:           1,
			DelAccountSIK:           1,
			SetAccountValidator:     10000,
		},
	}),
}

var stageGenesis = comettypes.GenesisDoc{
	GenesisTime: time.Date(2024, time.January, 30, 1, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni/STAGE/11",
	ConsensusParams: &comettypes.ConsensusParams{
		Block:     DefaultBlockParams(),
		Evidence:  comettypes.DefaultEvidenceParams(),
		Validator: DefaultValidatorParams(),
		Version: comettypes.VersionParams{
			App: 1,
		},
	},
	AppState: jsonRawMessage(AppState{
		MaxElectionSize: 500000,
		NetworkCapacity: 10000,
		Validators: []AppStateValidators{
			{ // 0
				Address:  ethereumAddrFromPubKey("02420b2ee645b9509453cd3b99a6bd8e5e10c1d746fb0bb0ac5af79aba19bb9784"),
				PubKey:   types.HexStringToHexBytes("02420b2ee645b9509453cd3b99a6bd8e5e10c1d746fb0bb0ac5af79aba19bb9784"),
				Power:    10,
				Name:     "vocdoni1",
				KeyIndex: 1,
			},
			{ // 1
				Address: ethereumAddrFromPubKey("03e6c55195825f9736ce8a4553913bbadb26c7f094540e06aed9ccda0e6e26050d"),
				PubKey:  types.HexStringToHexBytes("03e6c55195825f9736ce8a4553913bbadb26c7f094540e06aed9ccda0e6e26050d"),
				Power:   10,
				Name:    "vocdoni2",
			},
			{ // 2
				Address: ethereumAddrFromPubKey("03cb39e1132eee0b25ec75d7dad1f2885460f9b2f200d108a923b78e648b783839"),
				PubKey:  types.HexStringToHexBytes("03cb39e1132eee0b25ec75d7dad1f2885460f9b2f200d108a923b78e648b783839"),
				Power:   10,
				Name:    "vocdoni3",
			},
			{ // 3
				Address:  ethereumAddrFromPubKey("03f6c246831a524e8214e9ceb61d3da2c3c4dbee09bcbe5d9d9878aaa085764d65"),
				PubKey:   types.HexStringToHexBytes("03f6c246831a524e8214e9ceb61d3da2c3c4dbee09bcbe5d9d9878aaa085764d65"),
				Power:    10,
				Name:     "vocdoni4",
				KeyIndex: 2,
			},
			{ // 4
				Address: ethereumAddrFromPubKey("02fd283ff5760958b4e59eac6b0647ed002669ef2862eb9361251376160aa72fe5"),
				PubKey:  types.HexStringToHexBytes("02fd283ff5760958b4e59eac6b0647ed002669ef2862eb9361251376160aa72fe5"),
				Power:   10,
				Name:    "vocdoni5",
			},
			{ // 5
				Address:  ethereumAddrFromPubKey("03369a8c595c70526baf8528b908591ec286e910b10796c3d6dfca0ef76a645167"),
				PubKey:   types.HexStringToHexBytes("03369a8c595c70526baf8528b908591ec286e910b10796c3d6dfca0ef76a645167"),
				Power:    10,
				Name:     "vocdoni6",
				KeyIndex: 3,
			},
			{ // 6
				Address: ethereumAddrFromPubKey("02b5005aeefdb8bb196d308df3fba157a7c1e84966f899a9def6aa97b086bc87e7"),
				PubKey:  types.HexStringToHexBytes("02b5005aeefdb8bb196d308df3fba157a7c1e84966f899a9def6aa97b086bc87e7"),
				Power:   10,
				Name:    "vocdoni7",
			},
		},
		Accounts: []Account{
			{ // faucet
				Address: types.HexStringToHexBytes("C7C6E17059801b6962cc144a374eCc3ba1b8A9e0"),
				Balance: 1000000000,
			},
		},
		TxCost: TransactionCosts{
			SetProcessStatus:        2,
			SetProcessCensus:        1,
			SetProcessQuestionIndex: 1,
			RegisterKey:             1,
			NewProcess:              5,
			SendTokens:              1,
			SetAccountInfoURI:       1,
			CreateAccount:           1,
			AddDelegateForAccount:   1,
			DelDelegateForAccount:   1,
			CollectFaucet:           1,
			SetAccountSIK:           1,
			DelAccountSIK:           1,
			SetAccountValidator:     500000,
		},
	}),
}

var ltsGenesis = comettypes.GenesisDoc{
	GenesisTime: time.Date(2024, time.April, 24, 9, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni/LTS/1.2",
	ConsensusParams: &comettypes.ConsensusParams{
		Block:     DefaultBlockParams(),
		Evidence:  comettypes.DefaultEvidenceParams(),
		Validator: DefaultValidatorParams(),
		Version: comettypes.VersionParams{
			App: 0,
		},
	},
	AppState: jsonRawMessage(AppState{
		MaxElectionSize: 1000000,
		NetworkCapacity: 5000,
		Validators: []AppStateValidators{
			{ // 0
				Address:  ethereumAddrFromPubKey("024e3fbcd7e1516ebbc332519a3602e39753c6dd49c46df307c1e60b976f0b29a5"),
				PubKey:   types.HexStringToHexBytes("024e3fbcd7e1516ebbc332519a3602e39753c6dd49c46df307c1e60b976f0b29a5"),
				Power:    10,
				Name:     "vocdoni-validator0",
				KeyIndex: 1,
			},
			{ // 1
				Address: ethereumAddrFromPubKey("02364db3aedf05ffbf25e67e81de971f3a9965b9e1a2d066af06b634ba5c959152"),
				PubKey:  types.HexStringToHexBytes("02364db3aedf05ffbf25e67e81de971f3a9965b9e1a2d066af06b634ba5c959152"),
				Power:   10,
				Name:    "vocdoni-validator1",
			},
			{ // 2
				Address: ethereumAddrFromPubKey("037a2e3b3e7ae07cb75dbc73aff9c39b403e0ec58b596cf03fe99a27555285ef73"),
				PubKey:  types.HexStringToHexBytes("037a2e3b3e7ae07cb75dbc73aff9c39b403e0ec58b596cf03fe99a27555285ef73"),
				Power:   10,
				Name:    "vocdoni-validator2",
			},
			{ // 3
				Address:  ethereumAddrFromPubKey("03553d1b75cdda0a49136417daee453c3a00ed75af64ec6aa20476cf227dfd946c"),
				PubKey:   types.HexStringToHexBytes("03553d1b75cdda0a49136417daee453c3a00ed75af64ec6aa20476cf227dfd946c"),
				Power:    10,
				Name:     "vocdoni-validator3",
				KeyIndex: 2,
			},
			{ // 4
				Address: ethereumAddrFromPubKey("036e25b61605a04ef3cf5829e73a2c9db4a4b0958a8a6be0895c3df19b69e7ad45"),
				PubKey:  types.HexStringToHexBytes("036e25b61605a04ef3cf5829e73a2c9db4a4b0958a8a6be0895c3df19b69e7ad45"),
				Power:   10,
				Name:    "vocdoni-validator4",
			},
			{ // 5
				Address:  ethereumAddrFromPubKey("027b034a05be20113cdf39eff609c5265d1575c5510bf3fcc611e6da0bed6d30b4"),
				PubKey:   types.HexStringToHexBytes("027b034a05be20113cdf39eff609c5265d1575c5510bf3fcc611e6da0bed6d30b4"),
				Power:    10,
				Name:     "vocdoni-validator5",
				KeyIndex: 3,
			},
			{ // 6
				Address: ethereumAddrFromPubKey("034105acd3392dffcfe08a7a2e1c48fb4f52c7f4cdce4477474afc0ddff023ec2d"),
				PubKey:  types.HexStringToHexBytes("034105acd3392dffcfe08a7a2e1c48fb4f52c7f4cdce4477474afc0ddff023ec2d"),
				Power:   10,
				Name:    "vocdoni-validator6",
			},
			{ // 7
				Address: ethereumAddrFromPubKey("038276c348971ef9d8b11abaf0cdce50e6cb89bd0f87df14301ef02d46db09db6d"),
				PubKey:  types.HexStringToHexBytes("038276c348971ef9d8b11abaf0cdce50e6cb89bd0f87df14301ef02d46db09db6d"),
				Power:   10,
				Name:    "vocdoni-validator7",
			},
			{ // 8
				Address:  ethereumAddrFromPubKey("02a94d4a25c0281980af65d014ce72d34b0aba6e5dff362da8b34c31e8b93b26a9"),
				PubKey:   types.HexStringToHexBytes("02a94d4a25c0281980af65d014ce72d34b0aba6e5dff362da8b34c31e8b93b26a9"),
				Power:    10,
				Name:     "vocdoni-validator8",
				KeyIndex: 4,
			},
		},
		Accounts: []Account{
			{ // treasury
				Address: types.HexStringToHexBytes("863a75f41025f0c8878d3a100c8c16576fe8fe4f"),
				Balance: 1000000000,
			},
			{ // faucet
				Address: types.HexStringToHexBytes("4Ca9F2Dc015Df06BFE1ed19F96bCB92ECF612a76"),
				Balance: 10000000,
			},
		},
		TxCost: TransactionCosts{
			SetProcessStatus:        1,
			SetProcessCensus:        5,
			SetProcessQuestionIndex: 1,
			RegisterKey:             1,
			NewProcess:              10,
			SendTokens:              1,
			SetAccountInfoURI:       5,
			CreateAccount:           1,
			AddDelegateForAccount:   1,
			DelDelegateForAccount:   1,
			CollectFaucet:           1,
			SetAccountSIK:           1,
			DelAccountSIK:           1,
			SetAccountValidator:     500000,
		},
	}),
}

// DefaultBlockParams returns different defaults than upstream DefaultBlockParams:
// MaxBytes = 2 megabytes, and MaxGas = -1
func DefaultBlockParams() comettypes.BlockParams {
	return comettypes.BlockParams{
		MaxBytes: 2097152,
		MaxGas:   -1,
	}
}

// DefaultValidatorParams returns different defaults than upstream DefaultValidatorParams:
// allows only secp256k1 pubkeys.
func DefaultValidatorParams() comettypes.ValidatorParams {
	return comettypes.ValidatorParams{
		PubKeyTypes: []string{comettypes.ABCIPubKeyTypeSecp256k1},
	}
}

// AvailableNetworks returns the list of hardcoded networks
func AvailableNetworks() []string {
	networks := []string{}
	for k := range genesis {
		networks = append(networks, k)
	}
	return networks
}

// HardcodedForNetwork returns the hardcoded genesis.Doc of a specific network. Panics if not found
func HardcodedForNetwork(network string) *Doc {
	g, ok := genesis[network]
	if !ok {
		panic("no hardcoded genesis found for current network")
	}
	if err := g.ValidateAndComplete(); err != nil {
		panic("hardcoded genesis is invalid")
	}
	return &Doc{g}
}

// LoadFromFile loads and unmarshals a genesis.json, returning a genesis.Doc
func LoadFromFile(path string) (*Doc, error) {
	gd, err := comettypes.GenesisDocFromFile(path)
	if err != nil {
		return nil, err
	}
	return &Doc{*gd}, nil
}

// ethereumAddrFromPubKey converts a hex string to a ethcommon.Address and returns its Bytes().
// It strips a leading '0x' or '0X' if found, for backwards compatibility.
// Panics if the string is not a valid hex string.
func ethereumAddrFromPubKey(hexString string) []byte {
	addr, err := ethereum.AddrFromPublicKey(types.HexStringToHexBytes(hexString))
	if err != nil {
		panic(err)
	}
	return addr.Bytes()
}

// jsonRawMessage marshals the appState into a json.RawMessage.
// Panics on error.
func jsonRawMessage(appState AppState) json.RawMessage {
	jrm, err := json.Marshal(appState)
	if err != nil {
		// must never happen
		panic(err)
	}
	return jrm
}
