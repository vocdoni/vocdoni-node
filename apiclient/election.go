package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/processid"
	"go.vocdoni.io/dvote/vochain/state/electionprice"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// Votes returns the list of votes for a given election.
// If untilPage is set different from 0, it will fetch votes until that page.
func (c *HTTPclient) Votes(electionID types.HexBytes, untilPage int) ([]api.Vote, error) {
	fullVotes := []api.Vote{}
	page := 0
	for {
		resp, code, err := c.Request(HTTPGET, nil, "elections", electionID.String(), "votes", "page", fmt.Sprintf("%d", page))
		if err != nil {
			return nil, err
		}
		if code != apirest.HTTPstatusOK {
			return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
		}
		// Unmarshal vote list
		voteList := api.VotesList{}
		if err := json.Unmarshal(resp, &voteList); err != nil {
			return nil, err
		}
		if len(voteList.Votes) == 0 || (untilPage > 0 && page >= untilPage) {
			// no more votes
			break
		}

		for _, v := range voteList.Votes {
			// For each vote, fetch the vote details
			resp, code, err := c.Request(HTTPGET, nil, "votes", v.VoteID.String())
			if err != nil {
				return nil, err
			}
			if code != apirest.HTTPstatusOK {
				return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
			}
			vote := api.Vote{}
			if err := json.Unmarshal(resp, &vote); err != nil {
				return nil, err
			}
			fullVotes = append(fullVotes, vote)
		}

		page++
	}
	return fullVotes, nil
}

// Election returns the election details given its ID.
func (c *HTTPclient) Election(electionID types.HexBytes) (*api.Election, error) {
	if electionID == nil {
		return nil, fmt.Errorf("passed electionID is nil")
	}
	resp, code, err := c.Request(HTTPGET, nil, "elections", electionID.String())
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	election := &api.Election{}
	if err = json.Unmarshal(resp, election); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return election, nil
}

// NewElectionRaw creates a new election given the protobuf Process message
// and returns the ElectionID
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
	resp, code, err := c.Request(HTTPPOST, electionCreate, "elections")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	electionCreate = new(api.ElectionCreate)
	if err := json.Unmarshal(resp, electionCreate); err != nil {
		return nil, err
	}

	return electionCreate.ElectionID, nil
}

// NewElection creates a new election given the election details
// and returns the ElectionID. If wait is true, it will wait until the election is created.
func (c *HTTPclient) NewElection(description *api.ElectionDescription, wait bool) (types.HexBytes, error) {
	if c.account == nil {
		return nil, fmt.Errorf("no account configured")
	}

	// Set start and end dates
	if description.EndDate.Before(time.Now()) {
		return nil, fmt.Errorf("election end date cannot be in the past")
	}
	var startTime, duration uint32
	if description.StartDate.IsZero() {
		// if start date is empty, start the election immediately.
		startTime = 0
		duration = uint32(description.EndDate.Unix() - time.Now().Unix())
	} else {
		startTime = uint32(description.StartDate.Unix())
		duration = uint32(description.EndDate.Unix() - description.StartDate.Unix())
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
		Type: api.ElectionProperties{
			Name: "single-choice-multiquestion",
		},
		Title:   description.Title,
		Version: "1.0",
	}

	maxChoiceValue := 0
	for _, question := range description.Questions {
		maxChoiceValue = max(maxChoiceValue, len(question.Choices))
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
		CostExponent:      1,
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
	metadataURI := "ipfs://" + ipfs.CalculateCIDv1json(metadataBytes)
	log.Debugf("metadataURI: %s", metadataURI)

	// get the own account details
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch account info")
	}

	// build the process transaction
	process := &models.Process{
		EntityId:      c.account.Address().Bytes(),
		Duration:      duration,
		StartTime:     startTime,
		CensusRoot:    root,
		CensusURI:     &description.Census.URL,
		Status:        models.ProcessStatus_READY,
		EnvelopeType:  envelopeType,
		Mode:          processMode,
		VoteOptions:   voteOptions,
		CensusOrigin:  censusOrigin,
		Metadata:      &metadataURI,
		MaxCensusSize: description.Census.Size,
		TempSIKs:      &description.TempSIKs,
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
	resp, code, err := c.Request(HTTPPOST, electionCreate, "elections")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	electionCreate = new(api.ElectionCreate)
	if err := json.Unmarshal(resp, electionCreate); err != nil {
		return nil, err
	}
	if electionCreate.MetadataURL == "" {
		log.Warnf("metadata could not be published")
	}
	if wait {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
		defer cancel()
		if _, err := c.WaitUntilElectionCreated(ctx, electionCreate.ElectionID); err != nil {
			return nil, err
		}
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
		return nil, fmt.Errorf("could not fetch account info: %s", acc.Address)
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
		},
	})
	if err != nil {
		return nil, err
	}
	hash, _, err := c.SignAndSendTx(txb)
	return hash, err
}

