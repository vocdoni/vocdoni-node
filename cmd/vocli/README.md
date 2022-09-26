# Philosophy
vocli is meant for end users who want to manage their own vochain/use a vochain. Managing a vochain includes treasury-account only transactions such as minting and setting txcosts. Using a vochain includes key management, sending and claiming tokens, and account information retrieval.


# Future Developments
In the future, support will be added for creating election and submitting votes to enable an entire election with a CLI.

# Usage
```
$ vocli --help
vocli is a convenience CLI that helps you do things on Vochain

Usage:
  vocli [command]

Available Commands:
  account     Manage a key's account. Accounts are stored on the vochain and are controlled by keys.
  claimfaucet Claim tokens from another account, using a payload generated from that account that acts as an authorization.
  genfaucet   Generate a payload allowing another account to claim tokens from this account.
  help        Help about any command
  keys        Create, import and list keys.
  mint        Mint more tokens to an address. Only the Treasurer may do this.
  send        Send tokens to another account
  txcost      Manage transaction costs for each type of transaction. Only the Treasurer may do this.

Flags:
  -d, --debug             prints additional information; $VOCHAIN_DEBUG
  -h, --help              help for vocli
      --home string       root directory where all vochain files are stored; $VOCHAIN_HOME (default "/home/shinichi/.dvote")
  -n, --nonce uint32      account nonce to use when sending transaction
                                (useful when it cannot be queried ahead of time, e.g. offline transaction signing)
      --password string   supply the password as an argument instead of prompting
  -u, --url string        Gateway RPC URL; $VOCHAIN_URL (default "https://gw1.dev.vocdoni.net/dvote")

Use "vocli [command] --help" for more information about a command.
```

# Examples
## Setup
Let's setup a quick vochain fork on your own computer. Vocone is perfect for this. You may use any Ethereum key.

`$ voconed --treasurer=0x...public_address --oracle=$ORACLE_PRIVKEY --txCosts 20 --logLevel=debug'`

`export VOCHAIN_URL=http://localhost:9095/dvote`

## Creating an account
In Ethereum, the words key/account are used interchangeably.

In vochain, the Key is the private/public keypair, stored by the client to sign transactions. The Account for the corresponding Key needs to be created on the vochain before you can do anything. Other than this, there are no differences: 1 Key can only control 1 Account.

Keys are stored in Ethereum JSON format.
```
$ vocli keys new
The key will be generated the key and saved, encrypted, on your disk.
		Remember to run 'vocli account set <address>' afterwards to create an account for this key on the vochain.Your new key file will be locked with a password. Please give a password:

Your new key was generated
Public address of the key:   0x8bA764A28aff06335559B351027551DEf0C8EDF7
Path of the secret key file: /home/shinichi/.dvote/keys/0x8bA764A28aff06335559B351027551DEf0C8EDF7-2022-6-21.vokey
- As usual, please BACKUP your key file and REMEMBER your password!

```

One must additionally set an information string when setting an account. Normally an `ipfs://...` info-URI goes here, but this is currently never validated and can thus contain anything.
```
./vocli account set /home/shinichi/.dvote/keys/0x8bA764A28aff06335559B351027551DEf0C8EDF7-2022-6-21.vokey randomstring
Please unlock your key:
Sending SetAccountInfo for key 0x8bA764A28aff06335559B351027551DEf0C8EDF7 on http://localhost:9095/dvote
Account created/updated on chain http://localhost:9095/dvote
```

```
$ vocli account info 0x8bA764A28aff06335559B351027551DEf0C8EDF7
infoURI:"randomstring"
```

## Mint new tokens to an account (Treasurer only)
```
$ vocli mint /home/shinichi/.dvote/keys/0xDad23752bB8F80Bb82E6A2422A762fdeAf8dAb74-2022-1-13.vokey 0x8bA764A28aff06335559B351027551DEf0C8EDF7 2000
Please unlock your key:

$ vocli account info 0x8bA764A28aff06335559B351027551DEf0C8EDF7
balance:2000  infoURI:"randomstring"
```

## Faucet
Account A, acting as the faucet, generates a payload that will then be used/signed/submitted to the vochain by Account B to claim tokens from A.

```
$ vocli account info 0x9f5894f6DA6c4D5902B178Ee2E530038d907B4De
infoURI:"accountb"

$ vocli genfaucet /home/shinichi/.dvote/keys/0x8bA764A28aff06335559B351027551DEf0C8EDF7-2022-6-21.vokey 0x9f5894f6DA6c4D5902B178Ee2E530038d907B4De 100
Please unlock your key:
0a230886aff1faad91fdfffa0112149f5894f6da6c4d5902b178ee2e530038d907b4de1864124157c202e2566a29d75d6c988602dcf491ba68c9dc59b631abd3ce3e9ec6a52db60f2448789a5fdcc0e0005dff29a3f2754ba839bd3776b047caa610acbbe42d4201

$ vocli claimfaucet /home/shinichi/.dvote/keys/0x9f5894f6DA6c4D5902B178Ee2E530038d907B4De-2022-1-24.vokey 0a230886aff1faad91fdfffa0112149f5894f6da6c4d5902b178ee2e530038d907b4de1864124157c202e2566a29d75d6c988602dcf491ba68c9dc59b631abd3ce3e9ec6a52db60f2448789a5fdcc0e0005dff29a3f2754ba839bd3776b047caa610acbbe42d4201
Please unlock your key:

$ vocli account info 0x9f5894f6DA6c4D5902B178Ee2E530038d907B4De
balance:100  nonce:1  infoURI:"accountb"
```

## Setting Transaction Costs (Treasurer only)
To discourage spam, every transaction must have a cost. However different actions may have different costs.

```
$ vocli txcost get
AddDelegateForAccount 20
CollectFaucet 20
DelDelegateForAccount 20
NewProcess 20
RegisterKey 20
SendTokens 20
SetAccountInfo 20
SetProcessCensus 20
SetProcessQuestionIndex 20
SetProcessResults 20
SetProcessStatus 20

$ vocli txcost get CollectFaucet SetProcessResults
CollectFaucet 20
SetProcessResults 20
```

```
export TREASURER=/home/shinichi/.dvote/keys/0x....vokey
$ vocli txcost set $TREASURER SetProcessStatus 10
Please unlock your key:

$ vocli txcost get SetProcessStatus
SetProcessStatus 10
```
