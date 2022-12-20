package apiclient

import (
	"encoding/json"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// Election returns the election details given its ID.
func (c *HTTPclient) Election(electionID types.HexBytes) (*api.Election, error) {
	resp, code, err := c.Request("GET", nil, "elections", electionID.String())
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	election := &api.Election{}
	if err = json.Unmarshal(resp, election); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return election, nil
}

// ElectionResults returns the election results given its ID.
func (c *HTTPclient) ElectionResults(electionID types.HexBytes) (*api.ElectionSummary, error) {
	resp, code, err := c.Request("GET", nil, "elections", electionID.String(), "results")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	electionSummary := &api.ElectionSummary{}
	if err = json.Unmarshal(resp, &electionSummary); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return electionSummary, nil
}

// NewElectionrRaw creates a new election given the protobuf Process message.
func (c *HTTPclient) NewElectionRaw(process *models.Process) (types.HexBytes, error) {
	// get the own account details
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch account info")
	}
	// build the transaction
	tx := models.Tx{
		Payload: &models.Tx_NewProcess{
			NewProcess: &models.NewProcessTx{
				Txtype:  models.TxType_NEW_PROCESS,
				Nonce:   acc.Nonce,
				Process: process,
			},
		},
	}
	txb, err := proto.Marshal(&tx)
	if err != nil {
		return nil, err
	}
	signedTxb, err := c.account.SignVocdoniTx(txb, c.chainID)
	if err != nil {
		return nil, err
	}
	stx, err := proto.Marshal(
		&models.SignedTx{
			Tx:        txb,
			Signature: signedTxb,
		})
	if err != nil {
		return nil, err
	}

	electionCreate := &api.ElectionCreate{
		TxPayload: stx,
	}
	resp, code, err := c.Request("POST", electionCreate, "elections")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusCodeOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	electionCreate = new(api.ElectionCreate)
	err = json.Unmarshal(resp, electionCreate)
	if err != nil {
		return nil, err
	}

	return electionCreate.ElectionID, nil
}