// SetElectionCensusSize sets the new census size of an election.
func (c *HTTPclient) SetElectionCensusSize(electionID types.HexBytes, newSize uint64) (types.HexBytes, error) {
	if c.account == nil {
		return nil, fmt.Errorf("no account configured")
	}
	// get the own account details
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch account info: %s", acc.Address)
	}

	// build the set process transaction
	tx := models.SetProcessTx{
		Txtype:     models.TxType_SET_PROCESS_CENSUS,
		ProcessId:  electionID,
		CensusSize: &newSize,
		Nonce:      acc.Nonce,
	}
	txb, err := proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetProcess{
			SetProcess: &tx,
		},
	})
	if err != nil {
		return nil, err
	}
	hash, _, err := c.SignAndSendTx(txb)
	return hash, err
}

// SetElectionDuration modify the duration of an election (in seconds).
func (c *HTTPclient) SetElectionDuration(electionID types.HexBytes, newDuration uint32) (types.HexBytes, error) {
	if c.account == nil {
		return nil, fmt.Errorf("no account configured")
	}
	// get the own account details
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch account info: %s", acc.Address)
	}

	// build the set process transaction
	tx := models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_DURATION,
		ProcessId: electionID,
		Duration:  &newDuration,
		Nonce:     acc.Nonce,
	}
	txb, err := proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetProcess{
			SetProcess: &tx,
		},
	})
	if err != nil {
		return nil, err
	}
	hash, _, err := c.SignAndSendTx(txb)
	return hash, err
}

// SetElectionCensus updates the census of an election. Root, URI and size can be updated.
func (c *HTTPclient) SetElectionCensus(electionID types.HexBytes, census api.ElectionCensus) (types.HexBytes, error) {
	if c.account == nil {
		return nil, fmt.Errorf("no account configured")
	}

	if _, ok := models.CensusOrigin_value[census.CensusOrigin]; !ok {
		return nil, fmt.Errorf("invalid census origin %s", census.CensusOrigin)
	}

	// get the own account details
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch account info: %s", acc.Address)
	}

	tx := &models.SetProcessTx{
		Txtype:     models.TxType_SET_PROCESS_CENSUS,
		Nonce:      acc.Nonce,
		ProcessId:  electionID,
		CensusRoot: census.CensusRoot,
		CensusURI:  &census.CensusURL,
		CensusSize: func() *uint64 {
			if census.MaxCensusSize > 0 {
				return &census.MaxCensusSize
			}
			return nil
		}(),
	}

	txb, err := proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetProcess{SetProcess: tx},
	})
	if err != nil {
		return nil, err
	}

	hash, _, err := c.SignAndSendTx(txb)
	return hash, err
}

// ElectionVoteCount returns the number of registered votes for a given election.
func (c *HTTPclient) ElectionVoteCount(electionID types.HexBytes) (uint32, error) {
	resp, code, err := c.Request(HTTPGET, nil, "elections", electionID.String(), "votes", "count")
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	votes := new(struct {
		Count uint32 `json:"count"`
	})

	if err := json.Unmarshal(resp, votes); err != nil {
		return 0, err
	}
	return votes.Count, nil
}

// ElectionResults returns the election results given its ID.
func (c *HTTPclient) ElectionResults(electionID types.HexBytes) (*api.ElectionResults, error) {
	resp, code, err := c.Request(HTTPGET, nil, "elections", electionID.String(), "scrutiny")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	electionResults := &api.ElectionResults{}
	if err = json.Unmarshal(resp, &electionResults); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return electionResults, nil
}

