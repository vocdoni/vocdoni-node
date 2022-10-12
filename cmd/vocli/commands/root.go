package commands

import (
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"sort"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.vocdoni.io/dvote/log"
	client "go.vocdoni.io/dvote/rpcclient"
	"go.vocdoni.io/dvote/vochain"
	"google.golang.org/protobuf/proto"
)

const envPrefix = "VOCHAIN"
const urlKey = "url"

var nonce uint32
var home string
var password string
var faucetHex string

// when running vocli in a test harness which has its own logger setup,
// SetupLogPackage should be false so that vocli won't override the test
// harness's logger settings
var SetupLogPackage bool
var Stdout io.Writer
var Stderr io.Writer
var Stdin *os.File
var v *viper.Viper

var txTypes []string

func init() {
	Stdout = os.Stdout
	Stderr = os.Stderr
	Stdin = os.Stdin
	RootCmd.CompletionOptions.DisableDefaultCmd = true
	SetupLogPackage = true

	v = viper.New()
	v.SetConfigName("vocli")
	v.SetConfigType("yaml")
	v.AddConfigPath(home)
	v.AddConfigPath(".")
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv() // viper.Get("url") -> VOCHAIN_URL envvar

	RootCmd.PersistentFlags().StringP("url", "u", "https://gw1.dev.vocdoni.net/dvote", "Gateway RPC URL; $VOCHAIN_URL")
	v.BindPFlag("url", RootCmd.PersistentFlags().Lookup("url")) // viper.Get("url") -> cobra flags --url
	RootCmd.PersistentFlags().StringVar(&home, "home", path.Join(os.Getenv("HOME"), ".dvote"),
		"root directory where all vochain files are stored; $VOCHAIN_HOME")
	v.BindPFlag("home", RootCmd.PersistentFlags().Lookup("home"))
	RootCmd.PersistentFlags().StringVar(&password, "password", "", "supply the password as an argument instead of prompting")
	RootCmd.PersistentFlags().BoolP("debug", "d", false, "prints additional information; $VOCHAIN_DEBUG")
	v.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug"))
	RootCmd.PersistentFlags().Uint32VarP(&nonce, "nonce", "n", 0, `account nonce to use when sending transaction
	(useful when it cannot be queried ahead of time, e.g. offline transaction signing)`)
	RootCmd.AddCommand(accCmd)
	RootCmd.AddCommand(sendCmd)
	RootCmd.AddCommand(claimFaucetCmd)
	RootCmd.AddCommand(genFaucetCmd)
	RootCmd.AddCommand(mintCmd)
	RootCmd.AddCommand(keysCmd)
	RootCmd.AddCommand(txCostCmd)
	RootCmd.AddCommand(adminCmd)
	RootCmd.AddCommand(processCmd)
	accCmd.AddCommand(accInfoCmd)
	accCmd.AddCommand(accSetInfoCmd)
	accCmd.AddCommand(accTreasurerCmd)
	accCmd.AddCommand(accAddDelegateCmd)
	accCmd.AddCommand(accDelDelegateCmd)
	keysCmd.AddCommand(keysNewCmd)
	keysCmd.AddCommand(keysImportCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysChangePasswordCmd)
	keysCmd.AddCommand(keysShowPrivKeyCmd)
	txCostCmd.AddCommand(txCostGetCmd)
	txCostCmd.AddCommand(txCostSetCmd)
	adminCmd.AddCommand(setOracleCmd)
	processCmd.AddCommand(setProcessCmd)

	keysNewCmd.Flags().StringVar(&faucetHex, "faucet", "", `specify an optional hex-encoded faucet payload to immediately top up
	the new account with tokens`)

	// it's useful to have a static list of TxTypes built from the map
	for k := range vochain.TxCostNameToTxTypeMap {
		txTypes = append(txTypes, k)
	}
	sort.Strings(txTypes)

	if err := viper.ReadInConfig(); err != nil {
		log.Warnf("could not find configuration file %s", err)
	}
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(0)
}

