package commands

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
)

const (
	scryptN = ethkeystore.StandardScryptN
	scryptP = ethkeystore.StandardScryptP
	keyExt  = ".vokey"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Create, import and list keys.",
	Long: `In Vochain, we make the distinction between a key and an account.
	A key controls an account which is stored in the blockchain's state.
	The Key itself is stored in encrypted form on the user's disk.`,
}

var keysNewCmd = &cobra.Command{
	Use:   "new [pre-existing keyfile]",
	Short: "Generate a new key and save it in go-ethereum's JSON format.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var key *ethkeystore.Key
		var keyPath string

		fmt.Fprintf(Stdout, `The key will be generated the key and saved, encrypted, on your disk.
		Remember to run 'vocli account set <address>' afterwards to create an account for this key on the vochain.`)

		password, err := PromptPassword("Your new key file will be locked with a password. Please give a password: ")
		if err != nil {
			return err
		}

		key, keyPath, err = storeNewKey(rand.Reader, password)
		if err != nil {
			return fmt.Errorf("couldn't generate a new key: %v", err)
		}
		fmt.Fprintf(Stdout, "\nYour new key was generated\n")
		fmt.Fprintf(Stdout, "Public address of the key:   %s\n", key.Address.Hex())
		fmt.Fprintf(Stdout, "Path of the secret key file: %s\n", keyPath)
		fmt.Fprintf(Stdout, "- As usual, please BACKUP your key file and REMEMBER your password!\n")

		return nil
	},
}

var keysImportCmd = &cobra.Command{
	Use:   "import <keyfile>",
	Short: "Reads a plain, unencrypted file containing a private key (hexstring) and stores it in go-ethereum's JSON format.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyfile := args[0]
		key, err := crypto.LoadECDSA(keyfile)
		if err != nil {
			utils.Fatalf("Failed to load the private key: %v", err)
		}
		passphrase, err := PromptPassword("Your new account is locked with a password. Please give a password.")
		if err != nil {
			return err
		}
		id, err := uuid.NewRandom()
		if err != nil {
			panic(fmt.Sprintf("Could not create random uuid: %v", err))
		}
		k := &ethkeystore.Key{
			Id:         id,
			Address:    crypto.PubkeyToAddress(key.PublicKey),
			PrivateKey: key,
		}

		keyjson, err := ethkeystore.EncryptKey(k, passphrase, scryptN, scryptP)
		if err != nil {
			return err
		}

		keyPath, err := generateKeyFilename(k.Address)
		if err != nil {
			return err
		}
		err = writeKeyFile(keyPath, keyjson)
		if err != nil {
			return err
		}
		fmt.Fprintln(Stdout, "Public address of the key:", crypto.PubkeyToAddress(key.PublicKey))
		fmt.Fprintln(Stdout, "Path of the secret key file:", keyPath)

		return nil
	},
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists the keys that vocli knows about.",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		keysDir, err := getKeysDir(home)
		if err != nil {
			return err
		}

		// check if directory exists first
		if _, err := os.Stat(keysDir); os.IsNotExist(err) {
			return err
		}

		return filepath.WalkDir(keysDir, func(path string, info os.DirEntry, err error) error {
			if !info.IsDir() && (filepath.Ext(path) == keyExt) {
				fmt.Fprintln(Stdout, path)
			}
			return nil
		})
	},
}

var keysChangePasswordCmd = &cobra.Command{
	Use:   "changepassword <filename>",
	Short: "Changes the password of a keyfile.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		k, _, err := openKeyfile(args[0], "Enter old password")
		if err != nil {
			return err
		}
		password = ""
		passwordNew, err := PromptPassword("Enter new password:")
		if err != nil {
			return err
		}
		keyJSONNew, err := ethkeystore.EncryptKey(k, passwordNew, scryptN, scryptP)
		if err != nil {
			return fmt.Errorf("couldn't encrypt the key with new password: %s", err)
		}

		return writeKeyFile(args[0], keyJSONNew)
	},
}

var keysShowPrivKeyCmd = &cobra.Command{
	Use:   "showprivkey <filename>",
	Short: "Prints the hex private key.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		k, _, err := openKeyfile(args[0], "Enter password:")
		if err != nil {
			return err
		}
		p := crypto.FromECDSA(k.PrivateKey)
		pHex := hex.EncodeToString(p)
		fmt.Fprintf(Stdout, "\n%s\n", pHex)
		return nil
	},
}

func openKeyfile(path, prompt string) (*ethkeystore.Key, *ethereum.SignKeys, error) {
	keyJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	password, err := PromptPassword(prompt)
	if err != nil {
		return nil, nil, err
	}
	k, err := ethkeystore.DecryptKey(keyJSON, string(password))
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't decrypt the key with given password: %s", err)
	}

	signer := ethereum.NewSignKeys()
	signer.Private = *k.PrivateKey
	signer.Public = k.PrivateKey.PublicKey
	return k, signer, nil
}

// PromptPassword asks the user for a password, but if one was specified through
// the --password argument, it simply returns that variable without printing
// anything
func PromptPassword(prompt string) (string, error) {
	if password == "" {
		fmt.Fprint(Stdout, prompt)
		// if this is a real terminal (not a test case mocked stdin), use the
		// standard term.ReadPassword() function
		if term.IsTerminal(int(Stdin.Fd())) {
			p, err := term.ReadPassword(int(Stdin.Fd()))
			fmt.Fprintln(Stdout, "")
			if err != nil {
				return "", err
			}
			return string(p), nil
		} else {
			// if this is a test case and stdin is mocked out, read from Stdin in a
			// different way
			reader := bufio.NewReader(Stdin)
			p, err := reader.ReadString('\n') // returns 'newPassword\n'
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(p), nil // newPassword
		}
	}
	return password, nil
}

func createAccount(key *ethkeystore.Key, url, infoUri, faucetPkg string) error {
	fmt.Fprintf(Stdout, "Sending SetAccountInfo for key %s on %s\n", key.Address.String(), url)

	signer := ethereum.NewSignKeys()
	signer.Private = *key.PrivateKey
	signer.Public = key.PrivateKey.PublicKey
	client, err := client.New(url)
	if err != nil {
		return err
	}

	nonce, err := getNonce(client, key.Address.Hex())
	if err != nil && err != vochain.ErrAccountNotExist {
		return err
	}

	if faucetHex != "" {
		faucetPkg, err := parseFaucetPayloadHex(faucetHex)
		if err != nil {
			return err
		}
		err = client.CreateOrSetAccount(signer, signer.Address(), infoUri, nonce, faucetPkg)
		if err != nil {
			return err
		}
	} else {
		err = client.CreateOrSetAccount(signer, signer.Address(), infoUri, nonce, nil)
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(Stdout, "Account created/updated on chain %s\n", v.GetString(urlKey))
	return nil
}

func parseFaucetPayloadHex(faucetPayload string) (*models.FaucetPackage, error) {
	faucetPackageRaw, err := hex.DecodeString(faucetPayload)
	if err != nil {
		return nil, fmt.Errorf("could not decode the hex-encoded input: %s", err)
	}
	faucetPackage := &models.FaucetPackage{}
	err = proto.Unmarshal(faucetPackageRaw, faucetPackage)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal the faucet package: %s", err)
	}

	return faucetPackage, nil
}
