package commands

import (
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	client "go.vocdoni.io/dvote/rpcclient"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage admin operations",
}

var setOracleCmd = &cobra.Command{
	Use:   "oracle <keystore> <address> <op>",
	Short: "Adds or deletes an oracle (op=0 -> DEL, op=1 -> ADD)",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return fmt.Errorf("requires <keystore> <address> <op>")
		}
		if args[2] != "0" && args[2] != "1" {
			return fmt.Errorf("invalid operation")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}
		_, key, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return err
		}

		t, err := c.GetTreasurer()
		if err != nil {
			return fmt.Errorf("could not lookup the account's nonce, try specifying manually: %s", err)
		}

		oracleAddr := common.HexToAddress(args[1])
		op, err := strconv.ParseBool(args[2])
		if err != nil {
			return fmt.Errorf("invalid operation, cannot parse bool")
		}
		return c.SetOracle(key, oracleAddr, t.Nonce, op)
	},
}
