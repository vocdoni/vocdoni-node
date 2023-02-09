package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt" // required for evm encoding
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/processid"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	ElectionHandler     = "elections"
	MaxOffchainFileSize = 1024 * 1024 * 1 // 1MB
)

func (a *API) enableElectionHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/elections",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionFullListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionFullListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/keys",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionKeysHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/votes",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionVotesHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/votes/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionVotesCountHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/votes/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionVotesHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/scrutiny",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionScrutinyHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections",
		"POST",
		apirest.MethodAccessTypePublic,
		a.electionCreateHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/files/cid",
		"POST",
		apirest.MethodAccessTypePublic,
		a.computeCidHandler,
	); err != nil {
		return err
	}

	return nil
}

// GET /elections
// GET /elections/page/<page>
func (a *API) electionFullListHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	page := 0
	if ctx.URLParam("page") != "" {
		var err error
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return fmt.Errorf("page (%s) cannot be decoded", ctx.URLParam("page"))
		}
	}
	elections, err := a.indexer.ProcessList(nil, page*MaxPageSize, MaxPageSize, "", 0, 0, "", false)
	if err != nil {
		return fmt.Errorf("cannot fetch election list: %w", err)
	}
	if len(elections) == 0 {
		return ctx.Send(nil, apirest.HTTPstatusCodeNotFound)
	}

	var list []ElectionSummary
	for _, eid := range elections {
		e, err := a.indexer.ProcessInfo(eid)
		if err != nil {
			return fmt.Errorf("cannot fetch electionID %x: %w", eid, err)
		}
		count, err := a.indexer.GetEnvelopeHeight(eid)
		if err != nil {
			return fmt.Errorf("cannot get envelope height: %w", err)
		}
		list = append(list, ElectionSummary{
			ElectionID:   eid,
			Status:       models.ProcessStatus_name[e.Status],
			StartDate:    a.vocinfo.HeightTime(int64(e.StartBlock)),
			EndDate:      a.vocinfo.HeightTime(int64(e.EndBlock)),
			FinalResults: e.FinalResults,
			VoteCount:    count,
		})
	}

	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// GET /elections/<electionID>
// get election information
func (a *API) electionHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil {
		return fmt.Errorf("electionID (%s) cannot be decoded", ctx.URLParam("electionID"))
	}
	proc, err := a.indexer.ProcessInfo(electionID)
	if err != nil {
		return fmt.Errorf("cannot fetch electionID %x: %w", electionID, err)
	}
	count, err := a.indexer.GetEnvelopeHeight(electionID)
	if err != nil {
		return fmt.Errorf("cannot get envelope height: %w", err)
	}

	election := Election{
		ElectionSummary: ElectionSummary{
			ElectionID:   electionID,
			Status:       models.ProcessStatus_name[proc.Status],
			StartDate:    a.vocinfo.HeightTime(int64(proc.StartBlock)),
			EndDate:      a.vocinfo.HeightTime(int64(proc.EndBlock)),
			FinalResults: proc.FinalResults,
			VoteCount:    count,
		},
		MetadataURL:   proc.Metadata,
		ElectionCount: proc.EntityIndex,
		CreationTime:  proc.CreationTime,
		VoteMode:      VoteMode{EnvelopeType: proc.Envelope},
		ElectionMode:  ElectionMode{ProcessMode: proc.Mode},
		TallyMode:     TallyMode{ProcessVoteOptions: proc.VoteOpts},
		Census: &ElectionCensus{
			CensusOrigin:           models.CensusOrigin_name[proc.CensusOrigin],
			CensusRoot:             proc.CensusRoot,
			PostRegisterCensusRoot: proc.RollingCensusRoot,
			CensusURL:              proc.CensusURI,
		},
	}
	election.Status = models.ProcessStatus_name[proc.Status]

	if proc.HaveResults {
		results, err := a.indexer.GetResults(electionID)
		if err != nil {
			return fmt.Errorf("cannot get envelope height: %w", err)
		}
		election.Results = results.Votes
	}

	// Try to retrieve the election metadata
	if a.storage != nil {
		stgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		metadataBytes, err := a.storage.Retrieve(stgCtx, election.MetadataURL, MaxOffchainFileSize)
		if err != nil {
			log.Warnf("cannot get metadata from %s: %v", election.MetadataURL, err)
		} else {
			electionMetadata := ElectionMetadata{}
			if err := json.Unmarshal(metadataBytes, &electionMetadata); err != nil {
				log.Warnf("cannot unmarshal metadata from %s: %v", election.MetadataURL, err)
			}
			election.Metadata = &electionMetadata
		}
	}
	data, err := json.Marshal(election)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// GET /elections/<electionID>/votes/count
