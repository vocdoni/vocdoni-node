package urlapi

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
)

func (u *URLAPI) EnableVotingHandlers(vocdoniAPP *vochain.BaseApplication, vocdoniInfo *vochaininfo.VochainInfo,
	scrut *scrutinizer.Scrutinizer) error {
	u.vocapp = vocdoniAPP
	u.vocinfo = vocdoniInfo
	u.scrutinizer = scrut

	if err := u.api.RegisterMethod(
		"/entities/{entity}/processes/{status}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.entityProcessHandler,
	); err != nil {
		return err
	}

	if err := u.api.RegisterMethod(
		"/process/{process}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.processHandler,
	); err != nil {
		return err
	}

	return nil
}

// https://server/v1/pub/entities/<entity>/processes/<status>
func (u *URLAPI) entityProcessHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	entityID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("entity")))
	if err != nil {
		return fmt.Errorf("entityID (%s) cannot be decoded", ctx.URLParam("entity"))
	}
	var pids [][]byte
	switch ctx.URLParam("status") {
	case "active":
		pids, err = u.scrutinizer.ProcessList(entityID, 0, 128, "", 0, "", "READY", false)
		if err != nil {
			return fmt.Errorf("cannot fetch process list: %w", err)
		}
	case "ended":
		pids, err = u.scrutinizer.ProcessList(entityID, 0, 128, "", 0, "", "RESULTS", false)
		if err != nil {
			return fmt.Errorf("cannot fetch process list: %w", err)
		}
		pids2, err := u.scrutinizer.ProcessList(entityID, 0, 128, "", 0, "", "ENDED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch process list: %w", err)
		}
		pids = append(pids, pids2...)
	default:
		return fmt.Errorf("missing status parameter or unknown")
	}

	processes, err := u.getProcessSummaryList(pids...)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&EntitiesMsg{
		EntityID:  types.HexBytes(entityID),
		Processes: processes,
	})
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	if err = ctx.Send(data, bearerstdapi.HTTPstatusCodeOK); err != nil {
		log.Warn(err)
	}
	return nil
}

// https://server/v1/priv/processes/<process>
func (u *URLAPI) processHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	processID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("process")))
	if err != nil {
		return fmt.Errorf("entityID (%s) cannot be decoded", ctx.URLParam("process"))
	}
	proc, err := u.scrutinizer.ProcessInfo(processID)
	if err != nil {
		return fmt.Errorf("cannot fetch process %x: %w", processID, err)
	}

	var jProcess Process
	jProcess.Status = models.ProcessStatus_name[proc.Status]
	jProcess.Type = u.formatProcessType(proc.Envelope)
	jProcess.VoteCount, err = u.scrutinizer.GetEnvelopeHeight(processID)
	if err != nil {
		return fmt.Errorf("cannot get envelope height: %w", err)
	}
	if proc.HaveResults {
		results, err := u.scrutinizer.GetResults(processID)
		if err != nil {
			return fmt.Errorf("cannot get envelope height: %w", err)
		}
		for _, r := range scrutinizer.GetFriendlyResults(results.Votes) {
			jProcess.Results = append(jProcess.Results, Result{Value: r})
		}
	}

	data, err := json.Marshal(jProcess)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	if err = ctx.Send(data, bearerstdapi.HTTPstatusCodeOK); err != nil {
		log.Warn(err)
	}
	return nil
}
