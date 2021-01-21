package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/types"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "get information about the gateway",
	RunE:  info,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func info(cmd *cobra.Command, args []string) error {
	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{Method: "getInfo"}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("Health: %v\n", resp.Health)
	fmt.Print("APIs: ")
	for _, api := range resp.APIList {
		fmt.Printf("%v ", api)
	}
	return err
}
