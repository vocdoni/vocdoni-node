package genesis

import (
	"encoding/hex"
	"strings"

	"go.vocdoni.io/dvote/crypto/zk/artifacts"
	"go.vocdoni.io/dvote/log"
)

// VochainGenesis is a struct containing the genesis details
type VochainGenesis struct {
	AutoUpdateGenesis bool
	SeedNodes         []string
	CircuitsConfig    []artifacts.CircuitConfig
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

	// Bizono production Network
	"bizono": {
		AutoUpdateGenesis: false,
		SeedNodes:         []string{"1612de9353b4bd5891981c69f554e56e07733870@seed.azeno.vocdoni.net:26656"},
		CircuitsConfig: []artifacts.CircuitConfig{
			{ // index: 0, size: 1024
				URI: "https://raw.githubusercontent.com/vocdoni/" +
					"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
				CircuitPath: "zkcensusproof/dev/1024",
				Parameters:  []int64{1024},
				LocalDir:    "zkCircuits",
				ZKeyHash:    hexToBytes("0x1cd0c9225210700d4d6307493bbe5f98554e29339daba6d9bd08a4e0e78df443"),
				WitnessHash: hexToBytes("0x61b40e11ece8de3fbfaf27dbd984e0e0b1fa05ee72d4faa0c2be06c1d7a9b845"),
				VKHash:      hexToBytes("0xaed892ff98ab37b877cfcb678cb5f48f1be9d09dbbaf74b5877f46b54d10f9ad"),
			},
			{ // index: 1, size: 65k
				URI: "https://raw.githubusercontent.com/vocdoni/" +
					"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
				CircuitPath: "zkcensusproof/dev/65536",
				Parameters:  []int64{65536},
				LocalDir:    "zkCircuits",
				ZKeyHash:    hexToBytes("0xb7fb6f74ecf56e41de103e679c76c45a1bde99e2203b2ab6928396020f4d4ab6"),
				WitnessHash: hexToBytes("0x1d975d68220d1f10bd54e2f53ea9526ce8f916efb15a2079edc3db9403a78278"),
				VKHash:      hexToBytes("0x50029154e81a2078eff83751454bb3ece2cf9391103cc17306d47f7d4461b0b6"),
			},
		},
		Genesis: `
      {
         "genesis_time": "2022-11-10T17:00:33.672114557Z",
         "chain_id": "bizono",
         "consensus_params": {
           "block": {
             "max_bytes": "5242880",
             "max_gas": "-1",
             "time_iota_ms": "10000"
           },
           "evidence": {
             "max_age_num_blocks": "100000",
             "max_age_duration": "10000"
           },
           "validator": {
             "pub_key_types": [
               "ed25519"
             ]
           }
         },
         "validators": [
           {
             "address": "24B62525552021A3E1970D933B4DB3E8B7927B8E",
             "pub_key": {
               "type": "tendermint/PubKeyEd25519",
               "value": "HZVyxtbiSAMTWFweTBVEUHh23bzJjr68iFUW5+P5MQc="
             },
             "power": "10",
             "name": "miner1"
           },
           {
             "address": "211D1922E2E5DCB6EEC60D69AA96F06BFCCFC85C",
             "pub_key": {
               "type": "tendermint/PubKeyEd25519",
               "value": "0KOkL5fhisXw4IUy8zv+s+FjMbk8gDnkWbCMgbbhL98="
             },
             "power": "10",
             "name": "miner2"
           },
           {
             "address": "EEE718BF22A3274753822E6A159258D9460A8FA1",
             "pub_key": {
               "type": "tendermint/PubKeyEd25519",
               "value": "0u+bXcCPBO+eTCWiildX5c4HM7cYJ9SbkU5ylzxPMDg="
             },
             "power": "10",
             "name": "miner3"
           },
           {
             "address": "12D60983CA24ACB37F14693671A2A81FD34FF7F2",
             "pub_key": {
               "type": "tendermint/PubKeyEd25519",
               "value": "wI/kn3XyPEQiiIVOjH9Ll3vUZyZK0zBY3Kho5qlx/nA="
             },
             "power": "10",
             "name": "miner4"
           }
         ],
         "app_hash": "",
         "app_state": {
           "validators": [
             {
               "address": "24B62525552021A3E1970D933B4DB3E8B7927B8E",
               "pub_key": {
                 "type": "tendermint/PubKeyEd25519",
                 "value": "HZVyxtbiSAMTWFweTBVEUHh23bzJjr68iFUW5+P5MQc="
               },
               "power": "10",
               "name": "miner1"
             },
             {
               "address": "211D1922E2E5DCB6EEC60D69AA96F06BFCCFC85C",
               "pub_key": {
                 "type": "tendermint/PubKeyEd25519",
                 "value": "0KOkL5fhisXw4IUy8zv+s+FjMbk8gDnkWbCMgbbhL98="
               },
               "power": "10",
               "name": "miner2"
             },
             {
               "address": "EEE718BF22A3274753822E6A159258D9460A8FA1",
               "pub_key": {
                 "type": "tendermint/PubKeyEd25519",
                 "value": "0u+bXcCPBO+eTCWiildX5c4HM7cYJ9SbkU5ylzxPMDg="
               },
               "power": "10",
               "name": "miner3"
             },
             {
               "address": "12D60983CA24ACB37F14693671A2A81FD34FF7F2",
               "pub_key": {
                 "type": "tendermint/PubKeyEd25519",
                 "value": "wI/kn3XyPEQiiIVOjH9Ll3vUZyZK0zBY3Kho5qlx/nA="
               },
               "power": "10",
               "name": "miner4"
             }
           ],
           "oracles": [
             "0xe0c941dd44ff4c43fc4683088b846ddb3234d169",
             "0x2c5066b71521dd5f2875cc2af226c2365b0dc7e8"
           ],
           "accounts": [
             {
               "address": "0xe0c941dd44ff4c43fc4683088b846ddb3234d169",
               "balance": 10000
             },
             {
               "address": "0x2c5066b71521dd5f2875cc2af226c2365b0dc7e8",
               "balance": 10000
             }
           ],
           "treasurer": "0x83832aa14c2d6a7fce927573b7a5607224f1e541",
           "tx_cost": {
             "Tx_SetProcessStatus": 1,
             "Tx_SetProcessCensus": 1,
             "Tx_SetProcessResults": 1,
             "Tx_SetProcessQuestionIndex": 1,
             "Tx_RegisterKey": 1,
             "Tx_NewProcess": 10,
             "Tx_SendTokens": 1,
             "Tx_CreateAccount": 5,
             "Tx_SetAccountInfoURI": 5,
             "Tx_AddDelegateForAccount": 5,
             "Tx_DelDelegateForAccount": 5,
             "Tx_CollectFaucet": 0
           }
         }
      }`,
	},
	// Development network
	"dev": {
		AutoUpdateGenesis: true,
		SeedNodes: []string{
			"7440a5b086e16620ce7b13198479016aa2b07988@seed.dev.vocdoni.net:26656"},
		CircuitsConfig: []artifacts.CircuitConfig{
			{ // index: 0, size: 1024
				URI: "https://raw.githubusercontent.com/vocdoni/" +
					"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
				CircuitPath: "zkcensusproof/dev/1024",
				Parameters:  []int64{1024},
				LocalDir:    "zkCircuits",
				ZKeyHash:    hexToBytes("0x1cd0c9225210700d4d6307493bbe5f98554e29339daba6d9bd08a4e0e78df443"),
				WitnessHash: hexToBytes("0x61b40e11ece8de3fbfaf27dbd984e0e0b1fa05ee72d4faa0c2be06c1d7a9b845"),
				VKHash:      hexToBytes("0xaed892ff98ab37b877cfcb678cb5f48f1be9d09dbbaf74b5877f46b54d10f9ad"),
			},
			{ // index: 1, size: 65k
				URI: "https://raw.githubusercontent.com/vocdoni/" +
					"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
				CircuitPath: "zkcensusproof/dev/65536",
				Parameters:  []int64{65536},
				LocalDir:    "zkCircuits",
				ZKeyHash:    hexToBytes("0xb7fb6f74ecf56e41de103e679c76c45a1bde99e2203b2ab6928396020f4d4ab6"),
				WitnessHash: hexToBytes("0x1d975d68220d1f10bd54e2f53ea9526ce8f916efb15a2079edc3db9403a78278"),
				VKHash:      hexToBytes("0x50029154e81a2078eff83751454bb3ece2cf9391103cc17306d47f7d4461b0b6"),
			},
		},
		Genesis: `
{
   "genesis_time":"2022-12-20T10:33:52.180295926Z",
   "chain_id":"vocdoni-development-73",
   "consensus_params":{
      "block":{
         "max_bytes":"5120000",
         "max_gas":"-1",
         "time_iota_ms":"8000"
      },
      "evidence":{
         "max_age_num_blocks":"100000",
         "max_age_duration":"10000"
      },
      "validator":{
         "pub_key_types":[
            "ed25519"
         ]
      }
   },
   "validators":[
      {
         "address":"5C69093136E0CB84E5CFA8E958DADB33C0D0CCCF",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"mXc5xXTKgDSYcy1lBCT1Ag7Lh1nPWHMa/p80XZPzAPY="
         },
         "power":"10",
         "name":"miner0"
      },
      {
         "address":"2E1B244B84E223747126EF621C022D5CEFC56F69",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"gaf2ZfdxpoielRXDXyBcMxkdzywcE10WsvLMe1K62UY="
         },
         "power":"10",
         "name":"miner1"
      },
      {
         "address":"4EF00A8C18BD472167E67F28694F31451A195581",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"dZXMBiQl4s0/YplfX9iMnCWonJp2gjrFHHXaIwqqtmc="
         },
         "power":"10",
         "name":"miner2"
      },
      {
         "address":"ECCC09A0DF8F4E5554A9C58F634E9D6AFD5F1598",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"BebelLYe4GZKwy9IuXCyBTySxQCNRrRoi1DSvAf6QxE="
         },
         "power":"10",
         "name":"miner3"
      }
   ],
   "app_hash":"",
   "app_state":{
      "validators":[
         {
            "address":"5C69093136E0CB84E5CFA8E958DADB33C0D0CCCF",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"mXc5xXTKgDSYcy1lBCT1Ag7Lh1nPWHMa/p80XZPzAPY="
            },
            "power":"10",
            "name":"miner0"
         },
         {
            "address":"2E1B244B84E223747126EF621C022D5CEFC56F69",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"gaf2ZfdxpoielRXDXyBcMxkdzywcE10WsvLMe1K62UY="
            },
            "power":"10",
            "name":"miner1"
         },
         {
            "address":"4EF00A8C18BD472167E67F28694F31451A195581",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"dZXMBiQl4s0/YplfX9iMnCWonJp2gjrFHHXaIwqqtmc="
            },
            "power":"10",
            "name":"miner2"
         },
         {
            "address":"ECCC09A0DF8F4E5554A9C58F634E9D6AFD5F1598",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"BebelLYe4GZKwy9IuXCyBTySxQCNRrRoi1DSvAf6QxE="
            },
            "power":"10",
            "name":"miner3"
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
		CircuitsConfig: []artifacts.CircuitConfig{
			{ // index: 0, size: 8
				URI: "https://raw.githubusercontent.com/vocdoni/" +
					"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
				CircuitPath: "zkcensusproof/dev/8",
				Parameters:  []int64{8},
				LocalDir:    "./circuits",
				ZKeyHash:    hexToBytes("0x7d09a880e14e08143bb66a6f28e13b2707afabc79693ce1613f68083fd70ac31"),
				WitnessHash: hexToBytes("0xd021d42d79d4e10b94a5785e1c1854ddb80d087cb58ad6bb29aa8b8d541d420d"),
				VKHash:      hexToBytes("0xf4876aa550e33de1d1f552dc38fa89f6e87e553fd05179e693f82f661cd0c6a0"),
			},
			{ // index: 1, size: 16
				URI: "https://raw.githubusercontent.com/vocdoni/" +
					"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
				CircuitPath: "zkcensusproof/dev/16",
				Parameters:  []int64{16},
				LocalDir:    "./circuits",
				ZKeyHash:    hexToBytes("0x2d84f9ce275f35e10b6debde1c53d74264ebb401077aaaa2ac4970e3a06eba03"),
				WitnessHash: hexToBytes("0xe55effa3d7ee28037a073f1096012ae75d97d949320bb0dfbc26ca553b818c70"),
				VKHash:      hexToBytes("0x0d8af5c3cc443cfbaed59b6144b1edb959daacbae085a97f74cbafbe109de2fa"),
			},
			{ // index: 2, size: 1024
				URI: "https://raw.githubusercontent.com/vocdoni/" +
					"zk-circuits-artifacts/6afb7c22d856c8b727262b0a0ae8ab7ca534dd4e",
				CircuitPath: "zkcensusproof/dev/1024",
				Parameters:  []int64{1024},
				LocalDir:    "./circuits",
				ZKeyHash:    hexToBytes("0x1cd0c9225210700d4d6307493bbe5f98554e29339daba6d9bd08a4e0e78df443"),
				WitnessHash: hexToBytes("0x61b40e11ece8de3fbfaf27dbd984e0e0b1fa05ee72d4faa0c2be06c1d7a9b845"),
				VKHash:      hexToBytes("0xaed892ff98ab37b877cfcb678cb5f48f1be9d09dbbaf74b5877f46b54d10f9ad"),
			},
		},
		Genesis: `
{
   "genesis_time":"2022-12-14T14:00:01.055210151Z",
   "chain_id":"vocdoni-stage-24",
   "consensus_params":{
      "block":{
         "max_bytes":"2048000",
         "max_gas":"-1"
      },
      "evidence":{
         "max_age_num_blocks":"100000",
         "max_age_duration":"10000"
      },
      "validator":{
         "pub_key_types":[
            "ed25519"
         ]
      }
   },
   "validators":[
      {
         "address":"B04F5541E932BB754B566969A3CD1F8E4193EFE8",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"JgZaEoVxLyv7jcMDikidK2HEbChqljSrZwN+humPh34="
         },
         "power":"10",
         "name":"miner1"
      },
      {
         "address":"2DECD25EBDD6E3FAB2F06AC0EE391C16C292DBAD",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"HfZWmadJhz647Gx8zpRsSz8FACcWVpU2z6jYwUwComA="
         },
         "power":"10",
         "name":"miner2"
      },
      {
         "address":"3C6FF3D424901733818B954AA3AB3BC2E3695332",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"sAsec4Da5SZrRAOmeIWDKKbwieDF5EwT28bjxtbPlpk="
         },
         "power":"10",
         "name":"miner3"
      },
      {
         "address":"92C9A63172DFB4E9637309DFBFE20B1D11EDC4E7",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"SeRU76Jq8DRKrjtSpPLR/W69khkbeQBeNLr8CMXiht8="
         },
         "power":"10",
         "name":"miner4"
      }
   ],
   "app_hash":"",
   "app_state":{
      "validators":[
         {
            "address":"B04F5541E932BB754B566969A3CD1F8E4193EFE8",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"JgZaEoVxLyv7jcMDikidK2HEbChqljSrZwN+humPh34="
            },
            "power":"10",
            "name":"miner1"
         },
         {
            "address":"2DECD25EBDD6E3FAB2F06AC0EE391C16C292DBAD",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"HfZWmadJhz647Gx8zpRsSz8FACcWVpU2z6jYwUwComA="
            },
            "power":"10",
            "name":"miner2"
         },
         {
            "address":"3C6FF3D424901733818B954AA3AB3BC2E3695332",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"sAsec4Da5SZrRAOmeIWDKKbwieDF5EwT28bjxtbPlpk="
            },
            "power":"10",
            "name":"miner3"
         },
         {
            "address":"92C9A63172DFB4E9637309DFBFE20B1D11EDC4E7",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"SeRU76Jq8DRKrjtSpPLR/W69khkbeQBeNLr8CMXiht8="
            },
            "power":"10",
            "name":"miner4"
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

// hexToBytes parses a hex string and returns the byte array from it. Warning,
// in case of error it will panic.
func hexToBytes(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		log.Fatalf("Error decoding hex string %s: %s", s, err)
	}
	return b
}
