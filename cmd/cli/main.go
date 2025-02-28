package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fatih/color"
	ui "github.com/manifoldco/promptui"
	flag "github.com/spf13/pflag"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

var (
	keysPrint   = color.New(color.FgCyan, color.Bold)
	valuesPrint = color.New(color.FgMagenta)
	infoPrint   = color.New(color.FgGreen)

	errAccountNotConfigured = "account not configured, please select or add a new one"
)

func main() {
	// Report the version before loading the config or logger init, just in case something goes wrong.
	// For the sake of including the version in the log, it's also included in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni version %q\n", internal.Version)

	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	host := flag.String("host", "", "API host endpoint to connect with (such as http://localhost:9090/v2)")
	logLevel := flag.String("logLevel", "error", "log level")
	cfgFile := flag.String("config", filepath.Join(home, ".vocdoni-cli.json"), "config file")
	flag.Parse()
	log.Init(*logLevel, "stdout", nil)
	log.Infow("starting "+filepath.Base(os.Args[0]), "version", internal.Version)

	cli, err := NewVocdoniCLI(*cfgFile, *host)
	if err != nil {
		log.Fatal(err)
	}

	accountInfoHeader := func() string {
		account := "account not configured"
		if a := cli.getCurrentAccount(); a != nil {
			account = fmt.Sprintf("%s [%s]", a.Memo, a.Address.String())
		}
		return fmt.Sprintf("%s | %s",
			color.New(color.FgHiGreen, color.Bold, color.Underline).Sprint(cli.chainID),
			color.New(color.FgHiBlue).Sprint(account),
		)
	}

	items := color.New(color.FgHiYellow, color.Bold)
	errorp := color.New(color.FgHiRed)

	for {
		prompt := ui.Select{
			Label:    accountInfoHeader(),
			HideHelp: true,
			Size:     10,
			Items: []string{
				items.Sprint("⚙️\tHandle accounts"),         // 0
				items.Sprint("📖\tAccount info"),             // 1
				items.Sprint("✍\tAccount set metadata"),     // 2
				items.Sprint("✨\tAccount bootstrap"),        // 3
				items.Sprint("👛\tTransfer tokens"),          // 4
				items.Sprint("🕸️\tNetwork info"),            // 5
				items.Sprint("📝\tBuild a new census"),       // 6
				items.Sprint("🗳️\tCreate an election"),      // 7
				items.Sprint("☑️\tSet validator"),           // 8
				items.Sprint("📝 Generate faucet package"),   // 9
				items.Sprint("🖧\tChange API endpoint host"), // 10
				items.Sprint("💾\tSave config to file"),      // 11
				items.Sprint("❌\tQuit"),                     // 12
			},
		}

		option, _, err := prompt.Run()
		if err != nil {
			errorp.Print("prompt failed: ", err, "\n")
			os.Exit(1)
		}
		switch option {
		case 0:
			if err := accountHandler(cli); err != nil {
				errorp.Println(err)
			}
		case 1:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfigured)
				break
			}
			if err := accountInfo(cli); err != nil {
				errorp.Println(err)
			}
		case 2:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfigured)
				break
			}
			if err := accountSetMetadata(cli); err != nil {
				errorp.Println(err)
			}
		case 3:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfigured)
				break
			}
			if err := bootStrapAccount(cli); err != nil {
				errorp.Println(err)
			}
		case 4:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfigured)
				break
			}
			if err := transfer(cli); err != nil {
				errorp.Println(err)
			}
		case 5:
			if err := networkInfo(cli); err != nil {
				errorp.Println(err)
			}
		case 7:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfigured)
				break
			}
			if err := electionHandler(cli); err != nil {
				errorp.Println(err)
			}
		case 8:
			if err := accountSetValidator(cli); err != nil {
				errorp.Println(err)
			}
		case 9:
			if err := faucetPkg(cli); err != nil {
				errorp.Println(err)
			}
		case 10:
			if err := hostHandler(cli); err != nil {
				errorp.Println(err)
			}
		case 11:
			if err := cli.save(); err != nil {
				errorp.Println(err)
			}
		case 12:
			os.Exit(0)
		default:
			errorp.Println("unknown option or not yet implemented")
		}
	}
}

func accountIsSet(c *VocdoniCLI) bool {
	return c.currentAccount >= 0
}

func accountHandler(c *VocdoniCLI) error {
	accountAddNewStr := "-> import an account (from hexadecimal private key)"
	accountGenerateStr := "-> generate a new account"
	p := ui.Select{
		Label: "Select an account",
		Items: append(c.listAccounts(), accountAddNewStr, accountGenerateStr),
	}

	opt, item, err := p.Run()
	if err != nil {
		return err
	}

	switch item {
	case accountAddNewStr:
		if err := accountSet(c); err != nil {
			return err
		}
	case accountGenerateStr:
		if err := accountGen(c); err != nil {
			return err
		}
	default:
		infoPrint.Print("using account ", opt, "\n")
		if err := c.useAccount(opt); err != nil {
			return err
		}
	}
	return nil
}

