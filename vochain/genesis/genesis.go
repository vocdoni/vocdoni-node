package genesis

import (
	"time"

	"go.vocdoni.io/dvote/types"
)

// Genesis is a map containing the defaut Genesis details
var Genesis = map[string]VochainGenesis{

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
}

var devGenesis = GenesisDoc{
	GenesisTime: time.Date(2023, time.March, 10, 10, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni-dev-3",
	ConsensusParams: &ConsensusParams{
		Block: BlockParams{
			MaxBytes: 5242880,
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
	AppState: GenesisAppState{
		MaxElectionSize: 20000,
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
		Oracles: []types.HexBytes{
			// oracle1
			types.HexStringToHexBytes("0xb926be24A9ca606B515a835E91298C7cF0f2846f"),
			// oracle2
			types.HexStringToHexBytes("0x4a081070E9D555b5D19629a6bcc8B77f4aE6d39c"),
		},
		Accounts: []GenesisAccount{
			{ // oracle1
				Address: types.HexStringToHexBytes("0xb926be24A9ca606B515a835E91298C7cF0f2846f"),
				Balance: 10000,
			},
			{ // oracle2
				Address: types.HexStringToHexBytes("0x4a081070E9D555b5D19629a6bcc8B77f4aE6d39c"),
				Balance: 10000,
			},
			{ // faucet
				Address: types.HexStringToHexBytes("0xC7C6E17059801b6962cc144a374eCc3ba1b8A9e0"),
				Balance: 1000000,
			},
		},
		Treasurer: types.HexStringToHexBytes("0x309Bd6959bf4289CDf9c7198cF9f4494e0244b7d"),
		TxCost: TransactionCosts{
			SetProcessStatus:        1,
			SetProcessCensus:        1,
			SetProcessResults:       1,
			SetProcessQuestionIndex: 1,
			RegisterKey:             1,
			NewProcess:              5,
			SendTokens:              1,
			SetAccountInfoURI:       1,
			CreateAccount:           1,
			AddDelegateForAccount:   1,
			DelDelegateForAccount:   1,
			CollectFaucet:           1,
		},
	},
}

var stageGenesis = GenesisDoc{
	GenesisTime: time.Date(2023, time.March, 10, 10, 0, 0, 0, time.UTC),
	ChainID:     "vocdoni-stage-3",
	ConsensusParams: &ConsensusParams{
		Block: BlockParams{
			MaxBytes: 5242880,
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
	AppState: GenesisAppState{
		MaxElectionSize: 1000,
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
		Oracles: []types.HexBytes{
			// oracle1
			types.HexStringToHexBytes("0x81ff945dda4b94690a13f49fdc8f0819970b2db0"),
			// oracle2
			types.HexStringToHexBytes("0x08acAbAfc667c21a82b07C87A269E701381641FC"),
		},
		Accounts: []GenesisAccount{
			{ // oracle1
				Address: types.HexStringToHexBytes("0x81ff945dda4b94690a13f49fdc8f0819970b2db0"),
				Balance: 10000,
			},
			{ // oracle2
				Address: types.HexStringToHexBytes("0x08acAbAfc667c21a82b07C87A269E701381641FC"),
				Balance: 10000,
			},
			{ // faucet
				Address: types.HexStringToHexBytes("0xC7C6E17059801b6962cc144a374eCc3ba1b8A9e0"),
				Balance: 1000000,
			},
		},
		Treasurer: types.HexStringToHexBytes("0x309Bd6959bf4289CDf9c7198cF9f4494e0244b7d"),
		TxCost: TransactionCosts{
			SetProcessStatus:        1,
			SetProcessCensus:        1,
			SetProcessResults:       1,
			SetProcessQuestionIndex: 1,
			RegisterKey:             1,
			NewProcess:              5,
			SendTokens:              1,
			SetAccountInfoURI:       1,
			CreateAccount:           1,
			AddDelegateForAccount:   1,
			DelDelegateForAccount:   1,
			CollectFaucet:           1,
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
