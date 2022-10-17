package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

var transactionConfirmationThreshold = 30 * time.Second

type Config struct {
	Accounts []Account  `json:"accounts"`
	Host     *url.URL   `json:"host"`
	Token    *uuid.UUID `json:"token"`
}

func (c *Config) Load(filepath string) error {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, c)
}

func (c *Config) Save(filepath string) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, data, 0o600)
}

type Account struct {
	PrivKey   types.HexBytes `json:"privKey"`
	Memo      string         `json:"memo"`
	Address   common.Address `json:"address"`
	PublicKey types.HexBytes `json:"pubKey"`
}

type vocdoniCLI struct {
	filepath string
	config   *Config
	api      *apiclient.HTTPclient
	chainID  string

	currentAccount int
}

func NewVocdoniCLI(configFile, host string) (*vocdoniCLI, error) {
	cfg := Config{}
	if err := cfg.Load(configFile); err != nil {
		return nil, err
	}
	if cfg.Token == nil {
		t := uuid.New()
		cfg.Token = &t
	} else {
		log.Infof("new bearer auth token %s", *cfg.Token)
	}

	var err error
	if host != "" {
		cfg.Host, err = url.Parse(host)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Host == nil {
		return nil, fmt.Errorf("no API server host configured")
	}
	hostURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	api, err := apiclient.NewHTTPclient(hostURL, cfg.Token)
	if err != nil {
		return nil, err
	}
	return &vocdoniCLI{
		filepath:       configFile,
		config:         &cfg,
		api:            api,
		chainID:        api.ChainID(),
		currentAccount: -1,
	}, nil
}

func (v *vocdoniCLI) setHost(host string) error {
	u, err := url.Parse(host)
	if err != nil {
		return err
	}
	v.api.SetHostAddr(u)
	info, err := v.api.ChainInfo()
	if err != nil {
		return err
	}
	v.chainID = info.ID
	return nil
}

func (v *vocdoniCLI) setAuthToken(token string) error {
	t, err := uuid.Parse(token)
	if err != nil {
		return err
	}
	v.config.Token = &t
	v.api.SetAuthToken(&t)
	return nil
}

func (v *vocdoniCLI) useAccount(index int) error {
	if index >= len(v.config.Accounts) {
		return fmt.Errorf("account %d does not exist", index)
	}
	v.currentAccount = index
	return v.api.SetAccount(v.config.Accounts[index].PrivKey.String())
}

func (v *vocdoniCLI) getAccount(index int) (*Account, error) {
	if index >= len(v.config.Accounts) {
		return nil, fmt.Errorf("account %d does not exist", index)
	}
	return &v.config.Accounts[index], nil
}

func (v *vocdoniCLI) getCurrentAccount() *Account {
	return &v.config.Accounts[v.currentAccount]
}

func (v *vocdoniCLI) setAccount(key, memo string) error {
	if err := v.api.SetAccount(key); err != nil {
		return err
	}
	// check if already exist to update only memo
	key = util.TrimHex(key)
	for i, k := range v.config.Accounts {
		if k.PrivKey.String() == key {
			v.config.Accounts[i].Memo = memo
			v.currentAccount = i
			return nil
		}
	}
	keyb, err := hex.DecodeString(key)
	if err != nil {
		return err
	}

	signer := ethereum.SignKeys{}
	if err := signer.AddHexKey(key); err != nil {
		return err
	}

	v.config.Accounts = append(v.config.Accounts,
		Account{
			PrivKey:   keyb,
			Address:   signer.Address(),
			PublicKey: signer.PublicKey(),
			Memo:      memo,
		})
	v.currentAccount = len(v.config.Accounts) - 1
	return nil
}

// listAccounts list the memo notes of all stored accounts
func (v *vocdoniCLI) listAccounts() []string {
	accounts := []string{}
	for _, a := range v.config.Accounts {
		accounts = append(accounts, a.Memo)
	}
	return accounts
}

func (v *vocdoniCLI) transactionMined(txHash types.HexBytes) bool {
	_, err := v.api.TransactionReference(txHash)
	return err == nil
}

func (v *vocdoniCLI) waitForTransaction(txHash types.HexBytes) bool {
	startTime := time.Now()
	for time.Now().Before(startTime.Add(transactionConfirmationThreshold)) {
		if v.transactionMined(txHash) {
			return true
		}
		time.Sleep(3 * time.Second)
	}
	return false
}

func (v *vocdoniCLI) save() error {
	return v.config.Save(v.filepath)
}