func accountSet(c *VocdoniCLI) error {
	p := ui.Prompt{
		Label: "Account private key",
	}
	key, err := p.Run()
	if err != nil {
		return err
	}
	p = ui.Prompt{
		Label: "Account memo note",
	}
	memo, err := p.Run()
	if err != nil {
		return err
	}
	infoPrint.Print("set account ", key, "\n")
	return c.setAPIaccount(key, memo)
}

func accountGen(c *VocdoniCLI) error {
	p := ui.Prompt{
		Label: "Account memo note",
	}
	memo, err := p.Run()
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%x", util.RandomBytes(32))
	infoPrint.Print("set account ", memo, "\n")
	return c.setAPIaccount(key, memo)
}

func accountInfo(c *VocdoniCLI) error {
	acc, err := c.api.Account("")
	if err != nil {
		return err
	}
	localAcc := c.getCurrentAccount()
	infoPrint.Print("details for account ", localAcc.Memo, "\n")
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ address"), valuesPrint.Sprint(acc.Address.String()))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ public key"), valuesPrint.Sprint(localAcc.PublicKey.String()))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ balance"), valuesPrint.Sprint(acc.Balance))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ nonce"), valuesPrint.Sprint(acc.Nonce))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ electionIndex"), valuesPrint.Sprint(acc.ElectionIndex))
	if acc.InfoURL != "" && acc.InfoURL != "none" {
		fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ info URL"), valuesPrint.Sprint(acc.InfoURL))
	}
	if acc.Metadata != nil {
		accMetadata, err := json.MarshalIndent(acc.Metadata, "", "  ")
		if err != nil {
			log.Debug("account metadata cannot be unmarshal")
		} else {
			fmt.Printf("%s:\n%s\n", keysPrint.Sprint(" ➥ metadata"), valuesPrint.Sprint(string(accMetadata)))
		}
	}

	return nil
}

func networkInfo(cli *VocdoniCLI) error {
	info, err := cli.api.ChainInfo()
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ API host"), valuesPrint.Sprint(cli.config.Host.Host))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ chainID"), valuesPrint.Sprint(info.ID))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ height"), valuesPrint.Sprint(info.Height))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ block time"), valuesPrint.Sprint(info.BlockTime))
	fmt.Printf("%s: %s\n", keysPrint.Sprint(" ➥ timestamp"), valuesPrint.Sprint(info.Timestamp))
	return nil
}

func bootStrapAccount(cli *VocdoniCLI) error {
	var faucetPkg *models.FaucetPackage
	p := ui.Prompt{
		Label:   "Do you have a faucet package? [y,n]",
		Default: "n",
	}
	yes, err := p.Run()
	if err != nil {
		return err
	}
	if yes == "y" {
		p := ui.Prompt{
			Label: "Please enter the base64 faucet package",
		}
		faucetPkgString, err := p.Run()
		if err != nil {
			return err
		}

		faucetPkgBytes, err := base64.StdEncoding.DecodeString(faucetPkgString)
		if err != nil {
			return err
		}
		faucetPkg, err = apiclient.UnmarshalFaucetPackage(faucetPkgBytes)
		if err != nil {
			return err
		}
	} else {
		info, err := cli.api.ChainInfo()
		if err != nil {
			return err
		}
		infoPrint.Print("trying to fetch faucet package from default remote service...\n")
		faucetPkg, err = apiclient.GetFaucetPackageFromDefaultService(cli.api.MyAddress().Hex(), info.ID)
		if err != nil {
			return err
		}
		infoPrint.Print("got faucet package!")
	}

	infoPrint.Print("bootstraping account...\n")

	txHash, err := cli.api.AccountBootstrap(faucetPkg, &api.AccountMetadata{
		Name: map[string]string{"default": "vocdoni cli account " + cli.getCurrentAccount().Address.Hex()},
	}, nil)
	if err != nil {
		return err
	}
	infoPrint.Print("transaction sent! hash ", txHash.String(), "\n")
	infoPrint.Print("waiting for confirmation...")
	ok := cli.waitForTransaction(txHash)
	if !ok {
		return fmt.Errorf("transaction was not included")
	}
	infoPrint.Print(" transaction confirmed!\n")
	return nil
}

