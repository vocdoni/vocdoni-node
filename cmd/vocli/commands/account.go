package commands

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	client "go.vocdoni.io/dvote/rpcclient"
)

var accInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get information about an account from the vochain.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}
		resp, err := c.GetAccount(common.HexToAddress(args[0]))
		if err != nil {
			return err
		}
		fmt.Fprintln(Stdout, resp.String())
		return err
	},
}

var accSetInfoCmd = &cobra.Command{
	Use:   "set <keystore, ipfs://info-URI>",
	Short: "Create/Update an account on the vochain",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, _, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return err
		}
		infoUri := args[1]

		if err := createAccount(key, v.GetString(urlKey), infoUri, faucetHex); err != nil {
			return err
		}
		return nil
	},
}

var accTreasurerCmd = &cobra.Command{
	Use:   "treasurer",
	Short: "Get information about the treasurer account (a special account) on the vochain.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		resp, err := c.GetTreasurer()
		if err != nil {
			return err
		}
		fmt.Fprintln(Stdout, "address", common.BytesToAddress(resp.Address))
		fmt.Fprintln(Stdout, "nonce", resp.Nonce)
		return err
	},
}

var accAddDelegateCmd = &cobra.Command{
	Use:   "add-delegate <keystore> <address>",
	Short: "Add a delegate address for an account",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		_, signer, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return err
		}

		nonce, err := getNonce(c, signer.Address().String())
		if err != nil {
			return err
		}

		return c.SetAccountDelegate(signer, common.HexToAddress(args[1]), true, nonce)
	},
}

var accDelDelegateCmd = &cobra.Command{
	Use:   "rm-delegate <keystore> <address>",
	Short: "Remove a delegate address for an account",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		_, signer, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return err
		}

		nonce, err := getNonce(c, signer.Address().String())
		if err != nil {
			return err
		}

		return c.SetAccountDelegate(signer, common.HexToAddress(args[1]), false, nonce)
	},
}
var accCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage a key's account. Accounts are stored on the vochain and are controlled by keys.",
}
