package commands

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

var censusCmd = &cobra.Command{
	Use:   "census",
	Short: "census subcommands",
}

var censusAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add [census id] [claim1] ...",
	RunE:  censusAdd,
}

var claimCmd = &cobra.Command{
	Use:   "claim [id] [key]",
	Short: "adds a single claim (hashed pubkey) to a census id",
	RunE:  addClaim,
}

var getRootCmd = &cobra.Command{
	Use:   "root [id]",
	Short: "return the root hash of the census merkle tree",
	RunE:  getRoot,
}

var getSizeCmd = &cobra.Command{
	Use:   "size [id]",
	Short: "return the size of the census",
	RunE:  getSize,
}

var genProofCmd = &cobra.Command{
	Use:   "genproof [id] [key] [value]",
	Short: "generate a proof of a given value",
	RunE:  genProof,
}

func init() {
	rootCmd.AddCommand(censusCmd)
	censusCmd.AddCommand(censusAddCmd)
	censusCmd.AddCommand(claimCmd)
	censusCmd.AddCommand(getRootCmd)
	censusCmd.AddCommand(getSizeCmd)
	censusCmd.AddCommand(genProofCmd)
	claimCmd.Flags().BoolVarP(&opt.digested, "digested", "", false,
		"will digest value in the gateway if false")
}

func censusAdd(cmd *cobra.Command, args []string) error {
	if err := opt.checkSignKey(); err != nil {
		return err
	}

	if len(args) < 1 {
		return fmt.Errorf("you must provide a census id")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{
		Method:   "addCensus",
		CensusID: args[0],
	}

	if len(args) > 1 {
		req.PubKeys = args[1:]
	}

	resp, err := cl.Request(req, opt.signKey)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("CensusID: %v\n", resp.CensusID)
	fmt.Printf("URI: %v\n", resp.URI)

	return err
}

func addClaim(cmd *cobra.Command, args []string) error {
	if err := opt.checkSignKey(); err != nil {
		return err
	}

	if len(args) < 2 {
		return fmt.Errorf("you must provide a census id and a claim key")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{
		Method:    "addClaim",
		CensusID:  args[0],
		Digested:  opt.digested,
		CensusKey: []byte(args[1]),
	}

	resp, err := cl.Request(req, opt.signKey)
	if err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("Root: %v\n", resp.Root)

	return err
}

func getRoot(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must provide a census id")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{
		Method:   "getRoot",
		CensusID: args[0],
	}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("Root: %v", hex.EncodeToString(resp.Root))
	return err
}

func getSize(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must provide a census id")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{
		Method:   "getSize",
		CensusID: args[0],
	}
	resp, err := cl.Request(req, nil)
	if err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf(resp.Message)
	}
	fmt.Printf("Size: %v", resp.Size)
	return err
}

func genProof(cmd *cobra.Command, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("you must provide a census id, a key, and a value to proof")
	}

	cl, err := client.New(opt.host)
	if err != nil {
		return err
	}
	defer cl.CheckClose(&err)

	req := types.MetaRequest{
		Method:    "genProof",
		CensusID:  args[0],
		Digested:  opt.digested,
		CensusKey: []byte(args[1]),
	}
	req.CensusValue, err = hex.DecodeString(util.TrimHex(args[2]))
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
	fmt.Printf("Siblings: %v", resp.Siblings)
	return err
}