// NewElection creates a new election given the election details.
func (c *HTTPclient) NewElection(description *api.ElectionDescription) (types.HexBytes, error) {
	var err error
	if c.account == nil {
		return nil, fmt.Errorf("no account configured")
	}

	// Set startBlock, endBlock and blockCount
	if description.EndDate.Before(time.Now()) {
		return nil, fmt.Errorf("election end date cannot be in the past")
	}
	endBlock, err := c.DateToHeight(description.EndDate)
	if err != nil {
		return nil, fmt.Errorf("unable to estimate endDate block height: %w", err)
	}
	var startBlock, blockCount uint32
	// if start date is empty, do not attempt to parse it. Set startBlock to 0, starting the
	// election immediately. Otherwise, ensure the startBlock is in the future
	if !description.StartDate.IsZero() {
		if startBlock, err = c.DateToHeight(description.StartDate); err != nil {
			return nil, fmt.Errorf("unable to estimate startDate block height: %w", err)
		}
		info, err := c.ChainInfo() // get current Height
		if err != nil {
			return nil, err
		}
		blockCount = endBlock - *info.Height
	} else {
		blockCount = endBlock - startBlock
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
	metadata := api.ElectionMetadata{
		Description: description.Description,
		Media: api.ProcessMedia{
			Header:    description.Header,
			StreamURI: description.StreamURI,
		},
		Meta:      nil,
		Questions: []api.Question{},
		Results: api.ElectionResultsDetails{
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
		metaQuestion := api.Question{
			Choices:     []api.ChoiceMetadata{},
			Description: question.Description,
			Title:       question.Title,
		}
		for _, choice := range question.Choices {
			metaQuestion.Choices = append(metaQuestion.Choices, api.ChoiceMetadata{
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
		CostExponent:      10000,
	}

	// Census Origin
	censusOrigin, root, err := api.CensusTypeToOrigin(description.Census)
	if err != nil {
		return nil, err
	}

	// calculate metadata IPFS CID, we need it to reference the election description
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		return nil, fmt.Errorf("cannot format metadata: %w", err)
	}
	log.Debugf("election metadata: %s", string(metadataBytes))
	metadataURI := "ipfs://" + data.CalculateIPFSCIDv1json(metadataBytes)
	log.Debugf("metadataURI: %s", metadataURI)

	// get the own account details
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch account info")
	}

	// build the process transaction
	process := &models.Process{
		EntityId:     c.account.Address().Bytes(),
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
	log.Debugf("election transaction: %+v", log.FormatProto(process))

	tx := models.Tx{
		Payload: &models.Tx_NewProcess{
			NewProcess: &models.NewProcessTx{
				Txtype:  models.TxType_NEW_PROCESS,
				Nonce:   acc.Nonce,
				Process: process,
			},
		},
	}
	txb, err := proto.Marshal(&tx)
	if err != nil {
		return nil, err
	}
	signedTxb, err := c.account.SignVocdoniTx(txb, c.chainID)
	if err != nil {
		return nil, err
	}
	stx, err := proto.Marshal(
		&models.SignedTx{
			Tx:        txb,
			Signature: signedTxb,
		})
	if err != nil {
		return nil, err
	}

	electionCreate := &api.ElectionCreate{
		TxPayload: stx,
		Metadata:  metadataBytes,
	}
	resp, code, err := c.Request("POST", electionCreate, "elections")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusCodeOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	electionCreate = new(api.ElectionCreate)
	err = json.Unmarshal(resp, electionCreate)
	if err != nil {
		return nil, err
	}
	if electionCreate.MetadataURL == "" {
		return electionCreate.ElectionID, fmt.Errorf("metadata could not be published")
	}

	return electionCreate.ElectionID, nil
}

// SetElectionStatus configures the status of an election. The status can be one of the following:
// "READY", "ENDED", "CANCELED", "PAUSED". Not all transition status are valid.
// Returns the transaction hash.
func (c *HTTPclient) SetElectionStatus(electionID types.HexBytes, status string) (types.HexBytes, error) {
	if c.account == nil {
		return nil, fmt.Errorf("no account configured")
	}
	statusEnum, err := func() (models.ProcessStatus, error) {
		statusInt, ok := models.ProcessStatus_value[status]
		if !ok {
			return 0, fmt.Errorf("invalid status %s", status)
		}
		return models.ProcessStatus(statusInt), nil
	}()
	if err != nil {
		return nil, err
	}

	// get the own account details
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch account info: %s", acc.Address.String())
	}

	// build the set process transaction
	tx := models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_STATUS,
		ProcessId: electionID,
		Status:    &statusEnum,
		Nonce:     acc.Nonce,
	}
	txb, err := proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetProcess{
			SetProcess: &tx,
		}})
	if err != nil {
		return nil, err
	}
	signedTxb, err := c.account.SignVocdoniTx(txb, c.chainID)
	if err != nil {
		return nil, err
	}
	stx, err := proto.Marshal(
		&models.SignedTx{
			Tx:        txb,
			Signature: signedTxb,
		})
	if err != nil {
		return nil, err
	}

	// send the transaction
	resp, code, err := c.Request("POST", &api.Transaction{Payload: stx}, "chain", "transactions")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusCodeOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	txResp := new(api.Transaction)
	err = json.Unmarshal(resp, txResp)
	if err != nil {
		return nil, err
	}
	return txResp.Hash, nil
}

// ElectionVoteCount returns the number of registered votes for a given election.
func (c *HTTPclient) ElectionVoteCount(electionID types.HexBytes) (uint32, error) {
	resp, code, err := c.Request("GET", nil, "elections", electionID.String(), "votes", "count")
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusCodeOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	votes := new(struct {
		Count uint32 `json:"count"`
	})

	err = json.Unmarshal(resp, votes)
	if err != nil {
		return 0, err
	}
	return votes.Count, nil
}
