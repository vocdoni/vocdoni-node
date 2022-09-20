package urlapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const ElectionHandler = "election"

func (u *URLAPI) enableElectionHandlers() error {
	if err := u.api.RegisterMethod(
		"/election/list/{organizationID}/status/{status}/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionListHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/election/list/{organizationID}/status/{status}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionListHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/election/list/{organizationID}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionListHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/election/list/{organizationID}/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionListHandler,
	); err != nil {
		return err
	}

	if err := u.api.RegisterMethod(
		"/election/{electionID}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionHandler,
	); err != nil {
		return err
	}

	if err := u.api.RegisterMethod(
		"/election/{electionID}/count",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionCountHandler,
	); err != nil {
		return err
	}

	if err := u.api.RegisterMethod(
		"/election/{electionID}/keys",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionKeysHandler,
	); err != nil {
		return err
	}

	if err := u.api.RegisterMethod(
		"/election/{electionID}/votes",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.electionVotesHandler,
	); err != nil {
		return err
	}

	if err := u.api.RegisterMethod(
		"/election/prepare",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		u.electionCreateHandler,
	); err != nil {
		return err
	}

	return nil
}

// /election/<organizationID>/list/<status>
// list the elections of an organization.
func (u *URLAPI) electionListHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	organizationID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("organizationID")))
	if err != nil || organizationID == nil {
		return fmt.Errorf("organizationID (%q) cannot be decoded", ctx.URLParam("organizationID"))
	}

	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return fmt.Errorf("cannot parse page number")
		}
	}
	page = page * MaxPageSize

	var pids [][]byte
	switch ctx.URLParam("status") {
	case "active":
		pids, err = u.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "READY", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	case "paused":
		pids, err = u.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "PAUSED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	case "ended":
		pids, err = u.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "RESULTS", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
		pids2, err := u.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "ENDED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
		pids = append(pids, pids2...)
	case "":
		pids, err = u.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	default:
		return fmt.Errorf("missing status parameter or unknown")
	}

	elections, err := u.getProcessSummaryList(pids...)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&Organization{
		OrganizationID: types.HexBytes(organizationID),
		Elections:      elections,
	})
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	if err = ctx.Send(data, bearerstdapi.HTTPstatusCodeOK); err != nil {
		log.Warn(err)
	}
	return nil
}

// /election/electionID/<electionID>
// get election information
func (u *URLAPI) electionHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil {
		return fmt.Errorf("electionID (%s) cannot be decoded", ctx.URLParam("electionID"))
	}
	proc, err := u.scrutinizer.ProcessInfo(electionID)
	if err != nil {
		return fmt.Errorf("cannot fetch electionID %x: %w", electionID, err)
	}

	var jElection Election
	jElection.Status = models.ProcessStatus_name[proc.Status]
	jElection.Type = u.formatElectionType(proc.Envelope)
	count, err := u.scrutinizer.GetEnvelopeHeight(electionID)
	if err != nil {
		return fmt.Errorf("cannot get envelope height: %w", err)
	}
	jElection.VoteCount = &count
	if proc.HaveResults {
		results, err := u.scrutinizer.GetResults(electionID)
		if err != nil {
			return fmt.Errorf("cannot get envelope height: %w", err)
		}
		for _, r := range scrutinizer.GetFriendlyResults(results.Votes) {
			jElection.Results = append(jElection.Results, Result{Value: r})
		}
	}

	data, err := json.Marshal(jElection)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /election/<organizationID>/count
func (u *URLAPI) electionCountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	organizationID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("organizationID")))
	if err != nil || organizationID == nil {
		return fmt.Errorf("organizationID (%q) cannot be decoded", ctx.URLParam("organizationID"))
	}
	count := u.scrutinizer.ProcessCount(organizationID)
	election := Election{
		Count: &count,
	}
	data, err := json.Marshal(election)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /election/<electionID>/keys
