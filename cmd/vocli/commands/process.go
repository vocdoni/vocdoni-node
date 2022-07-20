package commands

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/proto/build/go/models"
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Create/update/end a process",
}

var setProcessCmd = &cobra.Command{
	Use:   "set <keystore> <process id> <process status>",
	Short: "Set a Process's status, where status is one of PROCESS_UNKNOWN,READY,ENDED,CANCELED,PAUSED,RESULTS",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return fmt.Errorf("requires <keystore> <process id> <process status>")
		}
		if _, exists := models.ProcessStatus_value[args[2]]; !exists {
			return fmt.Errorf("invalid process status specified")
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
		pid, err := hex.DecodeString(args[1])
		if err != nil {
			return fmt.Errorf("could not decode hexstring %s into bytes", args[1])
		}
		return c.SetProcessStatus(key, pid, args[2])
	},
}
