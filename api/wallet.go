package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	WalletHandler = "wallet"

	walletDBprefixPrivateKeyHash      = "wph_"
	walletDBprefixEncryptedPrivateKey = "wpk_"
	privateKeyHashPrefix              = "vocdoniPrivateKey"
)

func (a *API) enableWalletHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/wallet/add/{privateKey}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.walletAddHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/wallet/bootstrap",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.walletCreateHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/wallet/transfer/{dstAddress}/{amount}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.walletTransferHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/wallet/election",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		a.walletElectionHandler,
	); err != nil {
		return err
	}

	return nil
}

func (a *API) walletFromToken(authToken string) (*ethereum.SignKeys, error) {
	token, err := uuid.Parse(authToken)
	if err != nil {
		return nil, fmt.Errorf("error parsing bearer token")
	}

	// generate the index from the token hash
	index := ethereum.HashRaw(token[:])
	// using the index, get the encrypted private key
	privKeyEncrypted, err := a.db.ReadTx().Get(append([]byte(walletDBprefixEncryptedPrivateKey), index...))
	if err != nil {
		return nil, fmt.Errorf("wallet not found")
	}
	// generate the decryption key
	encryptKey := ethereum.HashRaw(append([]byte(privateKeyHashPrefix), token[:]...))
	cipher, err := nacl.DecodePrivate(fmt.Sprintf("%x", encryptKey))
	if err != nil {
		return nil, err
	}
	// decrypt the private key
	privKey, err := cipher.Decrypt(privKeyEncrypted)
	if err != nil {
		return nil, err
	}
	// return the wallet
	wallet := ethereum.SignKeys{}
	return &wallet, wallet.AddHexKey(fmt.Sprintf("%x", privKey))
}

func (a *API) walletCheckKeyExists(privKey []byte) error {
	privKeyHash := ethereum.HashRaw(privKey)
	_, err := a.db.ReadTx().Get(append([]byte(walletDBprefixPrivateKeyHash), privKeyHash...))
	if err == db.ErrKeyNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("key already exist")
}

func (a *API) walletSignAndSendTx(stx *models.SignedTx, wallet *ethereum.SignKeys) (*Transaction, error) {
	var err error
	if stx.Signature, err = wallet.SignVocdoniTx(stx.Tx, a.vocapp.ChainID()); err != nil {
		return nil, err
	}
	txData, err := proto.Marshal(stx)
	if err != nil {
		return nil, err
	}

	resp, err := a.vocapp.SendTx(txData)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("no reply from vochain")
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("%s", string(resp.Data))
	}

	return &Transaction{
		Response: resp.Data.Bytes(),
		Hash:     resp.Hash.Bytes(),
		Code:     &resp.Code,
	}, nil
}

