package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var (
	keysPrint   = color.New(color.FgCyan, color.Bold)
	valuesPrint = color.New(color.FgMagenta)
	infoPrint   = color.New(color.FgGreen)

	errAccountNotConfgirued = "account not configured, please select or add a new one"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	host := flag.String("host", "", "API host endpoint to connect with (such as http://localhost:9090/v2)")
	logLevel := flag.String("logLevel", "error", "log level")
	cfgFile := flag.String("config", filepath.Join(home, ".vocdoni-cli.json"), "config file")
	flag.Parse()
	log.Init(*logLevel, "stdout")

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
			color.New(color.FgHiGreen, color.Bold, color.Underline).Sprintf(cli.chainID),
			color.New(color.FgHiBlue).Sprintf(account),
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
				items.Sprint("⚙️\tHandle accounts"),
				items.Sprint("📖\tAccount info"),
				items.Sprint("✨\tAccount bootstrap"),
				items.Sprint("👛\tTransfer tokens"),
				items.Sprint("🕸️\tNetwork info"),
				items.Sprint("📝\tBuild a new census"),
				items.Sprint("🗳️\tCreate an election"),
				items.Sprint("☑️\tVote"),
				items.Sprint("🖧\tChange API endpoint host"),
				items.Sprint("💾\tSave config to file"),
				items.Sprint("❌\tQuit"),
			},
		}

		option, _, err := prompt.Run()
		if err != nil {
			errorp.Printf("prompt failed: %v\n", err)
			os.Exit(1)
		}
		switch option {
		case 0:
			if err := accountHandler(cli); err != nil {
				errorp.Println(err)
			}
		case 1:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfgirued)
				break
			}
			if err := accountInfo(cli); err != nil {
				errorp.Println(err)
			}
		case 2:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfgirued)
				break
			}
			if err := bootStrapAccount(cli); err != nil {
				errorp.Println(err)
			}
		case 3:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfgirued)
				break
			}
			if err := transfer(cli); err != nil {
				errorp.Println(err)
			}
		case 4:
			if err := networkInfo(cli); err != nil {
				errorp.Println(err)
			}
		case 6:
			if !accountIsSet(cli) {
				errorp.Println(errAccountNotConfgirued)
				break
			}
			if err := electionHandler(cli); err != nil {
				errorp.Println(err)
			}
		case 8:
			if err := hostHandler(cli); err != nil {
				errorp.Println(err)
			}
		case 9:
			if err := cli.save(); err != nil {
				errorp.Println(err)
			}
		case 10:
			os.Exit(0)
		default:
			errorp.Println("unknown option or not yet implemented")
		}
	}

}

func accountIsSet(c *vocdoniCLI) bool {
	return c.currentAccount >= 0
}

func accountHandler(c *vocdoniCLI) error {
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
		infoPrint.Printf("using account %d\n", opt)
		if err := c.useAccount(opt); err != nil {
			return err
		}
	}
	return nil
}

func accountSet(c *vocdoniCLI) error {
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
	infoPrint.Printf("set account %s\n", key)
	return c.setAccount(key, memo)
}

func accountGen(c *vocdoniCLI) error {
	p := ui.Prompt{
		Label: "Account memo note",
	}
	memo, err := p.Run()
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%x", util.RandomBytes(32))
	infoPrint.Printf("set account %s\n", memo)
	return c.setAccount(key, memo)
}

func accountInfo(c *vocdoniCLI) error {
	acc, err := c.api.Account("")
	if err != nil {
		return err
	}
	localAcc := c.getCurrentAccount()
	infoPrint.Printf("details for account %s\n", localAcc.Memo)
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ address"), valuesPrint.Sprintf(acc.Address.String()))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ public key"), valuesPrint.Sprintf(localAcc.PublicKey.String()))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ balance"), valuesPrint.Sprintf("%d", acc.Balance))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ nonce"), valuesPrint.Sprintf("%d", acc.Nonce))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ electionIndex"), valuesPrint.Sprintf("%d", acc.ElectionIndex))
	if acc.InfoURL != "" && acc.InfoURL != "none" {
		fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ info URL"), valuesPrint.Sprintf(acc.InfoURL))
	}

	return nil
}