var RootCmd = &cobra.Command{
	Use:   "vocli",
	Short: "vocli is a convenience CLI that helps you do things on Vochain",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if SetupLogPackage {
			if v.GetBool("debug") {
				log.Init("debug", "stdout")
			} else {
				log.Init("error", "stdout")
			}
		}
		log.Debugf("vocli config: %v", v.AllSettings())
	},
}

var sendCmd = &cobra.Command{
	Use:   "send <from keystore, recipient, amount>",
	Short: "Send tokens to another account",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		amount, err := strconv.ParseUint(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("sorry, what amount did you say again? %s", err)
		}

		_, signer, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return fmt.Errorf("could not open keyfile %s", err)
		}
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		nonce, err := getNonce(c, signer.AddressString())
		if err != nil {
			return fmt.Errorf("could not lookup the account's nonce, try specifying manually: %s", err)
		}

		err = c.SendTokens(signer, common.HexToAddress(args[1]), nonce, amount)
		return err
	},
}

var claimFaucetCmd = &cobra.Command{
	Use:   "claimfaucet <to keystore, hex encoded faucet package>",
	Short: "Claim tokens from another account, using a payload generated from that account that acts as an authorization.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		faucetPackage, err := parseFaucetPayloadHex(args[1])
		if err != nil {
			return err
		}

		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}
		_, signer, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return err
		}

		nonce, err := getNonce(c, signer.AddressString())
		if err != nil {
			return fmt.Errorf("could not lookup the nonce for %s, try specifying manually: %s", signer.AddressString(), err)
		}

		err = c.CollectFaucet(signer, nonce, faucetPackage)
		if err != nil {
			return err
		}
		return nil
	},
}

var genFaucetCmd = &cobra.Command{
	Use:   "genfaucet <from keystore, recipient, amount>",
	Short: "Generate a payload allowing another account to claim tokens from this account.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, signer, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return fmt.Errorf("could not open keyfile %s", err)
		}
		amount, err := strconv.ParseUint(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("sorry, what amount did you say again? %s", err)
		}

		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		faucetPackage, err := c.GenerateFaucetPackage(signer, common.HexToAddress(args[1]), amount, rand.Uint64())
		if err != nil {
			return err
		}

		faucetPackageMarshaled, err := proto.Marshal(faucetPackage)
		if err != nil {
			return err
		}
		fmt.Fprintln(Stdout, hex.EncodeToString(faucetPackageMarshaled))
		return nil
	},
}

var mintCmd = &cobra.Command{
	Use:   "mint <treasurer's keystore, recipient, amount>",
	Short: "Mint more tokens to an address. Only the Treasurer may do this.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		amount, err := strconv.ParseUint(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("sorry, what amount did you say again? %s", err)
		}
		c, err := client.New(v.GetString(urlKey))
		if err != nil {
			return err
		}

		_, signer, err := openKeyfile(args[0], "Please unlock your key: ")
		if err != nil {
			return fmt.Errorf("could not open keyfile %s", err)
		}

		t, err := c.GetTreasurer()
		if err != nil {
			return fmt.Errorf("could not lookup the account's nonce, try specifying manually: %s", err)
		}

		err = c.MintTokens(signer, common.HexToAddress(args[1]), t.GetNonce(), amount)
		return err
	},
}

// getNonce calls Client.GetAccount to get the current information of a normal
// account. It is not to be used for the treasurer account.
func getNonce(c *client.Client, address string) (uint32, error) {
	resp, err := c.GetAccount(common.HexToAddress(address))
	if err != nil {
		return 0, nil
	}
	if resp == nil {
		return 0, vochain.ErrAccountNotExist
	}
	return resp.Account.Nonce, nil
}

// getTreasurerNonce is like getNonce but only for the treasurer
func getTreasurerNonce(c *client.Client) (uint32, error) {
	resp, err := c.GetTreasurer()
	if err != nil {
		return 0, fmt.Errorf("could not lookup the treasurer's nonce, try specifying manually: %s", err)
	}
	return resp.Nonce, nil
}