// /wallet/add/{privateKey}
// add a new account to the local store
func (a *API) walletAddHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	// check private key format is correct and transorm it to wallet and bytes
	wallet := ethereum.SignKeys{}
	if err := wallet.AddHexKey(ctx.URLParam("privateKey")); err != nil {
		return err
	}
	privKeyBytes, err := hex.DecodeString(ctx.URLParam("privateKey"))
	if err != nil {
		return err
	}
	// check if private key is already registered
	if err := a.walletCheckKeyExists(privKeyBytes); err != nil {
		return err
	}
	// create the UUID token that will be used as encryption key and identifier
	token := uuid.New()
	// the encryption key is the hash of the token with a common prefix
	encryptKey := ethereum.HashRaw(append([]byte(privateKeyHashPrefix), token[:]...))
	// create the cipher with the new ecryption key
	cipher, err := nacl.DecodePrivate(fmt.Sprintf("%x", encryptKey))
	if err != nil {
		return err
	}
	// encrypt the private key
	encryptedPrivKey, err := cipher.Encrypt(privKeyBytes, nil)
	if err != nil {
		return err
	}
	// generate the database index (hash of token), used for finding the key afterwards
	index := ethereum.HashRaw(token[:])
	wtx := a.db.WriteTx()
	// store the encrypted private key
	if err := wtx.Set(append([]byte(walletDBprefixEncryptedPrivateKey), index...), encryptedPrivKey); err != nil {
		return err
	}
	// store the private key hash, so we can check afterwards if the private key already exist
	privKeyHash := ethereum.HashRaw(privKeyBytes)
	if err := wtx.Set(append([]byte(walletDBprefixPrivateKeyHash), privKeyHash...), nil); err != nil {
		return err
	}
	if err := wtx.Commit(); err != nil {
		return err
	}

	var data []byte
	if data, err = json.Marshal(Account{
		Token:   &token,
		Address: wallet.Address().Bytes(),
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /wallet/bootstrap
// set a new account
func (a *API) walletCreateHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	wallet, err := a.walletFromToken(msg.AuthToken)
	if err != nil {
		return err
	}

	if acc, _ := a.vocapp.State.GetAccount(wallet.Address(), true); acc != nil {
		return fmt.Errorf("account %s already exist", wallet.AddressString())
	}

	stx := models.SignedTx{}
	infoURI := string("none")
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetAccount{
			SetAccount: &models.SetAccountTx{
				Txtype:        models.TxType_SET_ACCOUNT_INFO_URI,
				Nonce:         new(uint32),
				InfoURI:       &infoURI,
				Account:       wallet.Address().Bytes(),
				FaucetPackage: nil,
			},
		}})
	if err != nil {
		return err
	}

	tx, err := a.walletSignAndSendTx(&stx, wallet)
	if err != nil {
		return err
	}
	tx.Address = wallet.Address().Bytes()
	var data []byte
	if data, err = json.Marshal(tx); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /wallet/transfer/{dstAddress}/{amount}
