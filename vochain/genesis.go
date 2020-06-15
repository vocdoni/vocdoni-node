package vochain

// Testnet Genesis File for Vocdoni KISS v1
const (
	ReleaseGenesis1 = `
{
  "genesis_time": "2020-06-15T09:30:50.199392102Z",
  "chain_id": "vocdoni-release-04",
  "consensus_params": {
    "block": {
      "max_bytes": "22020096",
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
      "address": "85CD8C680EC27712EC83FEA63C16BE97115EBA97",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "BpIfgz39GAuFSPSExT/TdJxm/UhoF1L1YxOY+pcbJzc="
      },
      "power": "10",
      "name": "miner4"
    },
    {
      "address": "5DC922017285EC24415F3E7ECD045665EADA8B5A",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "4MlhCW62N/5bj5tD66//h9RnsAh+xjAdMU8lGiEwvyM="
      },
      "power": "10",
      "name": "miner1"
    },
    {
      "address": "77EA441EA0EB29F049FC57DE524C55833A7FF575",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "GyZfKNK3lT5AQXQ4pwrVdgG3rRisx9tS4bM9EIZ0zYY="
      },
      "power": "10",
      "name": "miner2"
    },
    {
      "address": "D8C253A41C7D8EE0E2AD04B2A1B6AED37FAE18E7",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "zNYNrEVl0tGegjLgq8ZQOHUC+glzpHnmOs9x+9n9UgQ="
      },
      "power": "10",
      "name": "miner3"
    }
  ],
  "app_hash": "",
  "app_state": {
    "validators": [
      {
        "address": "85CD8C680EC27712EC83FEA63C16BE97115EBA97",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "BpIfgz39GAuFSPSExT/TdJxm/UhoF1L1YxOY+pcbJzc="
        },
        "power": "10",
        "name": "miner4"
      },
      {
        "address": "5DC922017285EC24415F3E7ECD045665EADA8B5A",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "4MlhCW62N/5bj5tD66//h9RnsAh+xjAdMU8lGiEwvyM="
        },
        "power": "10",
        "name": "miner1"
      },
      {
        "address": "77EA441EA0EB29F049FC57DE524C55833A7FF575",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "GyZfKNK3lT5AQXQ4pwrVdgG3rRisx9tS4bM9EIZ0zYY="
        },
        "power": "10",
        "name": "miner2"
      },
      {
        "address": "D8C253A41C7D8EE0E2AD04B2A1B6AED37FAE18E7",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "zNYNrEVl0tGegjLgq8ZQOHUC+glzpHnmOs9x+9n9UgQ="
        },
        "power": "10",
        "name": "miner3"
      }
        ],
    "oracles": [
      "0x73ab4566b2e404e2be4fe9a8fb007f4014292c0c",
	  "0x28696305c434dd4e962677d77c719ab460c73325"
    ]
  }
}
`

	DevelopmentGenesis1 = `
{
   "genesis_time":"2020-06-01T20:29:50.512370579Z",
   "chain_id":"vocdoni-development-13",
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
      },
      {
         "address":"3272B3046C31D87F92E26D249B97CC144D835DA6",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"hUz8jCePBfG4Bi9s13IdleWq5MZ5upe03M+BX2ah7c4="
         },
         "power":"10",
         "name":"miner4"
      },
      {
         "address":"05BA8FCBEA4A4EDCFD49081B42CA3F9ED13246C1",
         "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"1FGNernnvg4QpV7psYFQPeIFZJm32yN1SjZULbliidg="
         },
         "power":"10",
         "name":"miner5"
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
         },
         {
            "address":"3272B3046C31D87F92E26D249B97CC144D835DA6",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"hUz8jCePBfG4Bi9s13IdleWq5MZ5upe03M+BX2ah7c4="
            },
            "power":"10",
            "name":"miner4"
         },
         {
            "address":"05BA8FCBEA4A4EDCFD49081B42CA3F9ED13246C1",
            "pub_key":{
               "type":"tendermint/PubKeyEd25519",
               "value":"1FGNernnvg4QpV7psYFQPeIFZJm32yN1SjZULbliidg="
            },
            "power":"10",
            "name":"miner5"
         }
      ],
      "oracles":[
         "0xb926be24A9ca606B515a835E91298C7cF0f2846f",
         "0x2f4ed2773dcf7ad0ec15eb84ec896f4eebe0e08a"
      ]
   }
}
`
)