func transfer(cli *VocdoniCLI) error {
	s := ui.Select{
		Label: "Select a destination account",
		Items: append(cli.listAccounts(), "to external account"),
	}

	opt, item, err := s.Run()
	if err != nil {
		return err
	}

	var dstAddress common.Address

	if item != "to external account" {
		account, err := cli.getAccount(opt)
		if err != nil {
			return err
		}
		dstAddress = account.Address
	} else {
		p := ui.Prompt{
			Label: "destination address",
		}
		destAddrStr, err := p.Run()
		if err != nil {
			return err
		}
		if _, err := cli.api.Account(destAddrStr); err != nil {
			return err
		}
		addr := common.HexToAddress(destAddrStr)
		dstAddress = addr
	}

	p := ui.Prompt{
		Label: "amount",
	}
	amountStr, err := p.Run()
	if err != nil {
		return err
	}
	amount, err := strconv.Atoi(amountStr)
	if err != nil {
		return err
	}

	p = ui.Prompt{
		Label: fmt.Sprintf("please confirm you want to transfer %d VOC tokens to %s",
			amount, dstAddress.String()),
		IsConfirm: true,
	}
	item, err = p.Run()
	if err != nil {
		return err
	}
	if item == "N" {
		log.Info("transfer canceled")
		return nil
	}

	txHash, err := cli.api.Transfer(dstAddress, uint64(amount))
	if err != nil {
		return err
	}
	infoPrint.Print("transaction sent! hash ", txHash.String(), "\n")
	infoPrint.Print("waiting for confirmation...")
	ok := cli.waitForTransaction(txHash)
	if !ok {
		return fmt.Errorf("transaction was not included")
	}
	infoPrint.Print(" transaction confirmed!\n")
	return nil
}

func faucetPkg(cli *VocdoniCLI) error {
	// FaucetPackage represents the data of a faucet package
	type FaucetPackage struct {
		// FaucetPackagePayload is the Vocdoni faucet package payload
		FaucetPayload []byte `json:"faucetPayload"`
		// Signature is the signature for the vocdoni faucet payload
		Signature []byte `json:"signature"`
	}
	signer := ethereum.SignKeys{}
	if err := signer.AddHexKey(cli.getCurrentAccount().PrivKey.String()); err != nil {
		return err
	}
	a := ui.Prompt{
		Label: "destination address",
	}
	addrString, err := a.Run()
	if err != nil {
		return err
	}
	to := common.HexToAddress(addrString)
	n := ui.Prompt{
		Label: "amount",
	}
	amountString, err := n.Run()
	if err != nil {
		return err
	}
	amount, err := strconv.Atoi(amountString)
	if err != nil {
		return err
	}
	fpackage, err := vochain.GenerateFaucetPackage(&signer, to, uint64(amount))
	if err != nil {
		return err
	}
	fpackageBytes, err := json.Marshal(FaucetPackage{
		FaucetPayload: fpackage.Payload,
		Signature:     fpackage.Signature,
	})
	if err != nil {
		return err
	}
	infoPrint.Print("faucet package for ", to.Hex(), " with amount ", amount, ": [ ", base64.StdEncoding.EncodeToString(fpackageBytes), " ]\n")
	return nil
}

func hostHandler(cli *VocdoniCLI) error {
	validateFunc := func(url string) error {
		log.Debug("performing ping test to ", url)
		_, err := http.NewRequest("GET", url+"/ping", http.NoBody)
		return err
	}
	p := ui.Prompt{
		Label:       "API host URL",
		Default:     cli.config.Host.String(),
		Validate:    validateFunc,
		Pointer:     ui.DefaultCursor,
		HideEntered: false,
	}
	host, err := p.Run()
	if err != nil {
		return err
	}
	infoPrint.Print("configuring API host to ", host, "\n")
	if err := cli.setHost(host); err != nil {
		return err
	}

	p = ui.Prompt{
		Label:   "API auth token",
		Default: cli.config.Token.String(),
		Pointer: ui.DefaultCursor,
	}
	token, err := p.Run()
	if err != nil {
		return err
	}
	return cli.setAuthToken(token)
}

func accountSetValidator(cli *VocdoniCLI) error {
	infoPrint.Print("enter the name and a public key of the validator, leave it bank for using the selected account\n")

	n := ui.Prompt{
		Label: "name",
	}
	name, err := n.Run()
	if err != nil {
		return err
	}

	p := ui.Prompt{
		Label: "public key",
	}
	pubKeyStr, err := p.Run()
	if err != nil {
		return err
	}
	pubKey := cli.getCurrentAccount().PublicKey
	if pubKeyStr != "" {
		pubKey, err = hex.DecodeString(pubKeyStr)
		if err != nil {
			return err
		}
	}

	hash, err := cli.api.AccountSetValidator(pubKey, name)
	if err != nil {
		return err
	}

	infoPrint.Print("transaction sent! hash ", hash.String(), "\n")
	infoPrint.Print("waiting for confirmation...")
	ok := cli.waitForTransaction(hash)
	if !ok {
		return fmt.Errorf("transaction was not included")
	}
	infoPrint.Print(" transaction confirmed!\n")

	return nil
}