// get the number of votes for an election
func (a *API) electionVotesCountHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return fmt.Errorf("electionID (%s) cannot be decoded", ctx.URLParam("electionID"))
	}
	count := a.vocapp.State.CountVotes(electionID, true)
	data, err := json.Marshal(
		struct {
			Count uint32 `json:"count"`
		}{Count: count},
	)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// GET /elections/<electionID>/keys
// returns the list of public/private encryption keys
func (a *API) electionKeysHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return fmt.Errorf("electionID (%q) cannot be decoded", ctx.URLParam("electionID"))
	}

	process, err := a.vocapp.State.Process(electionID, true)
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
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// GET elections/<electionID>/votes/page/<page>
// returns the list of voteIDs for an election (paginated)
func (a *API) electionVotesHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return fmt.Errorf("electionID (%q) cannot be decoded", ctx.URLParam("electionID"))
	}
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return fmt.Errorf("cannot parse page number")
		}
	}
	page = page * MaxPageSize

	votesRaw, err := a.indexer.GetEnvelopes(electionID, MaxPageSize, page, "")
	if err != nil {
		return err
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
	data, err := json.Marshal(votes)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// GET /elections/<electionID>/scrutiny
// returns the consensus results of an election
func (a *API) electionScrutinyHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil || electionID == nil {
		return fmt.Errorf("electionID (%q) cannot be decoded", ctx.URLParam("electionID"))
	}
	process, err := a.vocapp.State.Process(electionID, true)
	if err != nil {
		return fmt.Errorf("cannot get election results: %w", err)
	}
	if process == nil {
		return fmt.Errorf("cannot fetch process %x", electionID)
	}
	// do not make distinction between live results elections and encrypted results elections
	// since we fetch the results from the blockchain state, elections must be terminated and
	// results must be available
	if process.Status != models.ProcessStatus_RESULTS {
		return fmt.Errorf("election results are not available yet")
	}
	if process.Results == nil {
		return fmt.Errorf("no results available")
	}

	// intermediateResults is an array of the results that are signed
	// by oracles. There can be void results see vochain/state/process.go#L330
	// but we want to ignore them. The way we can know if a result is void is
	// by checking if the oracle address is empty.
	intermediateResults := []*models.ProcessResult{}
	for k, processResult := range process.Results {
		if processResult == nil {
			log.Warnw("nil process result",
				"electionID", fmt.Sprintf("%x", electionID),
				"results index", k,
			)
			continue
		}
		if len(processResult.OracleAddress) == 0 { // check oracle sent the results otherwise ignore
			continue
		}
		intermediateResults = append(intermediateResults, processResult)
	} // now we have an array of non void results set by a set of oracles via SetProcessResults

	if len(intermediateResults) == 0 {
		return fmt.Errorf("no results available")
	}

	// Results are being compared because we want to make sure that
	// all oracles are sending the same results.
	equalResults := func(baseResult, otherResult *models.ProcessResult) bool {
		// if the number of votes is different, the results are different
		if len(baseResult.Votes) != len(otherResult.Votes) {
			log.Warnw("different number of votes",
				"baseResult", len(baseResult.Votes),
				"otherResult", len(otherResult.Votes),
			)
			return false
		}
		for k, vote1 := range otherResult.Votes {
			vote2 := baseResult.Votes[k]
			if vote1 == nil {
				log.Warnw("invalid question result (nil) at index", k)
				continue
			}
			if len(vote1.Question) != len(vote2.Question) {
				log.Warnw("question length do not match",
					"baseResult", len(vote2.Question),
					"otherResult", len(vote1.Question),
				)
				continue
			}
			for kk, question1 := range vote1.Question {
				question2 := vote2.Question[kk]
				if question1 == nil {
					log.Warnw("invalid question option (nil) at index", kk)
					continue
				}
				if !bytes.Equal(question1, question2) {
					log.Warnw("question option bytes do not match",
						"baseResult", fmt.Sprintf("%x", question2),
						"otherResult", fmt.Sprintf("%x", question1),
					)
					return false
				}
			}
		}
		return true
	}

	firstResult := intermediateResults[0]
	// compare only if there are more than one result
	if len(intermediateResults) > 1 {
		for _, processResult := range intermediateResults[1:] {
			if !equalResults(firstResult, processResult) {
				return fmt.Errorf("reported election results missmatch")
			}
		}
	} // at this point results are equal

	electionResults := &ElectionResults{
		CensusRoot:            process.CensusRoot,
		ElectionID:            electionID,
		SourceContractAddress: process.SourceNetworkContractAddr,
		OrganizationID:        process.EntityId,
		Results:               state.GetFriendlyResults(firstResult.Votes),
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
		return fmt.Errorf("cannot abi.encode results: %w", err)
	}

	data, err := json.Marshal(electionResults)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// POST elections
// creates a new election
func (a *API) electionCreateHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &ElectionCreate{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}

	// check if the transaction is of the correct type and extract metadata URI
	metadataURI, err := func() (string, error) {
		stx := &models.SignedTx{}
		if err := proto.Unmarshal(req.TxPayload, stx); err != nil {
			return "", err
		}
		tx := &models.Tx{}
		if err := proto.Unmarshal(stx.GetTx(), tx); err != nil {
			return "", err
		}
		if np := tx.GetNewProcess(); np != nil {
			if p := np.GetProcess(); p != nil {
				return p.GetMetadata(), nil
			}
		}
		return "", fmt.Errorf("could not get metadata URI")
	}()
	if err != nil {
		return err
	}

	// Check if the tx metadata URI is provided (in case of metadata bytes provided).
	// Note that we enforce the metadata URI to be provided in the tx payload only if
	// req.Metadata is provided, but not in the other direction.
	if req.Metadata != nil && metadataURI == "" {
		return fmt.Errorf("metadata provided but no metadata URI found in transaction")
	}

	var metadataCID string
	if req.Metadata != nil {
		// if election metadata defined, check the format
		metadata := ElectionMetadata{}
		if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
			return fmt.Errorf("wrong metadata format: %w", err)
		}

		// set metadataCID from metadata bytes
		metadataCID = data.CalculateIPFSCIDv1json(req.Metadata)
		// check metadata URI matches metadata content
		if !data.IPFSCIDequals(metadataCID, strings.TrimPrefix(metadataURI, "ipfs://")) {
			return fmt.Errorf("metadata URI does not match metadata content")
		}
	}

	// send the transaction
	res, err := a.vocapp.SendTx(req.TxPayload)
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("no reply from vochain")
	}
	if res.Code != 0 {
		return fmt.Errorf("%s", string(res.Data))
	}

	resp := &ElectionCreate{
		TxHash:     res.Hash.Bytes(),
		ElectionID: res.Data.Bytes(),
	}

	// check the electionID returned by Vochain is actually valid
	pid := processid.ProcessID{}
	if err := pid.Unmarshal(resp.ElectionID); err != nil {
		return fmt.Errorf("received election id after executing transaction is not valid")
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
			log.Errorw(fmt.Errorf("metadata do not match: %s != %s", cid, metadataCID),
				"published metadata returned an unexpected CID")
		}
	}

	var data []byte
	if data, err = json.Marshal(resp); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// POST /files/cid
// helper endpoint to get the IPFS CID hash of a file
func (a *API) computeCidHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	if len(msg.Data) > MaxOffchainFileSize {
		return fmt.Errorf("file size exceeds the maximum allowed (%d bytes)", MaxOffchainFileSize)
	}
	req := &File{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}
	// check if the file is a valid JSON object
	var js json.RawMessage
	if err := json.Unmarshal(req.Payload, &js); err != nil {
		return fmt.Errorf("payload is not a JSON object")
	}
	data, err := json.Marshal(&File{
		CID: "ipfs://" + data.CalculateIPFSCIDv1json(req.Payload),
	})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}
