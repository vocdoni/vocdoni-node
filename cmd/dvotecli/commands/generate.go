package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/types"
)

const minersPower = 10

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate keys and genesis for vochain",
	RunE:  generate,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().Int("miners", 4, "number of miner keys")
	generateCmd.Flags().Int("oracles", 2, "number of oracle keys")
	generateCmd.Flags().String("chainId", "", "an ID name for the genesis chain to generate (required)")
	generateCmd.MarkFlagRequired("chainId")
}

func generate(cmd *cobra.Command, args []string) error {

	// Generate miners
	mCount, _ := cmd.Flags().GetInt("miners")

	minerPVs := make([]*privval.FilePV, mCount)
	for i := range minerPVs {
		minerPVs[i] = privval.GenFilePV("", "")
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
	consensusParams := &types.ConsensusParams{
		Block: types.BlockParams(tmConsensusParams.Block),
		//Evidence:  types.EvidenceParams(tmConsensusParams.Evidence),
		Validator: types.ValidatorParams(tmConsensusParams.Validator),
	}
	chainID, _ := cmd.Flags().GetString("chainId")
	appState := new(types.GenesisAppState)

	appState.Oracles = oracles
	cdc := amino.NewCodec()

	appState.Validators = make([]types.GenesisValidator, mCount)
	for idx, val := range minerPVs {
		pubk, err := val.GetPubKey()
		if err != nil {
			return err
		}

		appState.Validators[idx] = types.GenesisValidator{
			Address: pubk.Address(),
			//PubKey:  pubk,
			Power: fmt.Sprintf("%d", minersPower),
			Name:  "miner-" + fmt.Sprint(idx+1),
		}
	}

	appStateBytes, err := cdc.MarshalJSON(appState)
	if err != nil {
		return err
	}
	genDoc := types.GenesisDoc{
		ChainID:         chainID,
		GenesisTime:     tmtime.Now(),
		ConsensusParams: consensusParams,
		Validators:      appState.Validators,
		AppState:        appStateBytes,
	}
	data, err := cdc.MarshalJSONIndent(genDoc, "", "  ")
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
