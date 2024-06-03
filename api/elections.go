package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/processid"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/state/electionprice"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	ElectionHandler     = "elections"
	MaxOffchainFileSize = 1024 * 1024 * 1 // 1MB
)

func (a *API) enableElectionHandlers() error {
	if err := a.Endpoint.RegisterMethod(
		"/elections/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionFullListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/elections/{electionID}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/elections/{electionID}/keys",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionKeysHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/elections/{electionID}/votes/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionVotesCountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/elections/{electionID}/votes/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionVotesHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/elections/{electionID}/scrutiny",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionScrutinyHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/elections",
		"POST",
		apirest.MethodAccessTypePublic,
		a.electionCreateHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/elections/price",
		"POST",
		apirest.MethodAccessTypePublic,
		a.electionPriceHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/files/cid",
		"POST",
		apirest.MethodAccessTypePublic,
		a.computeCidHandler,
	); err != nil {
		return err
	}

	if err := a.Endpoint.RegisterMethod(
		"/elections/filter/page/{page}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.electionFilterPaginatedHandler,
	); err != nil {
		return err
	}

	if err := a.Endpoint.RegisterMethod(
		"/elections/id",
		"POST",
		apirest.MethodAccessTypePublic,
		a.nextElectionIDHandler,
	); err != nil {
		return err
	}

	return nil
}

// electionFullListHandler
//
//	@Summary		List elections
//	@Description	Get a list of elections summaries.
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			page	path		number	true	"Page "
//	@Success		200		{object}	object{elections=[]ElectionSummary}
//	@Router			/elections/page/{page} [get]
func (a *API) electionFullListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	page := 0
	if ctx.URLParam("page") != "" {
		var err error
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber.With(ctx.URLParam("page"))
		}
	}
	elections, err := a.indexer.ProcessList(nil, page*MaxPageSize, MaxPageSize, "", 0, 0, "", false)
	if err != nil {
		return ErrCantFetchElectionList.WithErr(err)
	}

	list := []ElectionSummary{}
	for _, eid := range elections {
		e, err := a.indexer.ProcessInfo(eid)
		if err != nil {
			return ErrCantFetchElection.Withf("(%x): %v", eid, err)
		}
		list = append(list, a.electionSummary(e))
	}
	// wrap list in a struct to consistently return list in an object, return empty
	// object if the list does not contains any result
	data, err := json.Marshal(struct {
		Elections []ElectionSummary `json:"elections"`
	}{list})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionHandler
//
//	@Summary		Election information
//	@Description	Get full election information
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			electionID	path		string	true	"Election id"
//	@Success		200			{object}	Election
//	@Router			/elections/{electionID} [get]
func (a *API) electionHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil {
		return ErrCantParseElectionID.Withf("(%s): %v", ctx.URLParam("electionID"), err)
	}
	proc, err := a.indexer.ProcessInfo(electionID)
	if err != nil {
		if errors.Is(err, indexer.ErrProcessNotFound) {
			return ErrElectionNotFound
		}
		return ErrCantFetchElection.Withf("(%x): %v", electionID, err)
	}

	election := Election{
		ElectionSummary: a.electionSummary(proc),
		MetadataURL:     proc.Metadata,
		CreationTime:    proc.CreationTime,
		VoteMode:        VoteMode{EnvelopeType: proc.Envelope},
		ElectionMode:    ElectionMode{ProcessMode: proc.Mode},
		TallyMode:       TallyMode{ProcessVoteOptions: proc.VoteOpts},
		Census: &ElectionCensus{
			CensusOrigin:  models.CensusOrigin_name[proc.CensusOrigin],
			CensusRoot:    proc.CensusRoot,
			CensusURL:     proc.CensusURI,
			MaxCensusSize: proc.MaxCensusSize,
		},
	}
	election.Status = models.ProcessStatus_name[proc.Status]

	if proc.HaveResults {
		election.Results = proc.ResultsVotes
	}

	// Try to retrieve the election metadata
	if a.storage != nil {
		stgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		metadataBytes, err := a.storage.Retrieve(stgCtx, election.MetadataURL, MaxOffchainFileSize)
		if err != nil {
			log.Warnf("cannot get metadata from %s: %v", election.MetadataURL, err)
		} else {
			// if metadata exists, add it to the election
			// if the metadata is not encrypted, unmarshal it, otherwise store it as bytes
			if !election.ElectionMode.EncryptedMetaData {
				electionMetadata := ElectionMetadata{}
				if err := json.Unmarshal(metadataBytes, &electionMetadata); err != nil {
					log.Warnf("cannot unmarshal metadata from %s: %v", election.MetadataURL, err)
				}
				election.Metadata = &electionMetadata
			} else {
				election.Metadata = metadataBytes
			}
		}
	}
	data, err := json.Marshal(election)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionVotesCountHandler
//
//	@Summary		Count election votes
//	@Description	Get the number of votes for an election
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			electionID	path		string	true	"Election id"
//	@Success		200			{object}	object{count=number}
//	@Router			/elections/{electionID}/votes/count [get]
func (a *API) electionVotesCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return ErrCantParseElectionID.Withf("(%s): %v", ctx.URLParam("electionID"), err)
	}
	// check process exists and return 404 if not
	// TODO: use the indexer to count votes
	if _, err := getElection(electionID, a.vocapp.State); err != nil {
		return err
	}

	count, err := a.vocapp.State.CountVotes(electionID, true)
	if errors.Is(err, statedb.ErrEmptyTree) {
		count = 0
	} else if err != nil {
		return ErrCantCountVotes.WithErr(err)
	}
	data, err := json.Marshal(
		struct {
			Count uint64 `json:"count"`
		}{Count: count},
	)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionKeysHandler
//
//	@Summary		List encryption keys
//	@Description	Returns the list of public/private encryption keys
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			electionID	path		string	true	"Election id"
//	@Success		200			{object}	ElectionKeys
//	@Router			/elections/{electionID}/keys [get]
func (a *API) electionKeysHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return ErrCantParseElectionID.Withf("(%s): %v", ctx.URLParam("electionID"), err)
	}
	// TODO: sqlite also has public and private keys, consider using it instead
	process, err := getElection(electionID, a.vocapp.State)
	if err != nil {
		return err
	}
	if !process.GetEnvelopeType().EncryptedVotes {
		return ErrNoElectionKeys
	}

	election := ElectionKeys{}
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
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionVotesHandler
//
//	@Summary		List election votes
//	@Description	Returns the list of voteIDs for an election (paginated)
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			electionID	path		string	true	"Election id"
//	@Param			page		path		number	true	"Page "
//	@Success		200			{object}	object{votes=[]Vote}
//	@Router			/elections/{electionID}/votes/page/{page} [get]
func (a *API) electionVotesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return ErrCantParseElectionID.Withf("(%s): %v", ctx.URLParam("electionID"), err)
	}
	// TODO: remove the getElection call?
	if _, err := getElection(electionID, a.vocapp.State); err != nil {
		return err
	}
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize

	votesRaw, err := a.indexer.GetEnvelopes(electionID, MaxPageSize, page, "")
	if err != nil {
		if errors.Is(err, indexer.ErrVoteNotFound) {
			return ErrVoteNotFound
		}
		return ErrCantFetchEnvelope.WithErr(err)
	}
	votes := []Vote{}
	for _, v := range votesRaw {
		votes = append(votes, Vote{
			VoteID:           v.Nullifier,
			VoterID:          v.VoterID,
			TxHash:           v.TxHash,
			BlockHeight:      v.Height,
			TransactionIndex: &v.TxIndex,
		})
	}
	data, err := json.Marshal(struct {
		Votes []Vote `json:"votes"`
	}{votes})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionScrutinyHandler
//
//	@Summary				Election results
//	@Description.markdown	electionScrutinyHandler
//	@Tags					Elections
//	@Accept					json
//	@Produce				json
//	@Param					electionID	path		string	true	"Election id"
//	@Success				200			{object}	ElectionResults
//	@Router					/elections/{electionID}/scrutiny [get]
func (a *API) electionScrutinyHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return ErrCantParseElectionID.Withf("(%s): %v", ctx.URLParam("electionID"), err)
	}
	process, err := getElection(electionID, a.vocapp.State)
	if err != nil {
		return err
	}
	// do not make distinction between live results elections and encrypted results elections
	// since we fetch the results from the blockchain state, elections must be terminated and
	// results must be available
	if process.Status != models.ProcessStatus_RESULTS {
		return ErrElectionResultsNotYetAvailable
	}
	if process.Results == nil {
		return ErrElectionResultsIsNil
	}

	electionResults := &ElectionResults{
		CensusRoot:            process.CensusRoot,
		ElectionID:            electionID,
		SourceContractAddress: process.SourceNetworkContractAddr,
		OrganizationID:        process.EntityId,
		Results:               state.GetFriendlyResults(process.Results.Votes),
	}

	// add the abi encoded results
	electionResults.ABIEncoded, err = encodeEVMResultsArgs(
		common.BytesToHash(electionID),
		common.BytesToAddress(electionResults.OrganizationID),
		common.BytesToHash(electionResults.CensusRoot),
		common.BytesToAddress(electionResults.SourceContractAddress),
		electionResults.Results,
	)
	if err != nil {
		return ErrCantABIEncodeResults.WithErr(err)
	}

	data, err := json.Marshal(electionResults)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionCreateHandler
//
//	@Summary				Create election
//	@Description.markdown	electionCreateHandler
//	@Tags					Elections
//	@Accept					json
//	@Produce				json
//	@Param					transaction	body		models.SignedTx	true	"Uses `txPayload` protobuf signed transaction, and the `metadata` base64-encoded JSON object"
//	@Success				200			{object}	ElectionCreate	"It return txId, electionId and the metadataURL for the newly created election. If metadataURL is returned empty, means that there is some issue with the storage provider.""
//	@Router					/elections [post]
func (a *API) electionCreateHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &ElectionCreate{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}

	// check if the transaction is of the correct type and extract metadata URI
	metadataURI, isEncryptedMetadata, err := func() (string, bool, error) {
		stx := &models.SignedTx{}
		if err := proto.Unmarshal(req.TxPayload, stx); err != nil {
			return "", false, err
		}
		tx := &models.Tx{}
		if err := proto.Unmarshal(stx.GetTx(), tx); err != nil {
			return "", false, err
		}
		if np := tx.GetNewProcess(); np != nil {
			if p := np.GetProcess(); p != nil {
				encryptedMeta := false
				if p.GetMode() != nil {
					encryptedMeta = p.GetMode().EncryptedMetaData
				}
				return p.GetMetadata(), encryptedMeta, nil
			}
		}
		return "", false, ErrCantExtractMetadataURI
	}()
	if err != nil {
		return err
	}

	// Check if the tx metadata URI is provided (in case of metadata bytes provided).
	// Note that we enforce the metadata URI to be provided in the tx payload only if
	// req.Metadata is provided, but not in the other direction.
	if req.Metadata != nil && metadataURI == "" {
		return ErrMetadataProvidedButNoURI
	}

	var metadataCID string
	if req.Metadata != nil {
		// if election metadata defined and not encrypted, check the format
		if !isEncryptedMetadata {
			metadata := ElectionMetadata{}
			if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
				return ErrCantParseMetadataAsJSON.WithErr(err)
			}
		}

		// set metadataCID from metadata bytes
		metadataCID = ipfs.CalculateCIDv1json(req.Metadata)
		// check metadata URI matches metadata content
		if !ipfs.CIDequals(metadataCID, metadataURI) {
			return ErrMetadataURINotMatchContent
		}
	}

	// send the transaction
	res, err := a.sendTx(req.TxPayload)
	if err != nil {
		return err
	}

	resp := &ElectionCreate{
		TxHash:     res.Hash.Bytes(),
		ElectionID: res.Data.Bytes(),
	}

	// check the electionID returned by Vochain is actually valid
	pid := processid.ProcessID{}
	if err := pid.Unmarshal(resp.ElectionID); err != nil {
		return ErrVochainReturnedInvalidElectionID
	}

	// if metadata exists, add it to the storage
	if a.storage != nil && req.Metadata != nil {
		sctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		cid, err := a.storage.Publish(sctx, req.Metadata)
		if err != nil {
			log.Errorf("could not publish to storage: %v", err)
		} else {
			resp.MetadataURL = a.storage.URIprefix() + cid
		}
		if strings.TrimPrefix(cid, "ipfs://") != strings.TrimPrefix(metadataCID, "ipfs://") {
			log.Errorf("%s (%s != %s)", ErrVochainReturnedWrongMetadataCID, cid, metadataCID)
		}
	}

	var data []byte
	if data, err = json.Marshal(resp); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// computeCidHandler
//
//	@Summary		Compute IPFS CIDv1 of file
//	@Description	Helper endpoint to get the IPFS CIDv1 hash of a file
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			transaction	body		object{payload=string}	true	"File bytes base64 encoded"
//	@Success		200			{object}	File
//	@Router			/files/cid [post]
func (*API) computeCidHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	if len(msg.Data) > MaxOffchainFileSize {
		return ErrFileSizeTooBig.Withf("%d vs %d bytes", len(msg.Data), MaxOffchainFileSize)
	}
	req := &File{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}
	data, err := json.Marshal(&File{
		CID: "ipfs://" + ipfs.CalculateCIDv1json(req.Payload),
	})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionPriceHandler
//
//	@Summary		Compute election price
//	@Description	Helper endpoint to get the election price.
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			transaction	body		electionprice.ElectionParameters	true	"5 election parameters that are required for calculating the price"
//	@Success		200			{object}	object{price=number}
//	@Router			/elections/price [post]
func (a *API) electionPriceHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &electionprice.ElectionParameters{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}
	price := a.vocapp.State.ElectionPriceCalc.Price(req)
	data, err := json.Marshal(struct {
		Price uint64 `json:"price"`
	}{price},
	)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// getElection retrieves an election from the vochain state.
// If not found or nil, returns an apirest.APIerror
func getElection(electionID []byte, vs *state.State) (*models.Process, error) {
	process, err := vs.Process(electionID, true)
	if err != nil {
		if errors.Is(err, state.ErrProcessNotFound) {
			return nil, ErrElectionNotFound
		}
		return nil, ErrCantFetchElection.Withf("(%x): %v", electionID, err)
	}
	if process == nil {
		return nil, ErrElectionIsNil.Withf("%x", electionID)
	}
	return process, nil
}

// electionFilterPaginatedHandler
//
//	@Summary				List elections (filtered)
//	@Description.markdown	electionFilterPaginatedHandler
//	@Tags					Elections
//	@Accept					json
//	@Produce				json
//	@Param					page		path		number			true	"Page to paginate"
//	@Param					transaction	body		ElectionFilter	true	"Filtered by partial organizationID, partial processID, process status and with results available or not"
//	@Success				200			{object}	object{elections=[]ElectionSummary}
//	@Router					/elections/filter/page/{page} [post]
func (a *API) electionFilterPaginatedHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// get organizationId from the request body
	body := &ElectionFilter{}
	if err := json.Unmarshal(msg.Data, &body); err != nil {
		return ErrCantParseDataAsJSON.WithErr(err)
	}
	// check that at least one filter is set
	if body.OrganizationID == nil && body.ElectionID == nil && body.Status == "" && body.WithResults == nil {
		return ErrMissingParameter
	}
	// get page
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber.WithErr(err)
		}
	}
	page = page * MaxPageSize
	if body.WithResults == nil {
		withResults := false
		body.WithResults = &withResults
	}
	elections, err := a.indexer.ProcessList(
		body.OrganizationID,
		page,
		MaxPageSize,
		body.ElectionID.String(),
		0,
		0,
		body.Status,
		*body.WithResults,
	)
	if err != nil {
		return ErrCantFetchElectionList.WithErr(err)
	}
	if len(elections) == 0 {
		return ErrElectionNotFound
	}

	var list []ElectionSummary
	// get election summary
	for _, eid := range elections {
		e, err := a.indexer.ProcessInfo(eid)
		if err != nil {
			return ErrCantFetchElection.WithErr(err)
		}
		list = append(list, a.electionSummary(e))
	}
	data, err := json.Marshal(struct {
		Elections []ElectionSummary `json:"elections"`
	}{
		Elections: list,
	})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// nextElectionIDHandler
//
//	@Summary		Get next election ID
//	@Description	nextElectionIDHandler
//	@Tags			Elections
//	@Accept			json
//	@Produce		json
//	@Param			transaction	body		NextElectionID	true	"OrganizationID, CensusOrigin and EnvelopeType"
//	@Success		200			{object}	object{electionID=string}
//	@Router			/elections/id [post]
func (a *API) nextElectionIDHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	body := &NextElectionID{}
	if err := json.Unmarshal(msg.Data, body); err != nil {
		return err
	}
	process := &models.Process{
		EntityId:     body.OrganizationID,
		CensusOrigin: models.CensusOrigin(body.CensusOrigin),
		EnvelopeType: &models.EnvelopeType{
			Serial:         body.EnvelopeType.Serial,
			Anonymous:      body.EnvelopeType.Anonymous,
			EncryptedVotes: body.EnvelopeType.EncryptedVotes,
			UniqueValues:   body.EnvelopeType.UniqueValues,
			CostFromWeight: body.EnvelopeType.CostFromWeight,
		},
	}
	pid, err := processid.BuildProcessID(
		process,
		a.vocapp.State,
	)
	if err != nil {
		return ErrCantParseElectionID.WithErr(err)
	}

	data, err := json.Marshal(struct {
		ElectionID string `json:"electionID"`
	}{
		ElectionID: hex.EncodeToString(pid.Marshal()),
	})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
