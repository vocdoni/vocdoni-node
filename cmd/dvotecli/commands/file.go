package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/types"
)

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "file subcommands",
}

var fileAddCmd = &cobra.Command{
	Use:   "add [name] [base64 payload]",
	Short: "add file from a Base64 payload",
	RunE:  fileAdd,
}

var fileFetchCmd = &cobra.Command{
	Use:   "fetch [ipfs://<hash>]",
	Short: "fetch file from URI, Base64 encoded",
	RunE:  fileFetch,
}

var pinListCmd = &cobra.Command{
	Use:   "list",
	Short: "list pinned files",
	RunE:  pinList,
}

func init() {
	rootCmd.AddCommand(fileCmd)
	fileCmd.AddCommand(fileAddCmd)
	fileCmd.AddCommand(fileFetchCmd)
	fileCmd.AddCommand(pinListCmd)
}

func fileAdd(cmd *cobra.Command, args []string) error {
	if err := opt.checkSignKey(); err != nil {
		return err
	}

	if len(args) < 2 {
		return fmt.Errorf("you must provide a file name and payload")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{
		Method: "addFile",
		Type:   "ipfs",
	}
	req.Content, err = base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		return err
	}
	req.Name = args[0]
	resp, err := cl.Request(req, opt.signKey)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("URI: %v", resp.URI)

	return err
}

func fileFetch(cmd *cobra.Command, args []string) error {
	if err := opt.checkSignKey(); err != nil {
		return err
	}

	if len(args) < 1 {
		return fmt.Errorf("you must provide a file URI")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{
		Method: "fetchFile",
		URI:    args[0],
	}
	resp, err := cl.Request(req, opt.signKey)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("%v", base64.StdEncoding.EncodeToString(resp.Content))

	return err
}

func pinList(cmd *cobra.Command, args []string) error {
	if err := opt.checkSignKey(); err != nil {
		return err
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	// Increase the default response size for large lists
	cl.Conn.SetReadLimit(32768 * 16)

	req := types.MetaRequest{Method: "pinList"}
	resp, err := cl.Request(req, opt.signKey)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	pins := make(map[string]string)
	if err := json.Unmarshal(resp.Files, &pins); err != nil {
		return err
	}

	buffer := new(bytes.Buffer)
	for uri, t := range pins {
		buffer.WriteString(uri + " " + t + "\n")
	}
	fmt.Print(buffer.String())
	return err
}