// ElectionFilterPaginated returns a list of elections filtered by the given parameters.
// POST /elections/filter/page/<page>
// Returns a list of elections filtered by the given parameters.
func (c *HTTPclient) ElectionFilterPaginated(organizationID types.HexBytes, electionID types.HexBytes,
	status models.ProcessStatus, withResults bool, page int,
) (*[]api.ElectionSummary, error) {
	body := struct {
		OrganizationID types.HexBytes `json:"organizationId,omitempty"`
		ElectionID     types.HexBytes `json:"electionId,omitempty"`
		WithResults    bool           `json:"withResults,omitempty"`
		Status         string         `json:"status,omitempty"`
	}{
		OrganizationID: organizationID,
		ElectionID:     electionID,
		WithResults:    withResults,
		Status:         status.String(),
	}
	resp, code, err := c.Request(HTTPPOST, body, "elections", "filter", "page", strconv.Itoa(page))
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	var elections []api.ElectionSummary
	if err := json.Unmarshal(resp, &elections); err != nil {
		return nil, err
	}
	return &elections, nil
}

// ElectionKeys fetches the encryption keys for an election.
// Note that only elections that are SecretUntilTheEnd will return keys
func (c *HTTPclient) ElectionKeys(electionID types.HexBytes) (*api.ElectionKeys, error) {
	resp, code, err := c.Request(HTTPGET, nil, "elections", electionID.String(), "keys")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	electionKeys := &api.ElectionKeys{}
	if err = json.Unmarshal(resp, &electionKeys); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %w", err)
	}
	return electionKeys, nil
}

// ElectionPrice returns the price of an election.
func (c *HTTPclient) ElectionPrice(election *api.ElectionDescription) (uint64, error) {
	endBlock, err := c.DateToHeight(election.EndDate)
	if err != nil {
		return 0, err
	}
	startBlock := uint32(0)
	if election.StartDate.IsZero() {
		info, err := c.ChainInfo()
		if err != nil {
			return 0, err
		}
		startBlock = info.Height
	} else {
		startBlock, err = c.DateToHeight(election.StartDate)
		if err != nil {
			return 0, err
		}
	}
	params := electionprice.ElectionParameters{
		MaxCensusSize:    election.Census.Size,
		ElectionDuration: endBlock - startBlock,
		EncryptedVotes:   election.ElectionType.SecretUntilTheEnd,
		AnonymousVotes:   election.ElectionType.Anonymous,
		MaxVoteOverwrite: uint32(election.VoteType.MaxVoteOverwrites),
	}
	resp, code, err := c.Request(HTTPPOST, params, "elections", "price")
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	price := struct {
		Price uint64 `json:"price"`
	}{}
	if err := json.Unmarshal(resp, &price); err != nil {
		return 0, err
	}
	return price.Price, nil
}

// NextElectionID gets the next election ID for an organization.
// POST /elections/id
func (c *HTTPclient) NextElectionID(organizationID types.HexBytes, censusOrigin int32, envelopeType *models.EnvelopeType) (string, error) {
	body := &api.BuildElectionID{
		Delta:          processid.BuildNextProcessID,
		OrganizationID: organizationID,
		CensusOrigin:   censusOrigin,
		EnvelopeType: struct {
			Serial         bool `json:"serial"`
			Anonymous      bool `json:"anonymous"`
			EncryptedVotes bool `json:"encryptedVotes"`
			UniqueValues   bool `json:"uniqueValues"`
			CostFromWeight bool `json:"costFromWeight"`
		}{
			Serial:         envelopeType.Serial,
			Anonymous:      envelopeType.Anonymous,
			EncryptedVotes: envelopeType.EncryptedVotes,
			UniqueValues:   envelopeType.UniqueValues,
			CostFromWeight: envelopeType.CostFromWeight,
		},
	}
	resp, code, err := c.Request(HTTPPOST, body, "elections", "id")
	if err != nil {
		return "", err
	}
	if code != apirest.HTTPstatusOK {
		return "", fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}

	var electionID string
	if err := json.Unmarshal(resp, &electionID); err != nil {
		return "", err
	}
	return electionID, nil
}
