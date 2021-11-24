package vochain

import (
	"encoding/hex"
	"strings"

	"go.vocdoni.io/dvote/crypto/zk/artifacts"
	"go.vocdoni.io/dvote/log"
)

type VochainGenesis struct {
	AutoUpdateGenesis bool
	SeedNodes         []string
	CircuitsConfig    []artifacts.CircuitConfig
	Genesis           string
}

// Genesis is a map containing the defaut Genesis details
var Genesis = map[string]VochainGenesis{

	// Production Network
	"main": {
		AutoUpdateGenesis: false,
		SeedNodes:         []string{"121e65eb5994874d9c05cd8d584a54669d23f294@seed.vocdoni.net:26656"},
		Genesis: `
   {
      "genesis_time":"2021-05-12T12:38:33.672114557Z",
      "chain_id":"vocdoni-release-1.0.1",
      "consensus_params":{
         "block":{
            "max_bytes":"10485760",
            "max_gas":"-1",
            "time_iota_ms":"10000"
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
            "address":"6DB4FEE1D370907B31196B493714FC0F45C62DED",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"R7U+HxyTrvlXccEm1sc80ww83Fpp4xg247nmpjmkYTc="
            },
            "power":"10",
            "name":"miner1"
         },
         {
            "address":"71AA2FEFA96447BC5AEF9FD928F3F8ED57E695CF",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"ixI91P+MP1jiVIy1JwQqwRdZIZxsVI0WrytAzohMGCk="
            },
            "power":"10",
            "name":"miner2"
         },
         {
            "address":"AA9CC01B46BDD1AC9E2197BB9B84993CCDF880B2",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"H6oEMFrNFeQemr9Kgxjq/wVk1kZQ1VE/J1wVnVJ+K9I="
            },
            "power":"10",
            "name":"miner3"
         },
         {
            "address":"314D17BBE991FBD3D234E5C62CFD5D0717123C95",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"FLEg/pgdF4dZ060mved/z99p/EJePu9kSsyLnrsRNC0="
            },
            "power":"10",
            "name":"miner4"
         },
         {
            "address":"34B048A4A720E6B3918CF8B75CF12555080465E5",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"aF/+WaNs5tknRMRpTPO49TJZLmDctO+JH8uckE5fTNU="
            },
            "power":"10",
            "name":"miner5"
         }
      ],
      "app_hash":"",
      "app_state":{
         "validators":[
            {
               "address":"6DB4FEE1D370907B31196B493714FC0F45C62DED",
               "pub_key":{
                  "type":"tendermint/PubKeyEd25519",
                  "value":"R7U+HxyTrvlXccEm1sc80ww83Fpp4xg247nmpjmkYTc="
               },
               "power":"10",
               "name":"miner1"
            },
            {
               "address":"71AA2FEFA96447BC5AEF9FD928F3F8ED57E695CF",
               "pub_key":{
                  "type":"tendermint/PubKeyEd25519",
                  "value":"ixI91P+MP1jiVIy1JwQqwRdZIZxsVI0WrytAzohMGCk="
               },
               "power":"10",
               "name":"miner2"
            },
            {
               "address":"AA9CC01B46BDD1AC9E2197BB9B84993CCDF880B2",
               "pub_key":{
                  "type":"tendermint/PubKeyEd25519",
                  "value":"H6oEMFrNFeQemr9Kgxjq/wVk1kZQ1VE/J1wVnVJ+K9I="
               },
               "power":"10",
               "name":"miner3"
            },
            {
               "address":"314D17BBE991FBD3D234E5C62CFD5D0717123C95",
               "pub_key":{
                  "type":"tendermint/PubKeyEd25519",
                  "value":"FLEg/pgdF4dZ060mved/z99p/EJePu9kSsyLnrsRNC0="
               },
               "power":"10",
               "name":"miner4"
            },
            {
               "address":"34B048A4A720E6B3918CF8B75CF12555080465E5",
               "pub_key":{
                  "type":"tendermint/PubKeyEd25519",
                  "value":"aF/+WaNs5tknRMRpTPO49TJZLmDctO+JH8uckE5fTNU="
               },
               "power":"10",
               "name":"miner5"
            }
         ],
         "oracles":[
            "0xc2e396d6e6ae9b12551f0c6111f9766bec926bfe",
            "0x1a361c26e04a33effbf3bd8617b1e3e0aa6b704f"
         ]
      }
   }
 `,
	},

	// Development network
	"dev": {
		AutoUpdateGenesis: true,
		SeedNodes: []string{
			"7440a5b086e16620ce7b13198479016aa2b07988@seed.dev.vocdoni.net:26656"},
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
   "genesis_time":"2021-11-10T09:43:28.668436552Z",
   "chain_id":"vocdoni-development-57",
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
         "0x2f4ed2773dcf7ad0ec15eb84ec896f4eebe0e08a"
      ]
   }
}
`,
	},

	"stage": {
		AutoUpdateGenesis: true,
		SeedNodes: []string{
			"588133b8309363a2a852e853424251cd6e8c5330@seed.stg.vocdoni.net:26656"},
		Genesis: `
{
   "genesis_time":"2021-05-24T14:41:19.055210151Z",
   "chain_id":"vocdoni-stage-9",
   "consensus_params":{
      "block":{
         "max_bytes":"22020096",
         "max_gas":"-1",
         "time_iota_ms":"10000"
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
         "0xcf83836eab1a4697bb9f9d07d7fb82aed707d918",
         "0x949a4b6b5dc64cdc2518c15c8dfdead4ebd07df0"
      ]
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
