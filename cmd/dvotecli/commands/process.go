package commands

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/router"
	"go.vocdoni.io/dvote/util"
)

var processCmd = &cobra.Command{
	Use:   "process list|info|keys|results|weight|finalresults|liveresults",
	Short: "process subcommands",
}

var processListCmd = &cobra.Command{
	Use:   "list [entityId]",
	Short: "list processes",
	RunE:  processList,
}

var processInfoCmd = &cobra.Command{
	Use:   "info [processId]",
	Short: "get process details",
	RunE:  processInfo,
}

var processKeysCmd = &cobra.Command{
	Use:   "keys [processId]",
	Short: "list keys of processes",
	RunE:  processKeys,
}

var processResultsCmd = &cobra.Command{
	Use:   "results [processId]",
	Short: "get the results of a process",
	RunE:  getResults,
}

var processResultsWeightCmd = &cobra.Command{
	Use:   "weight [processId]",
	Short: "get the current accumulated cast votes weight",
	RunE:  getResultsWeight,
}

func init() {
	rootCmd.AddCommand(processCmd)
	processCmd.AddCommand(processListCmd)
	processCmd.AddCommand(processInfoCmd)
	processCmd.AddCommand(processKeysCmd)
	processCmd.AddCommand(processResultsCmd)
	processCmd.AddCommand(processResultsWeightCmd)
}

func processList(cmd *cobra.Command, args []string) error {
	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := api.MetaRequest{Method: "getProcessList"}
	if len(args) >= 1 {
		req.EntityId, err = hex.DecodeString(util.TrimHex(args[0]))
		if err != nil {
			return err
		}
	}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	procs := append([]string{}, resp.ProcessList...)
	if len(procs) == router.MaxListSize {
		for i, count := 1, len(procs); count > 0 && i < MaxListIterations; i++ {
			req.From = i * router.MaxListSize
			resp, err := cl.Request(req, nil)
			if err != nil {
				return err
			}
			procs = append(procs, resp.ProcessList...)
			count = len(resp.ProcessList)
		}
	}

	buffer := new(bytes.Buffer)
	for _, proc := range procs {
		buffer.WriteString(proc + "\n")
	}
	fmt.Print(buffer.String())
	return err
}

func processInfo(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must provide a process id")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := api.MetaRequest{Method: "getProcessInfo"}
	req.ProcessID, err = hex.DecodeString(util.TrimHex(args[0]))
	if err != nil {
		return err
	}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Println(resp.Process.String())
	return nil
}

func processKeys(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must provide a process id")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := api.MetaRequest{Method: "getProcessKeys"}
	req.ProcessID, err = hex.DecodeString(util.TrimHex(args[0]))
	if err != nil {
		return err
	}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	if len(resp.EncryptionPublicKeys) == 0 {
		fmt.Print("this is not an encrypted poll")
		return err
	}

	buffer := new(bytes.Buffer)
	buffer.WriteString("Encryption Public Keys: ")
	for _, pubk := range resp.EncryptionPublicKeys {
		fmt.Fprintf(buffer, "%v ", pubk.Key)
	}
	buffer.WriteString("\nEncryption Private Keys: ")
	for _, pvk := range resp.EncryptionPrivKeys {
		fmt.Fprintf(buffer, "%v ", pvk.Key)
	}
	fmt.Print(buffer.String())
	return err
}

func getResults(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must provide a process id")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := api.MetaRequest{Method: "getResults"}
	req.ProcessID, err = hex.DecodeString(util.TrimHex(args[0]))
	if err != nil {
		return err
	}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	jresp, err := json.MarshalIndent(resp, "", " ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", jresp)
	return nil
}

func getResultsWeight(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must provide a process id")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := api.MetaRequest{Method: "getResultsWeight"}
	req.ProcessID, err = hex.DecodeString(util.TrimHex(args[0]))
	if err != nil {
		return err
	}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Println(resp.Weight)
	return err
}
