package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

var txCostCmd = &cobra.Command{
	Use:   "txcost",
	Short: "Manage transaction costs for each type of transaction. Only the Treasurer may do this.",
}

var txCostGetCmd = &cobra.Command{
	Use:   "get <keystore> [txtypes as string]",
	Short: `Get a single or all transaction costs.`,
	Long: `Get a single or all transaction costs.
	Examples of valid txtypes: AddDelegateForAccount, CollectFaucet, DelDelegateForAccount,
	NewProcess, RegisterKey, SendTokens, SetAccountInfo, SetProcessCensus, SetProcessQuestionIndex,
	SetProcessResults, SetProcessStatus`,
	Args: cobra.MatchAll(
		cobra.MinimumNArgs(1),
		func(cmd *cobra.Command, args []string) error {
			for _, t := range args[1:] {
				a := vochain.TxCostNameToTxType(t)
				if a == models.TxType_TX_UNKNOWN {
					return fmt.Errorf("%s is not a valid TxType; valid TxTypes are %v", t, txTypes)
				}
			}
			return nil
		},
	),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		var txTypesToQuery []string
		if len(args[1:]) == 0 {
			txTypesToQuery = txTypes
		} else {
			txTypesToQuery = args[1:]
		}

		for _, t := range txTypesToQuery {
			a := vochain.TxCostNameToTxType(t)
			cost, err := c.GetTransactionCost(a)
			if err != nil && strings.Contains(err.Error(), "TX_UNKNOWN") {
				return fmt.Errorf("the node does not recognize this txtype: \"%s\"", err)
			} else if err != nil {
				return err
			}
			fmt.Fprintln(Stdout, t, cost)
		}
		return nil
	},
}

var txCostSetCmd = &cobra.Command{
	Use:   "set <keystore> <txtype> <cost>",
	Short: `Set a cost for a transaction type.`,
	Long: `Set a cost for a transaction type.
	Examples of valid txtypes: AddDelegateForAccount, CollectFaucet, DelDelegateForAccount,
	NewProcess, RegisterKey, SendTokens, SetAccountInfo, SetProcessCensus, SetProcessQuestionIndex,
	SetProcessResults, SetProcessStatus`,
	Args: cobra.MatchAll(
		cobra.ExactArgs(3),
		func(cmd *cobra.Command, args []string) error {
			a := vochain.TxCostNameToTxType(args[1])
			if a == models.TxType_TX_UNKNOWN {
				return fmt.Errorf("%s is not a valid TxType; valid TxTypes are %v", args[1], txTypes)
			}
			return nil
		},
	),
	RunE: func(cmd *cobra.Command, args []string) error {
		cost, err := strconv.ParseUint(args[2], 10, 64)
		if err != nil {
			return err
		}
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		_, signer, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return fmt.Errorf("could not open keyfile %s", err)
		}

		nonce, err := getTreasurerNonce(c)
		if err != nil {
			return err
		}

		a := vochain.TxCostNameToTxType(args[1])
		err = c.SetTransactionCost(signer, a, cost, nonce)
		if err != nil && strings.Contains(err.Error(), "TX_UNKNOWN") {
			return fmt.Errorf("the node does not recognize this txtype: \"%s\"", err)
		} else if err != nil {
			return err
		}
		return nil
	},
}
