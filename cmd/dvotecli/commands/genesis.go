package commands

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/vochain"
)

type keypair struct {
	Address string `json:"address"`
	PrivKey string `json:"priv_key"`
}

type keyring struct {
	Seeds   []keypair `json:"seeds"`
	Miners  []keypair `json:"miners"`
	Oracles []keypair `json:"oracles"`
	// Treasurer is unique
	Treasurer keypair `json:"treasurer"`
}

var genesisGenCmd = &cobra.Command{
	Use:   "genesis",
	Short: "Generate keys and genesis for vochain",
	RunE:  genesisGen,
}

func init() {
	rootCmd.AddCommand(genesisGenCmd)
	genesisGenCmd.Flags().Int("seeds", 1, "number of seed keys")
	genesisGenCmd.Flags().Int("miners", 4, "number of miner keys")
	genesisGenCmd.Flags().Int("oracles", 2, "number of oracle keys")
	genesisGenCmd.Flags().String("treasurer", "", "address of the treasurer")
	genesisGenCmd.Flags().String("chainId", "",
		"an ID name for the genesis chain to generate (required)")
	genesisGenCmd.Flags().BoolP("json", "j", false, "output a JSON document")
	genesisGenCmd.Flags().StringP("file", "w", "", "write genesis to <file> instead of stdout")
	cobra.CheckErr(genesisGenCmd.MarkFlagRequired("chainId"))
}

func genesisGen(cmd *cobra.Command, args []string) error {
	var keys keyring

	// Generate seeds
	sCount, _ := cmd.Flags().GetInt("seeds")

	seedPKs := make([]ed25519.PrivKey, sCount)
	for i := range seedPKs {
		pk := ed25519.GenPrivKey()
		seedPKs[i] = pk
		keys.Seeds = append(keys.Seeds, keypair{
			Address: hex.EncodeToString(seedPKs[i].PubKey().Address()),
			PrivKey: hex.EncodeToString(seedPKs[i]),
		})
	}

	// Generate miners
	mCount, _ := cmd.Flags().GetInt("miners")

	minerPVs := make([]privval.FilePV, mCount)
	for i := range minerPVs {
		pv, err := privval.GenFilePV("", "", tmtypes.ABCIPubKeyTypeEd25519)
		if err != nil {
			return err
		}
		minerPVs[i] = *pv
		keys.Miners = append(keys.Miners, keypair{
			Address: fmt.Sprintf("%s", minerPVs[i].Key.Address),
			PrivKey: fmt.Sprintf("%x", minerPVs[i].Key.PrivKey),
		})
	}

	// Generate oracles
	oCount, _ := cmd.Flags().GetInt("oracles")
	oKeys := make([]*ethereum.SignKeys, oCount)
	oracles := make([]string, oCount)
	for i := range oKeys {
		oKeys[i] = ethereum.NewSignKeys()
		if err := oKeys[i].Generate(); err != nil {
			return err
		}

		oracles[i] = oKeys[i].AddressString()
		_, priv := oKeys[i].HexString()
		keys.Oracles = append(keys.Oracles, keypair{
			Address: oKeys[i].AddressString(),
			PrivKey: priv,
		})
	}

	// Generate genesis
	tmConsensusParams := tmtypes.DefaultConsensusParams()
	consensusParams := &vochain.ConsensusParams{
		Block:     vochain.BlockParams(tmConsensusParams.Block),
		Validator: vochain.ValidatorParams(tmConsensusParams.Validator),
	}

	// Get or generate treasurer
	treasurer, _ := cmd.Flags().GetString("treasurer")
	t := &ethereum.SignKeys{}
	if treasurer == "" {
		// generate new treasurer
		t = ethereum.NewSignKeys()
		if err := t.Generate(); err != nil {
			return err
		}
	} else {
		if err := t.AddHexKey(treasurer); err != nil {
			return err
		}
	}
	_, priv := t.HexString()
	keys.Treasurer = keypair{
		Address: t.Address().String(),
		PrivKey: priv,
	}

	// Get chainID
	chainID, _ := cmd.Flags().GetString("chainId")

	genesisBytes, err := vochain.NewGenesis(nil, chainID, consensusParams, minerPVs, oracles, t.Address().String())
	if err != nil {
		return err
	}
	data := new(bytes.Buffer)
	err = json.Indent(data, genesisBytes, "", "  ")
	if err != nil {
		return err
	}

	printJson, _ := cmd.Flags().GetBool("json")

	if printJson {
		bytes, err := json.MarshalIndent(keys, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", bytes)
	} else {
		prettyPrintKeyring(keys)
	}

	fmt.Println()

	if file, _ := cmd.Flags().GetString("file"); file != "" {
		if err := os.WriteFile(file, data.Bytes(), 0o600); err != nil {
			return err
		}
	} else {
		if !printJson {
			prettyHeader("Genesis JSON")
		}
		fmt.Printf("%s\n ", data)
	}

	return nil
}

func prettyHeader(text string) {
	fmt.Println(au.Red(">>>"), au.Blue(text))
}

func prettyPrintKeyring(keys keyring) {
	prettyPrint(keys.Seeds, "Seed")
	prettyPrint(keys.Miners, "Miner")
	prettyPrint(keys.Oracles, "Oracle")
	prettyPrint([]keypair{keys.Treasurer}, "Treasurer")
}

func prettyPrint(k []keypair, label string) {
	for i, item := range k {
		if i == 0 {
			fmt.Println()
		}
		prettyHeader(fmt.Sprintf("%s #%d", label, i))
		fmt.Printf("Address: %s\n", au.Yellow(item.Address))
		fmt.Printf("Private Key: %s\n", au.Yellow(item.PrivKey))
	}
}
