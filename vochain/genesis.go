package vochain

// Testnet Genesis File for Vocdoni KISS v1
const (
	TestnetGenesis1 = `
{
  "genesis_time": "2019-09-13T08:08:50.199392102Z",
  "chain_id": "0x5",
  "consensus_params": {
    "block": {
      "max_bytes": "22020096",
      "max_gas": "-1",
      "time_iota_ms": "10000"
    },
    "evidence": {
      "max_age": "100000"
    },
    "validator": {
      "pub_key_types": [
        "ed25519"
      ]
    }
  },
  "validators": [
    {
      "address": "243A633E60AAFB177018D76C5AA0A3DF0ACC13D1",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "MlOJMC1nwAYDmaju+2VJijoIO6cBF36Ygmsdc4gKZtk="
      },
      "power": "10",
      "name": ""
    },
    {
      "address": "5DC922017285EC24415F3E7ECD045665EADA8B5A",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "4MlhCW62N/5bj5tD66//h9RnsAh+xjAdMU8lGiEwvyM="
      },
      "power": "10",
      "name": ""
    },
    {
      "address": "77EA441EA0EB29F049FC57DE524C55833A7FF575",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "GyZfKNK3lT5AQXQ4pwrVdgG3rRisx9tS4bM9EIZ0zYY="
      },
      "power": "10",
      "name": ""
    },
    {
      "address": "D8C253A41C7D8EE0E2AD04B2A1B6AED37FAE18E7",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "zNYNrEVl0tGegjLgq8ZQOHUC+glzpHnmOs9x+9n9UgQ="
      },
      "power": "10",
      "name": ""
    }
  ],
  "app_hash": "",
  "app_state": {
    "validators": [
      {
        "address": "243A633E60AAFB177018D76C5AA0A3DF0ACC13D1",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "MlOJMC1nwAYDmaju+2VJijoIO6cBF36Ygmsdc4gKZtk="
        },
        "power": "10",
        "name": ""
      },
      {
        "address": "5DC922017285EC24415F3E7ECD045665EADA8B5A",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "4MlhCW62N/5bj5tD66//h9RnsAh+xjAdMU8lGiEwvyM="
        },
        "power": "10",
        "name": ""
      },
      {
        "address": "77EA441EA0EB29F049FC57DE524C55833A7FF575",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "GyZfKNK3lT5AQXQ4pwrVdgG3rRisx9tS4bM9EIZ0zYY="
        },
        "power": "10",
        "name": ""
      },
      {
        "address": "D8C253A41C7D8EE0E2AD04B2A1B6AED37FAE18E7",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "zNYNrEVl0tGegjLgq8ZQOHUC+glzpHnmOs9x+9n9UgQ="
        },
        "power": "10",
        "name": ""
      }
        ],
    "oracles": [
      "0xF904848ea36c46817096E94f932A9901E377C8a5"
    ]
  }
}
`

	TestnetGenesis2 = `
{
  "genesis_time": "2019-10-15T15:45:55.298705612Z",
  "chain_id": "0x2",
  "consensus_params": {
    "block": {
      "max_bytes": "22020096",
      "max_gas": "-1",
      "time_iota_ms": "20000"
    },
    "evidence": {
      "max_age": "100000"
    },
    "validator": {
      "pub_key_types": [
        "ed25519"
      ]
    }
  },
  "validators": [
    {
      "address": "8A84E3572812E4D76377322AA9C242859A39133F",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "GyCU9rGOtlIjqnCyj0tNxkYUdVlkE6XcM98xzpajc2g="
      },
      "power": "10",
      "name": ""
    }
  ],
  "app_hash": "",
  "app_state": {
    "validators": [
      {
        "address": "8A84E3572812E4D76377322AA9C242859A39133F",
        "pub_key": {
          "type": "tendermint/PubKeyEd25519",
          "value": "GyCU9rGOtlIjqnCyj0tNxkYUdVlkE6XcM98xzpajc2g="
        },
        "power": "10",
        "name": ""
      }
    ],
    "oracles": [
      "0xF904848ea36c46817096E94f932A9901E377C8a5"
    ]
  }
}
`

	TestGenesis2NodeKey = `
{
  "priv_key":{
    "type":"tendermint/PrivKeyEd25519",
    "value":"4aBFMyszl4MflGS/NY4yxi8/mU7mJUHav4rc8kejwKhCqpW0kVHimV9l/Vu0koAgI0a1z9ojhQRD2UQyKKEuDQ=="
  }
}
`

	TestGenesis2PrivValKey = `
{
  "address": "8A84E3572812E4D76377322AA9C242859A39133F",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "GyCU9rGOtlIjqnCyj0tNxkYUdVlkE6XcM98xzpajc2g="
  },
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "sUd///0ux6mjdzqHBPgwZetB4KpG2VEqyU18aL69wYYbIJT2sY62UiOqcLKPS03GRhR1WWQTpdwz3zHOlqNzaA=="
  }
}
`

	TestGenesisPrivValState = `
{
  "height": "0",
  "round": "0",
  "step": 0,
  "signature": "",
  "signbytes": ""
}
`
)
