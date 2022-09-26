package urlapi

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/proto/build/go/models"
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