func accountSetMetadata(cli *VocdoniCLI) error {
	currentAccount, err := cli.api.Account("")
	if err != nil {
		return err
	}
	if currentAccount == nil {
		return fmt.Errorf("account does not exist yet")
	}

	accMeta := api.AccountMetadata{
		Version:     "1.0",
		Name:        map[string]string{"default": "vocdoni account " + cli.getCurrentAccount().Address.Hex()},
		Description: map[string]string{"default": "my account description"},
		Media: &api.AccountMedia{
			Logo:   "https://upload.wikimedia.org/wikipedia/commons/f/f6/HAL9000.svg",
			Avatar: "https://upload.wikimedia.org/wikipedia/commons/f/f6/HAL9000.svg",
			Header: "https://images.unsplash.com/photo-1543000968-1fe3fd3b714e",
		},
	}

	if currentAccount.Metadata != nil {
		log.Info("account has metadata (", currentAccount.InfoURL, ") let's update it")
		accMeta = *currentAccount.Metadata
	}

	accMetaText, err := json.MarshalIndent(&accMeta, "", " ")
	if err != nil {
		return err
	}

	file, err := os.CreateTemp("", "accMeta*")
	if err != nil {
		return err
	}
	_, err = file.Write(accMetaText)
	if err != nil {
		return err
	}
	fileName := file.Name()
	if err := file.Close(); err != nil {
		return nil
	}

	if err := OpenFileInEditor(fileName, GetPreferredEditorFromEnvironment); err != nil {
		return err
	}

	p := ui.Prompt{
		Label: fmt.Sprintf(
			"A template file has been created on %s for editing. Select 'y' once finished",
			fileName),
		IsConfirm: true,
	}
	confirm, err := p.Run()
	if err != nil {
		return err
	}
	if confirm == "N" {
		return nil
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &accMeta); err != nil {
		return err
	}

	infoPrint.Print("set account metadata...\n")
	txHash, err := cli.api.AccountSetMetadata(&accMeta)
	if err != nil {
		return err
	}
	infoPrint.Print("account transaction sent! hash is ", txHash.String(), "\n")
	infoPrint.Print("waiting for confirmation...\n")
	if !cli.waitForTransaction(txHash) {
		return fmt.Errorf("transaction was not included")
	}
	return nil
}

func electionHandler(cli *VocdoniCLI) error {
	infoPrint.Print("preparing the election template...\n")
	description := api.ElectionDescription{
		Title:        map[string]string{"default": "election title"},
		Description:  map[string]string{"default": "election description"},
		Header:       "https://images.unsplash.com/photo-1540910419892-4a36d2c3266c",
		StreamURI:    "",
		StartDate:    time.Now().Add(time.Minute * 20),
		EndDate:      time.Now().Add(time.Hour * 48),
		VoteType:     api.VoteType{MaxVoteOverwrites: 1},
		ElectionType: api.ElectionType{Autostart: true, Interruptible: true},
		Questions: []api.Question{
			{
				Title:       map[string]string{"default": "question title"},
				Description: map[string]string{"default": "question description"},
				Choices: []api.ChoiceMetadata{
					{
						Title: map[string]string{"default": "1 choice title"},
						Value: 0,
					},
					{
						Title: map[string]string{"default": "2 choice title"},
						Value: 1,
					},
				},
			},
		},
		Census: api.CensusTypeDescription{
			Type:     "weighted",
			RootHash: make(types.HexBytes, 32),
			Size:     100,
		},
	}
	descriptionText, err := json.MarshalIndent(&description, "", " ")
	if err != nil {
		return err
	}

	file, err := os.CreateTemp("", "election*")
	if err != nil {
		return err
	}
	_, err = file.Write(descriptionText)
	if err != nil {
		return err
	}
	fileName := file.Name()
	if err := file.Close(); err != nil {
		return nil
	}

	if err := OpenFileInEditor(fileName, GetPreferredEditorFromEnvironment); err != nil {
		return err
	}

	p := ui.Prompt{
		Label: fmt.Sprintf(
			"A template file has been created on %s for editing. Select 'y' once finished",
			fileName),
		IsConfirm: true,
	}
	confirm, err := p.Run()
	if err != nil {
		return err
	}
	if confirm == "N" {
		return nil
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &description); err != nil {
		return err
	}

	infoPrint.Print("creating new election...\n")
	electionID, err := cli.api.NewElection(&description, true)
	if err != nil {
		return err
	}

	infoPrint.Print("election transaction sent! electionID is ", electionID.String(), "\n")
	return nil
}