// returns the list of public/private encryption keys
func (u *URLAPI) electionKeysHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return fmt.Errorf("electionID (%q) cannot be decoded", ctx.URLParam("electionID"))
	}

	process, err := u.vocapp.State.Process(electionID, true)
	if err != nil {
		return fmt.Errorf("cannot get election keys: %w", err)
	}

	election := Election{}
	for idx, pubk := range process.EncryptionPublicKeys {
		if len(pubk) > 0 {
			pk, err := hex.DecodeString(pubk)
			if err != nil {
				panic(err)
			}
			election.PublicKeys = append(election.PublicKeys, Key{Index: idx, Key: pk})
		}
	}
	for idx, privk := range process.EncryptionPrivateKeys {
		if len(privk) > 0 {
			pk, err := hex.DecodeString(privk)
			if err != nil {
				panic(err)
			}
			election.PrivateKeys = append(election.PrivateKeys, Key{Index: idx, Key: pk})
		}
	}

	data, err := json.Marshal(election)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// election/<electionID>/votes
// returns the list of voteIDs for an election (paginated)
func (u *URLAPI) electionVotesHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	// TODO
	return nil
}

// election/prepare
func (u *URLAPI) electionCreateHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	var err error
	description := &ElectionDescription{}
	if err := json.Unmarshal(msg.Data, description); err != nil {
		return fmt.Errorf("could not unmarshal JSON: %w", err)
	}

	// Set startBlock and endBlock
	var startBlock uint32
	startDate := time.Now()
	// if start date is empty, do not attempt to parse it. Set startBlock to 0, starting the
	// election immediately. Otherwise, ensure the startBlock is in the future
	if !description.StartDate.IsZero() {
		if startBlock, err = u.vocinfo.EstimateBlockHeight(startDate); err != nil {
			return fmt.Errorf("unable to estimate startDate block height: %w", err)
		}
	}

	if description.EndDate.Before(time.Now()) {
		return fmt.Errorf("election end date cannot be in the past")
	}
	endBlock, err := u.vocinfo.EstimateBlockHeight(description.EndDate)
	if err != nil {
		return fmt.Errorf("unable to estimate endDate block height: %w", err)
	}
	if description.EndDate.Before(startDate) {
		return fmt.Errorf("end date must be after start date")
	}

	// Calculate block count
	blockCount := endBlock - startBlock
	if startBlock == 0 {
		// If startBlock is set to 0 (process starts asap), set the blockcount to the desired
		// end block, minus the expected start block of the process
		blockCount = blockCount - uint32(u.vocinfo.Height()) + 3
	}

	// Set the envelope and process models
	envelopeType := &models.EnvelopeType{
		Serial:         false,
		Anonymous:      description.ElectionMode.Anonymous,
		EncryptedVotes: description.ElectionMode.SecretUntilTheEnd,
		UniqueValues:   description.VoteType.UniqueChoices,
		CostFromWeight: description.VoteType.CostFromWeight,
	}

	processMode := &models.ProcessMode{
		AutoStart:     description.ElectionMode.Autostart,
		Interruptible: description.ElectionMode.Interruptible,
		DynamicCensus: description.ElectionMode.DynamicCensus,
	}

	// Prepare the election metadata information
	metadata := ElectionMetadata{
		Description: map[string]string{"default": description.Description},
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
		Title:   map[string]string{"default": description.Title},
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
	censusOrigin, root, err := censusTypeToOrigin(description.Census)
	if err != nil {
		return err
	}

	// Publish the metadata to IPFS
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		return fmt.Errorf("cannot format metadata: %w", err)
	}
	storageCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	metadataURI, err := u.storage.Publish(storageCtx, metadataBytes)
	cancel()
	if err != nil {
		return fmt.Errorf("cannot publish metadata file: %w", err)
	}

	// Build the process transaction
	process := &models.Process{
		EntityId:     description.OrganizationID,
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

	processData, err := proto.Marshal(process)
	if err != nil {
		return fmt.Errorf("cannot marshal election transaction: %w", err)
	}
	tx := &Transaction{Payload: processData}
	data, err := json.Marshal(tx)
	if err != nil {
		log.Error(err)
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
