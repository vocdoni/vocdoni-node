package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	ElectionHandler     = "elections"
	MaxOffchainFileSize = 1024 * 1024 * 1 // 1MB
)

func (a *API) enableElectionHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/keys",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionKeysHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/votes",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionVotesHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/votes/count",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionVotesCountHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections/{electionID}/votes/page/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionVotesHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/elections",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		a.electionCreateHandler,
	); err != nil {
		return err
	}

	return nil
}

// /elections/<electionID>
// get election information
func (a *API) electionHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	electionID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("electionID")))
	if err != nil {
		return fmt.Errorf("electionID (%s) cannot be decoded", ctx.URLParam("electionID"))
	}
	proc, err := a.scrutinizer.ProcessInfo(electionID)
	if err != nil {
		return fmt.Errorf("cannot fetch electionID %x: %w", electionID, err)
	}
	count, err := a.scrutinizer.GetEnvelopeHeight(electionID)
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
		results, err := a.scrutinizer.GetResults(electionID)
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
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /elections/<electionID>/votes/count
// get the number of votes for an election
func (a *API) electionVotesCountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
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
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /elections/<electionID>/keys
// returns the list of public/private encryption keys
func (a *API) electionKeysHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
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
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// elections/<electionID>/votes/page/<page>
// returns the list of voteIDs for an election (paginated)
func (a *API) electionVotesHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
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

	votesRaw, err := a.scrutinizer.GetEnvelopes(electionID, MaxPageSize, page, "")
	if err != nil {
		return err
	}
	votes := []Vote{}
	for _, v := range votesRaw {
		votes = append(votes, Vote{
			VoteID:      v.Nullifier,
			VoterID:     v.VoterID,
			TxHash:      v.TxHash,
			BlockHeight: v.Height,
		})
	}
	data, err := json.Marshal(votes)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// POST elections
// creates a new election
func (a *API) electionCreateHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
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
	pid := vochain.ProcessID{}
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
		if cid != metadataCID {
			log.Errorf("metadata CID does not match metadata content (%s != %s)", cid, metadataCID)
		}
	}

	var data []byte
	if data, err = json.Marshal(resp); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
