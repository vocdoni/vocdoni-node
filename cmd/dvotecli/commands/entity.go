package commands

import (
	"bytes"
	"fmt"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/rpcapi"
)

var (
	entityCmd = &cobra.Command{
		Use:   "entity list",
		Short: "entity subcommands",
	}
	entityListCmd = &cobra.Command{
		Use:   "list [prefix]",
		Short: "list entities matching an optional prefix",
		RunE:  entityList,
	}
)

func init() {
	rootCmd.AddCommand(entityCmd)
	entityCmd.AddCommand(entityListCmd)
}

func entityList(cmd *cobra.Command, args []string) error {
	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := api.APIrequest{Method: "getEntityList"}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}

	entities := append([]string{}, *resp.EntityIDs...)
	if len(entities) == rpcapi.MaxListSize {
		for i, count := 1, len(entities); count > 0 && i < MaxListIterations; i++ {
			req.From = i * rpcapi.MaxListSize
			resp, err := cl.Request(req, nil)
			if err != nil {
				return err
			}
			entities = append(entities, *resp.EntityIDs...)
			count = len(*resp.EntityIDs)
		}
	}

	buffer := new(bytes.Buffer)
	for _, ent := range entities {
		buffer.WriteString(ent + "\n")
	}
	fmt.Print(buffer.String())
	return err
}
