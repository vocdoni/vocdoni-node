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
	"go.vocdoni.io/dvote/httprouter/apirest"
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
	if err := a.Endpoint.RegisterMethod(
		"/wallet/add/{privateKey}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.walletAddHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/wallet/bootstrap",
		"GET",
		apirest.MethodAccessTypePublic,
		a.walletCreateHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/wallet/transfer/{dstAddress}/{amount}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.walletTransferHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/wallet/election",
		"POST",
		apirest.MethodAccessTypePublic,
		a.walletElectionHandler,
	); err != nil {
		return err
	}

	return nil
}

func (a *API) walletFromToken(authToken string) (*ethereum.SignKeys, error) {
	token, err := uuid.Parse(authToken)
	if err != nil {
		return nil, ErrCantParseBearerToken
	}

	// generate the index from the token hash
	index := ethereum.HashRaw(token[:])
	// using the index, get the encrypted private key
	privKeyEncrypted, err := a.db.Get(append([]byte(walletDBprefixEncryptedPrivateKey), index...))
	if err != nil {
		return nil, ErrWalletNotFound
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
	_, err := a.db.Get(append([]byte(walletDBprefixPrivateKeyHash), privKeyHash...))
	if err == db.ErrKeyNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrWalletPrivKeyAlreadyExists
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

	resp, err := a.sendTx(txData)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		Response: resp.Data.Bytes(),
		Hash:     resp.Hash.Bytes(),
		Code:     &resp.Code,
	}, nil
}

// walletAddHandler
//
//	@Summary		Add account
//	@Description	Add a new account to the local store. It returns a token used to manage this account on the future.
//	@Tags			Wallet
//	@Accept			json
//	@Produce		json
//	@Param			privateKey	path		string	true	"Private key to add"
//	@Success		200			{object}	object{token=string,address=string}
//	@Router			/wallet/add/{privateKey} [post]
func (a *API) walletAddHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// check private key format is correct and transform it to wallet and bytes
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
	// create the cipher with the new encryption key
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

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// walletCreateHandler
//
//	@Summary		Set wallet account
//	@Description	Set a new account. Needed the bearer token associated the account.
//	@Security		ApiKeyAuth
//	@Tags			Wallet
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	Transaction
//	@Router			/wallet/bootstrap [get]
func (a *API) walletCreateHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	wallet, err := a.walletFromToken(msg.AuthToken)
	if err != nil {
		return err
	}

	if acc, _ := a.vocapp.State.GetAccount(wallet.Address(), true); acc != nil {
		return ErrAccountAlreadyExists.With(wallet.AddressString())
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetAccount{
			SetAccount: &models.SetAccountTx{
				Txtype:        models.TxType_SET_ACCOUNT_INFO_URI,
				Nonce:         new(uint32),
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

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// walletTransferHandler
//
//	@Summary		Transfer tokens
//	@Description	Transfer balance to another account. Needed the bearer token associated the account.
//	@Security		ApiKeyAuth
//	@Tags			Wallet
//	@Accept			json
//	@Produce		json
//	@Param			dstAddress	path		string	true	"Destination address"
//	@Param			amount		path		string	true	"Amount of tokens to transfer"
//	@Success		200			{object}	Transaction
//	@Router			/wallet/transfer/{dstAddress}/{amount} [get]
func (a *API) walletTransferHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	wallet, err := a.walletFromToken(msg.AuthToken)
	if err != nil {
		return err
	}

	acc, err := a.vocapp.State.GetAccount(wallet.Address(), true)
	if err != nil {
		return err
	}
	if len(util.TrimHex(ctx.URLParam("dstAddress"))) != common.AddressLength*2 {
		return ErrDstAddressMalformed
	}
	dst := common.HexToAddress(ctx.URLParam("dstAddress"))
	if dstAcc, err := a.vocapp.State.GetAccount(dst, true); dstAcc == nil || err != nil {
		return ErrDstAccountUnknown
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

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// walletElectionHandler
//
//		@Summary		Create election for wallet
//		@Description	Creates an election. Requires the bearer token of the account you want to create the election.
//		@Security		ApiKeyAuth
//	 	@Param 			Auth-Token header string true "Bearer "
//		@Tags			Wallet
//		@Accept			json
//		@Produce		json
//		@Param			description	body		ElectionDescription	true	"Election description"
//		@Success		200			{object}	Transaction
//		@Router			/wallet/election [post]
func (a *API) walletElectionHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
		return ErrCantParseDataAsJSON.WithErr(err)
	}

	// Set startBlock and endBlock
	var startTime uint32

	if !description.StartDate.IsZero() {
		startTime = uint32(description.StartDate.Unix())
	} else {
		description.StartDate = time.Now().Add(time.Minute * 10)
	}

	if description.EndDate.Before(time.Now()) {
		return ErrElectionEndDateInThePast
	}
	if description.EndDate.Before(description.StartDate) {
		return ErrElectionEndDateBeforeStart
	}
	endTime := uint32(description.EndDate.Unix())

	// Calculate block count
	duration := endTime - startTime
	if startTime == 0 {
		duration = endTime - uint32(time.Now().Unix())
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
		Type: ElectionProperties{
			Name: "single-choice-multiquestion",
		},
		Title:   description.Title,
		Version: "1.0",
	}

	maxChoiceValue := 0
	for _, question := range description.Questions {
		maxChoiceValue = max(maxChoiceValue, len(question.Choices))
		metaQuestion := Question{
			Choices:     []ChoiceMetadata{},
			Description: question.Description,
			Title:       question.Title,
		}

		metaQuestion.Choices = append(metaQuestion.Choices, question.Choices...)
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
		return ErrCantMarshalMetadata.WithErr(err)
	}
	storageCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	metadataURI, err := a.storage.Publish(storageCtx, metadataBytes)
	cancel()
	if err != nil {
		return ErrCantPublishMetadata.WithErr(err)
	}
	metadataURI = "ipfs://" + metadataURI

	// Build the process transaction
	process := &models.Process{
		EntityId:     wallet.Address().Bytes(),
		StartTime:    startTime,
		Duration:     duration,
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
		},
	},
	); err != nil {
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
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}