// transfer balance to another account
func (a *API) walletTransferHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	wallet, err := a.walletFromToken(msg.AuthToken)
	if err != nil {
		return err
	}

	acc, err := a.vocapp.State.GetAccount(wallet.Address(), true)
	if err != nil {
		return err
	}
	if len(util.TrimHex(ctx.URLParam("dstAddress"))) != common.AddressLength*2 {
		return fmt.Errorf("destination address malformed")
	}
	dst := common.HexToAddress(ctx.URLParam("dstAddress"))
	if dstAcc, err := a.vocapp.State.GetAccount(dst, true); dstAcc == nil || err != nil {
		return fmt.Errorf("destination account is unknown")
	}
	amount, err := strconv.ParseUint(ctx.URLParam("amount"), 10, 64)
	if err != nil {
		return err
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SendTokens{
			SendTokens: &models.SendTokensTx{
				Txtype: models.TxType_SEND_TOKENS,
				Nonce:  acc.GetNonce(),
				From:   wallet.Address().Bytes(),
				To:     dst.Bytes(),
				Value:  amount,
			},
		}})
	if err != nil {
		return err
	}
	tx, err := a.walletSignAndSendTx(&stx, wallet)
	if err != nil {
		return err
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /wallet/election POST
// creates an election
func (a *API) walletElectionHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	wallet, err := a.walletFromToken(msg.AuthToken)
	if err != nil {
		return err
	}
	acc, err := a.vocapp.State.GetAccount(wallet.Address(), true)
	if err != nil {
		return err
	}

	description := &ElectionDescription{}
	if err := json.Unmarshal(msg.Data, description); err != nil {
		return fmt.Errorf("could not unmarshal JSON: %w", err)
	}

	// Set startBlock and endBlock
	var startBlock uint32
	// if start date is empty, do not attempt to parse it. Set startBlock to 0, starting the
	// election immediately. Otherwise, ensure the startBlock is in the future
	if !description.StartDate.IsZero() {
		if startBlock, err = a.vocinfo.EstimateBlockHeight(description.StartDate); err != nil {
			return fmt.Errorf("unable to estimate startDate block height: %w", err)
		}
	} else {
		description.StartDate = time.Now().Add(time.Minute * 10)
	}

	if description.EndDate.Before(time.Now()) {
		return fmt.Errorf("election end date cannot be in the past")
	}
	endBlock, err := a.vocinfo.EstimateBlockHeight(description.EndDate)
	if err != nil {
		return fmt.Errorf("unable to estimate endDate block height: %w", err)
	}
	if description.EndDate.Before(description.StartDate) {
		return fmt.Errorf("end date must be after start date")
	}

	// Calculate block count
	blockCount := endBlock - startBlock
	if startBlock == 0 {
		// If startBlock is set to 0 (process starts asap), set the blockcount to the desired
		// end block, minus the expected start block of the process
		blockCount = blockCount - uint32(a.vocinfo.Height()) + 3
	}

	// Set the envelope and process models
	envelopeType := &models.EnvelopeType{
		Serial:         false,
		Anonymous:      description.ElectionType.Anonymous,
		EncryptedVotes: description.ElectionType.SecretUntilTheEnd,
		UniqueValues:   description.VoteType.UniqueChoices,
		CostFromWeight: description.VoteType.CostFromWeight,
	}

	processMode := &models.ProcessMode{
		AutoStart:     description.ElectionType.Autostart,
		Interruptible: description.ElectionType.Interruptible,
		DynamicCensus: description.ElectionType.DynamicCensus,
	}

	// Prepare the election metadata information
	metadata := ElectionMetadata{
		Description: description.Description,
		Media: ProcessMedia{
			Header:    description.Header,
			StreamURI: description.StreamURI,
		},
		Meta:      nil,
		Questions: []Question{},
		Results: ElectionResultsDetails{
			Aggregation: "discrete-values",
			Display:     "multiple-choice",
		},
		Title:   description.Title,
		Version: "1.0",
	}

	maxChoiceValue := 0
	for _, question := range description.Questions {
		if len(question.Choices) > maxChoiceValue {
			maxChoiceValue = len(question.Choices)
		}
		metaQuestion := Question{
			Choices:     []ChoiceMetadata{},
			Description: question.Description,
			Title:       question.Title,
		}
		for _, choice := range question.Choices {
			metaQuestion.Choices = append(metaQuestion.Choices, ChoiceMetadata{
				Title: choice.Title,
				Value: choice.Value,
			})
		}
		metadata.Questions = append(metadata.Questions, metaQuestion)
	}

	// TODO: respect maxCount and maxValue if specified
	voteOptions := &models.ProcessVoteOptions{
		MaxCount:          uint32(len(description.Questions)),
		MaxValue:          uint32(maxChoiceValue),
		MaxVoteOverwrites: uint32(description.VoteType.MaxVoteOverwrites),
		MaxTotalCost:      uint32(len(description.Questions) * maxChoiceValue),
		CostExponent:      1,
	}

	// Census Origin
	censusOrigin, root, err := CensusTypeToOrigin(description.Census)
	if err != nil {
		return err
	}

	// Publish the metadata to IPFS
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		return fmt.Errorf("cannot format metadata: %w", err)
	}
	storageCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	metadataURI, err := a.storage.Publish(storageCtx, metadataBytes)
	cancel()
	if err != nil {
		return fmt.Errorf("cannot publish metadata file: %w", err)
	}
	metadataURI = "ipfs://" + metadataURI

	// Build the process transaction
	process := &models.Process{
		EntityId:     wallet.Address().Bytes(),
		StartBlock:   startBlock,
		BlockCount:   blockCount,
		CensusRoot:   root,
		CensusURI:    &description.Census.URL,
		Status:       models.ProcessStatus_READY,
		EnvelopeType: envelopeType,
		Mode:         processMode,
		VoteOptions:  voteOptions,
		CensusOrigin: censusOrigin,
		Metadata:     &metadataURI,
	}

	log.Debugf(log.FormatProto(process))

	stx := models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_NewProcess{
			NewProcess: &models.NewProcessTx{
				Process: process,
				Nonce:   acc.GetNonce(),
				Txtype:  models.TxType_NEW_PROCESS,
			},
		}}); err != nil {
		return err
	}
	tx, err := a.walletSignAndSendTx(&stx, wallet)
	if err != nil {
		return err
	}

	// NewProcessTx execution returns the deterministic electionID,
	// so we add it to the output response.
	tx.ProcessID = tx.Response
	tx.Response = nil
	tx.Code = nil
	data, err := json.Marshal(tx)
	if err != nil {
		log.Error(err)
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
