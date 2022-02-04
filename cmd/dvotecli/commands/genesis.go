package commands

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/vochain"
)

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
	cobra.CheckErr(genesisGenCmd.MarkFlagRequired("chainId"))
}

func genesisGen(cmd *cobra.Command, args []string) error {
	// Generate seeds
	sCount, _ := cmd.Flags().GetInt("seeds")

	seedPKs := make([]ed25519.PrivKey, sCount)
	for i := range seedPKs {
		pk := ed25519.GenPrivKey()
		seedPKs[i] = pk
		prettyHeader(fmt.Sprintf("Seed #%d", i+1))
		fmt.Printf("Address: %s\n", au.Yellow(hex.EncodeToString(seedPKs[i].PubKey().Address())))
		fmt.Printf("Private Key: %s\n", au.Yellow(hex.EncodeToString(seedPKs[i])))
	}
	fmt.Println()

	// Generate miners
	mCount, err := cmd.Flags().GetInt("miners")
	if err != nil {
		return err
	}

	minerPVs := make([]privval.FilePV, mCount)
	for i := range minerPVs {
		// TENDERMINT 0.35
		//pv, err := privval.GenFilePV("", "", tmtypes.ABCIPubKeyTypeEd25519)
		//if err != nil {
		//	return err
		//}
		pv := privval.GenFilePV("", "")
		minerPVs[i] = *pv
		prettyHeader(fmt.Sprintf("Miner #%d", i+1))
		fmt.Printf("Address: %s\n", au.Yellow(minerPVs[i].Key.Address))
		fmt.Printf("Private Key: %x\n", au.Yellow(minerPVs[i].Key.PrivKey))
	}
	fmt.Println()

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

		prettyHeader(fmt.Sprintf("Oracle #%d", i+1))
		_, priv := oKeys[i].HexString()
		fmt.Printf("Address: %s\n", au.Yellow(oKeys[i].AddressString()))
		fmt.Printf("Private Key: %x\n", au.Yellow(priv))
	}
	fmt.Println()

	// Generate genesis
	tmConsensusParams := tmtypes.DefaultConsensusParams()
	consensusParams := &vochain.ConsensusParams{
		Block:     vochain.BlockParams(tmConsensusParams.Block),
		Validator: vochain.ValidatorParams(tmConsensusParams.Validator),
	}

	// Get treasurer
	treasurer, err := cmd.Flags().GetString("treasurer")
	if err != nil {
		return err
	}
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
	prettyHeader("Treasurer")
	fmt.Printf("Address: %s\n", au.Yellow(t.Address().String()))
	_, priv := t.HexString()
	fmt.Printf("Private Key: %x\n", au.Yellow(priv))

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
	prettyHeader("Genesis JSON")
	fmt.Printf("%s\n ", data)

	return nil
}

func prettyHeader(text string) {
	fmt.Println(au.Red(">>>"), au.Blue(text))
}
