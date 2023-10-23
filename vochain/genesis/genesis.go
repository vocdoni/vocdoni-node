package genesis

import (
	"time"

	"go.vocdoni.io/dvote/types"
)

// Genesis is a map containing the default Genesis details
var Genesis = map[string]Vochain{
	// Development network
	"dev": {
		AutoUpdateGenesis: true,
		SeedNodes: []string{
			"7440a5b086e16620ce7b13198479016aa2b07988@seed.dev.vocdoni.net:26656"},
		CircuitsConfigTag: "dev",
		Genesis:           &devGenesis,
	},

	// Staging network
	"stage": {
		AutoUpdateGenesis: true,
		SeedNodes: []string{
			"588133b8309363a2a852e853424251cd6e8c5330@seed.stg.vocdoni.net:26656"},
		CircuitsConfigTag: "dev",
		Genesis:           &stageGenesis,
	},

	// LTS production network
	"lts": {
		AutoUpdateGenesis: false,
		SeedNodes: []string{
			"32acbdcda649fbcd35775f1dd8653206d940eee4@seed1.lts.vocdoni.net:26656",
			"02bfac9bd98bf25429d12edc50552cca5e975080@seed2.lts.vocdoni.net:26656",
		},
		CircuitsConfigTag: "prod",
		Genesis:           &ltsGenesis,
	},
}

