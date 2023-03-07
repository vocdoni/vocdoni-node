package genesis

// VochainGenesis is a struct containing the genesis details
type VochainGenesis struct {
	AutoUpdateGenesis bool
	SeedNodes         []string
	CircuitsConfigTag string
	Genesis           string
}

// GenesisAvailableChains returns the list of hardcoded chains
func GenesisAvailableChains() []string {
	chains := []string{}
	for k := range Genesis {
		chains = append(chains, k)
	}
	return chains
}

// Genesis is a map containing the defaut Genesis details
var Genesis = map[string]VochainGenesis{
	// Development network
	"dev": {
		AutoUpdateGenesis: true,
		SeedNodes: []string{
			"7440a5b086e16620ce7b13198479016aa2b07988@seed.dev.vocdoni.net:26656"},
		CircuitsConfigTag: "dev",
		Genesis: `
{
   "genesis_time": "2023-03-01T22:35:46.527310402Z",
   "chain_id": "vocdoni-dev-1",
   "initial_height": "0",
   "consensus_params": {
     "block": {
       "max_bytes": "5242880",
       "max_gas": "-1"
     },
     "evidence": {
       "max_age_num_blocks": "100000",
       "max_age_duration": "10000",
       "max_bytes": "1048576"
     },
     "validator": {
       "pub_key_types": [
         "secp256k1"
       ]
     },
     "version": {
       "app_version": "0"
     }
   },
   "app_hash": "",
   "app_state": {
     "validators": [
       {
         "signer_address": "04cc36be85a0a6e2bfd09295396625e6302d7c60",
         "consensus_pub_key": "03c61c8399828b0c5644455e43c946979272dc3ca0859267f798268802303015f7",
         "power": 10,
         "name": "",
         "key_index": 1
       },
       {
         "signer_address": "fc095a35338d96503b6fd1010475e45a3545fc25",
         "consensus_pub_key": "0383fe95c5fddee9932ef0f77c180c3c5d0357dba566f2ee77de666a64d9d8c2a6",
         "power": 10,
         "name": "",
         "key_index": 2
       },
       {
         "signer_address": "a9b1008f17654b36f2a9abd29323c53d344415a0",
         "consensus_pub_key": "03503c0872bdcd804b1635cf187577ca1caddbbb14ec8eb3af68579fe4bedcf071",
         "power": 10,
         "name": "",
         "key_index": 3
       },
       {
         "signer_address": "234120598e3fcfcfae5d969254d371248b0cf8d1",
         "consensus_pub_key": "02159b8dd9b1cea02cd0ff78ae26dc8aa4efc65f46511537d8550fe1ce407100c3",
         "power": 10,
         "name": "",
         "key_index": 4
       }
     ],       
      "oracles":[
         "0xb926be24A9ca606B515a835E91298C7cF0f2846f",
         "0x4a081070E9D555b5D19629a6bcc8B77f4aE6d39c"
      ],
      "accounts":[
         { 
            "address":"0xb926be24A9ca606B515a835E91298C7cF0f2846f", 
            "balance":10000 
         },
         { 
            "address":"0x4a081070E9D555b5D19629a6bcc8B77f4aE6d39c", 
            "balance":10000 
         },
         {
            "address": "0xC7C6E17059801b6962cc144a374eCc3ba1b8A9e0",
            "balance": 1000000
         }
      ],
      "treasurer": "0x309Bd6959bf4289CDf9c7198cF9f4494e0244b7d",
      "tx_cost": {
         "Tx_SetProcessStatus": 10,
         "Tx_SetProcessCensus": 10,
         "Tx_SetProcessResults": 10,
         "Tx_SetProcessQuestionIndex": 10,
         "Tx_RegisterKey": 10,
         "Tx_NewProcess": 10,
         "Tx_SendTokens": 10,
         "Tx_CreateAccount": 10,
         "Tx_SetAccountInfoURI": 10,
         "Tx_AddDelegateForAccount": 10,
         "Tx_DelDelegateForAccount": 10,
         "Tx_CollectFaucet": 10
      }
   }
}
`,
	},

	"stage": {
		AutoUpdateGenesis: true,
		SeedNodes: []string{
			"588133b8309363a2a852e853424251cd6e8c5330@seed.stg.vocdoni.net:26656"},
		CircuitsConfigTag: "dev",
		Genesis: `
{
   "genesis_time": "2023-03-01T22:40:44.294386842Z",
   "chain_id": "vocdoni-stage-1",
   "initial_height": "0",
   "consensus_params": {
     "block": {
       "max_bytes": "5242880",
       "max_gas": "-1"
     },
     "evidence": {
       "max_age_num_blocks": "100000",
       "max_age_duration": "10000",
       "max_bytes": "1048576"
     },
     "validator": {
       "pub_key_types": [
         "secp256k1"
       ]
     },
     "version": {
       "app_version": "0"
     }
   },
   "app_hash": "",
   "app_state": {
     "validators": [
       {
         "signer_address": "321d141cf1fcb41d7844af611b5347afc380a03f",
         "consensus_pub_key": "02420b2ee645b9509453cd3b99a6bd8e5e10c1d746fb0bb0ac5af79aba19bb9784",
         "power": 10,
         "name": "",
         "key_index": 1
       },
       {
         "signer_address": "5e6c49d98ff3b90ca46387d7c583d20cf99f29bd",
         "consensus_pub_key": "03e6c55195825f9736ce8a4553913bbadb26c7f094540e06aed9ccda0e6e26050d",
         "power": 10,
         "name": "",
         "key_index": 2
       },
       {
         "signer_address": "9d4c46f7485036faea5f15c3034e9e864b9415b5",
         "consensus_pub_key": "03cb39e1132eee0b25ec75d7dad1f2885460f9b2f200d108a923b78e648b783839",
         "power": 10,
         "name": "",
         "key_index": 3
       },
       {
         "signer_address": "52d74938f81569aba46f384c8108c370b5403585",
         "consensus_pub_key": "03f6c246831a524e8214e9ceb61d3da2c3c4dbee09bcbe5d9d9878aaa085764d65",
         "power": 10,
         "name": "",
         "key_index": 4
       }
     ],       
      "oracles":[
         "0x81ff945dda4b94690a13f49fdc8f0819970b2db0",
         "0x08acAbAfc667c21a82b07C87A269E701381641FC"
      ],
      "accounts":[
         {
            "address":"0x81ff945dda4b94690a13f49fdc8f0819970b2db0",
            "balance":100000
         },
         {
            "address":"0x08acAbAfc667c21a82b07C87A269E701381641FC",
            "balance": 100000
         },
         {
            "address": "0xC7C6E17059801b6962cc144a374eCc3ba1b8A9e0",
            "balance": 1000000
         }
      ],
      "treasurer": "0x309Bd6959bf4289CDf9c7198cF9f4494e0244b7d",
      "tx_cost": {
         "Tx_SetProcessStatus": 1,
         "Tx_SetProcessCensus": 1,
         "Tx_SetProcessResults": 1,
         "Tx_SetProcessQuestionIndex": 1,
         "Tx_RegisterKey": 1,
         "Tx_NewProcess": 5,
         "Tx_SendTokens": 1,
         "Tx_CreateAccount": 1,
         "Tx_SetAccountInfoURI": 1,
         "Tx_AddDelegateForAccount": 1,
         "Tx_DelDelegateForAccount": 1,
         "Tx_CollectFaucet": 1
       }
   }
}
`,
	},
}