func networkInfo(cli *vocdoniCLI) error {
	info, err := cli.api.ChainInfo()
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ API host"), valuesPrint.Sprintf(cli.config.Host.Host))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ chainID"), valuesPrint.Sprintf(info.ID))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ height"), valuesPrint.Sprintf("%d", *info.Height))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ block time"), valuesPrint.Sprintf("%v", *info.BlockTime))
	fmt.Printf("%s: %s\n", keysPrint.Sprintf(" ➥ timestamp"), valuesPrint.Sprintf("%d", *info.Timestamp))
	return nil
}

func bootStrapAccount(cli *vocdoniCLI) error {
	var faucetPkg []byte
	p := ui.Prompt{
		Label:     "Do you have a faucet package?",
		IsConfirm: true,
	}
	yes, err := p.Run()
	if err != nil {
		return err
	}
	if yes == "y" {
		p := ui.Prompt{
			Label: "Please enter the base64 payload",
		}
		faucetPayloadStr, err := p.Run()
		if err != nil {
			return err
		}
		p = ui.Prompt{
			Label: "Please enter the hexadecimal signature",
		}
		faucetSignatureStr, err := p.Run()
		if err != nil {
			return err
		}
		faucetPayload, err := base64.StdEncoding.DecodeString(faucetPayloadStr)
		if err != nil {
			return err
		}
		faucetSignature, err := hex.DecodeString(faucetSignatureStr)
		if err != nil {
			return err
		}
		faucetPkg, err = proto.Marshal(&models.FaucetPackage{
			Payload:   faucetPayload,
			Signature: faucetSignature,
		})
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("no package provided and automatic faucet is not implemented yet")
	}

	infoPrint.Printf("bootstraping account...\n")

	txHash, err := cli.api.AccountBootstrap(faucetPkg, &api.AccountMetadata{
		Name: map[string]string{"default": "vocdoni cli account " + cli.getCurrentAccount().Address.Hex()},
	})
	if err != nil {
		return err
	}
	infoPrint.Printf("transaction sent! hash %s\n", txHash.String())
	infoPrint.Printf("waiting for confirmation...")
	ok := cli.waitForTransaction(txHash)
	if !ok {
		return fmt.Errorf("transaction was not included")
	}
	infoPrint.Printf(" transaction confirmed!\n")
	return nil
}

func transfer(cli *vocdoniCLI) error {
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
		log.Infof("transfer canceled")
		return nil
	}

	txHash, err := cli.api.Transfer(dstAddress, uint64(amount))
	if err != nil {
		return err
	}
	infoPrint.Printf("transaction sent! hash %s\n", txHash.String())
	infoPrint.Printf("waiting for confirmation...")
	ok := cli.waitForTransaction(txHash)
	if !ok {
		return fmt.Errorf("transaction was not included")
	}
	infoPrint.Printf(" transaction confirmed!\n")
	return nil
}

func hostHandler(cli *vocdoniCLI) error {
	validateFunc := func(url string) error {
		log.Debugf("performing ping test to %s", url)
		_, err := http.NewRequest("GET", url+"/ping", nil)
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
	infoPrint.Printf("configuring API host to %s\n", host)
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

func electionHandler(cli *vocdoniCLI) error {
	infoPrint.Printf("preparing the eletion template...\n")
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
			{Title: map[string]string{"default": "question title"},
				Description: map[string]string{"default": "question description"},
				Choices: []api.ChoiceMetadata{
					{
						Title: map[string]string{"default": "1 choice title"},
						Value: 0},
					{
						Title: map[string]string{"default": "2 choice title"},
						Value: 1},
				}}},
		Census: api.CensusTypeDescription{
			Type:     "weighted",
			RootHash: make(types.HexBytes, 32),
		},
	}
	descriptionText, err := json.MarshalIndent(&description, "", " ")
	if err != nil {
		return err
	}

	file, err := ioutil.TempFile("", "election*")
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

	infoPrint.Printf("creating new election...\n")
	electionID, err := cli.api.NewElection(&description)
	if err != nil {
		return err
	}

	infoPrint.Printf("election transaction sent! electionID is %s\n", electionID.String())
	return nil
}