var devGenesis = Doc{
	GenesisTime: time.Date(2023, time.October, 17, 1, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni-dev-25",
	ConsensusParams: &ConsensusParams{
		Block: BlockParams{
			MaxBytes: 2097152,
			MaxGas:   -1,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: 100000,
			MaxAgeDuration:  10000,
		},
		Validator: ValidatorParams{
			PubKeyTypes: []string{"secp256k1"},
		},
		Version: VersionParams{
			AppVersion: 0,
		},
	},
	AppState: AppState{
		MaxElectionSize: 100000,
		NetworkCapacity: 20000,
		Validators: []AppStateValidators{
			{ // 0
				Address:  types.HexStringToHexBytes("04cc36be85a0a6e2bfd09295396625e6302d7c60"),
				PubKey:   types.HexStringToHexBytes("03c61c8399828b0c5644455e43c946979272dc3ca0859267f798268802303015f7"),
				Power:    10,
				Name:     "",
				KeyIndex: 1,
			},
			{ // 1
				Address:  types.HexStringToHexBytes("fc095a35338d96503b6fd1010475e45a3545fc25"),
				PubKey:   types.HexStringToHexBytes("0383fe95c5fddee9932ef0f77c180c3c5d0357dba566f2eeb2f1b2c1f1c1f1c1f1"),
				Power:    10,
				Name:     "",
				KeyIndex: 2,
			},
			{ // 2
				Address:  types.HexStringToHexBytes("a9b1008f17654b36f2a9abd29323c53d344415a0"),
				PubKey:   types.HexStringToHexBytes("03503c0872bdcd804b1635cf187577ca1caddbbb14ec8eb3af68579fe4bedcf071"),
				Power:    10,
				Name:     "",
				KeyIndex: 3,
			},
			{ // 3
				Address:  types.HexStringToHexBytes("234120598e3fcfcfae5d969254d371248b0cf8d1"),
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
	},
}

var stageGenesis = Doc{
	GenesisTime: time.Date(2023, time.September, 21, 1, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni-stage-8",
	ConsensusParams: &ConsensusParams{
		Block: BlockParams{
			MaxBytes: 2097152,
			MaxGas:   -1,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: 100000,
			MaxAgeDuration:  10000,
		},
		Validator: ValidatorParams{
			PubKeyTypes: []string{"secp256k1"},
		},
		Version: VersionParams{
			AppVersion: 0,
		},
	},
	AppState: AppState{
		MaxElectionSize: 50000,
		NetworkCapacity: 2000,
		Validators: []AppStateValidators{
			{ // 0
				Address:  types.HexStringToHexBytes("321d141cf1fcb41d7844af611b5347afc380a03f"),
				PubKey:   types.HexStringToHexBytes("02420b2ee645b9509453cd3b99a6bd8e5e10c1d746fb0bb0ac5af79aba19bb9784"),
				Power:    10,
				Name:     "",
				KeyIndex: 1,
			},
			{ // 1
				Address:  types.HexStringToHexBytes("5e6c49d98ff3b90ca46387d7c583d20cf99f29bd"),
				PubKey:   types.HexStringToHexBytes("03e6c55195825f9736ce8a4553913bbadb26c7f094540e06aed9ccda0e6e26050d"),
				Power:    10,
				Name:     "",
				KeyIndex: 0,
			},
			{ // 2
				Address:  types.HexStringToHexBytes("9d4c46f7485036faea5f15c3034e9e864b9415b5"),
				PubKey:   types.HexStringToHexBytes("03cb39e1132eee0b25ec75d7dad1f2885460f9b2f200d108a923b78e648b783839"),
				Power:    10,
				Name:     "",
				KeyIndex: 0,
			},
			{ // 3
				Address:  types.HexStringToHexBytes("52d74938f81569aba46f384c8108c370b5403585"),
				PubKey:   types.HexStringToHexBytes("03f6c246831a524e8214e9ceb61d3da2c3c4dbee09bcbe5d9d9878aaa085764d65"),
				Power:    10,
				Name:     "",
				KeyIndex: 2,
			},
			{ // 4
				Address:  types.HexStringToHexBytes("ad6ff21ccfb31002adc52714043e37da1b555b15"),
				PubKey:   types.HexStringToHexBytes("02fd283ff5760958b4e59eac6b0647ed002669ef2862eb9361251376160aa72fe5"),
				Power:    10,
				Name:     "",
				KeyIndex: 0,
			},
			{ // 5
				Address:  types.HexStringToHexBytes("8367a1488c3afda043a2a602c13f01d801d0270e"),
				PubKey:   types.HexStringToHexBytes("03369a8c595c70526baf8528b908591ec286e910b10796c3d6dfca0ef76a645167"),
				Power:    10,
				Name:     "",
				KeyIndex: 0,
			},
			{ // 6
				Address:  types.HexStringToHexBytes("4146598ff76009f45903958c4c7a3195683b2f61"),
				PubKey:   types.HexStringToHexBytes("02b5005aeefdb8bb196d308df3fba157a7c1e84966f899a9def6aa97b086bc87e7"),
				Power:    10,
				Name:     "",
				KeyIndex: 3,
			},
			{ // 7
				Address:  types.HexStringToHexBytes("205bd3ae071118849535c9746f577ddd5eb226e6"),
				PubKey:   types.HexStringToHexBytes("02cc2386b56cd46c196d33ec23cc9ad5228942fabd62e1cdbc12b4688fa3ac49e0"),
				Power:    10,
				Name:     "",
				KeyIndex: 0,
			},
			{ // 8
				Address:  types.HexStringToHexBytes("96db730d4478eaea44ee2bcb90fc1430a22521cb"),
				PubKey:   types.HexStringToHexBytes("02a059f4f5454555a588ef99b1c6ba525b551a16621d6c476998e4972ef5e42913"),
				Power:    10,
				Name:     "",
				KeyIndex: 0,
			},
			{ // 9
				Address:  types.HexStringToHexBytes("c7864964dab0107eb01a1cade4c3319dae69754e"),
				PubKey:   types.HexStringToHexBytes("022fb2c1b4df15418a268b3d8027809e44d321c640b21ace6f0d5c6fa9fde18989"),
				Power:    10,
				Name:     "",
				KeyIndex: 4,
			},
		},
		Accounts: []Account{
			{ // faucet
				Address: types.HexStringToHexBytes("0xC7C6E17059801b6962cc144a374eCc3ba1b8A9e0"),
				Balance: 1000000000,
			},
		},
		TxCost: TransactionCosts{
			SetProcessStatus:        2,
			SetProcessCensus:        50,
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
			SetAccountValidator:     100000,
		},
	},
}

var ltsGenesis = Doc{
	GenesisTime: time.Date(2023, time.October, 17, 17, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni-lts-v1",
	ConsensusParams: &ConsensusParams{
		Block: BlockParams{
			MaxBytes: 2097152,
			MaxGas:   -1,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: 100000,
			MaxAgeDuration:  10000,
		},
		Validator: ValidatorParams{
			PubKeyTypes: []string{"secp256k1"},
		},
		Version: VersionParams{
			AppVersion: 0,
		},
	},
	AppState: AppState{
		MaxElectionSize: 1000000,
		NetworkCapacity: 5000,
		Validators: []AppStateValidators{
			{ // 0
				Address:  types.HexStringToHexBytes("8a67aa6e63ea24a029fade79b93f39aa2f935608"),
				PubKey:   types.HexStringToHexBytes("024e3fbcd7e1516ebbc332519a3602e39753c6dd49c46df307c1e60b976f0b29a5"),
				Power:    10,
				Name:     "vocdoni-validator0",
				KeyIndex: 1,
			},
			{ // 1
				Address:  types.HexStringToHexBytes("dd47c5e9db1be4f9c6fac3474b9d9aec5c00ecdd"),
				PubKey:   types.HexStringToHexBytes("02364db3aedf05ffbf25e67e81de971f3a9965b9e1a2d066af06b634ba5c959152"),
				Power:    10,
				Name:     "vocdoni-validator1",
				KeyIndex: 0,
			},
			{ // 2
				Address:  types.HexStringToHexBytes("6bc0fe0ac7e7371294e3c2d39b0e1337b9757193"),
				PubKey:   types.HexStringToHexBytes("037a2e3b3e7ae07cb75dbc73aff9c39b403e0ec58b596cf03fe99a27555285ef73"),
				Power:    10,
				Name:     "vocdoni-validator2",
				KeyIndex: 0,
			},
			{ // 3
				Address:  types.HexStringToHexBytes("d863a79bb3c019941de5ebfc10a136bbfbbc2982"),
				PubKey:   types.HexStringToHexBytes("03553d1b75cdda0a49136417daee453c3a00ed75af64ec6aa20476cf227dfd946c"),
				Power:    10,
				Name:     "vocdoni-validator3",
				KeyIndex: 2,
			},
			{ // 4
				Address:  types.HexStringToHexBytes("d16a9fe63456ea0b3706a2855b98a3a20f10e308"),
				PubKey:   types.HexStringToHexBytes("036e25b61605a04ef3cf5829e73a2c9db4a4b0958a8a6be0895c3df19b69e7ad45"),
				Power:    10,
				Name:     "vocdoni-validator4",
				KeyIndex: 0,
			},
			{ // 5
				Address:  types.HexStringToHexBytes("4ece41dd2b0f0ddd4ddef9fa83ad6d973c98a48c"),
				PubKey:   types.HexStringToHexBytes("027b034a05be20113cdf39eff609c5265d1575c5510bf3fcc611e6da0bed6d30b4"),
				Power:    10,
				Name:     "vocdoni-validator5",
				KeyIndex: 3,
			},
			{ // 6
				Address:  types.HexStringToHexBytes("2b617bad95bb36805512c76a02144c778d3cda20"),
				PubKey:   types.HexStringToHexBytes("034105acd3392dffcfe08a7a2e1c48fb4f52c7f4cdce4477474afc0ddff023ec2d"),
				Power:    10,
				Name:     "vocdoni-validator6",
				KeyIndex: 0,
			},
			{ // 7
				Address:  types.HexStringToHexBytes("4945fd40d29870a931561b26a30a529081ded677"),
				PubKey:   types.HexStringToHexBytes("038276c348971ef9d8b11abaf0cdce50e6cb89bd0f87df14301ef02d46db09db6d"),
				Power:    10,
				Name:     "vocdoni-validator7",
				KeyIndex: 0,
			},
			{ // 8
				Address:  types.HexStringToHexBytes("34868fa6ef1b001b830a5a19a06c69f62f622410"),
				PubKey:   types.HexStringToHexBytes("02a94d4a25c0281980af65d014ce72d34b0aba6e5dff362da8b34c31e8b93b26a9"),
				Power:    10,
				Name:     "vocdoni-validator8",
				KeyIndex: 4,
			},
		},
		Accounts: []Account{
			{ // faucet
				Address: types.HexStringToHexBytes("863a75f41025f0c8878d3a100c8c16576fe8fe4f"),
				Balance: 1000000000,
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
			SetAccountValidator:     200000,
		},
	},
}

// AvailableChains returns the list of hardcoded chains
func AvailableChains() []string {
	chains := []string{}
	for k := range Genesis {
		chains = append(chains, k)
	}
	return chains
}
