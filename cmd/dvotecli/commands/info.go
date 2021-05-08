package commands

import (
	"bytes"
	"fmt"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/client"
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

	req := api.MetaRequest{Method: "getInfo"}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("Health:\t%v\n", resp.Health)
	b := bytes.Buffer{}
	b.WriteString("APIs:\t")
	for _, api := range resp.APIList {
		b.WriteString(api + " ")
	}
	fmt.Println(b.String())
	return err
}
