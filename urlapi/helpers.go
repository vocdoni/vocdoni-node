package urlapi

import (
	"fmt"
	"strings"

	"go.vocdoni.io/proto/build/go/models"
)

func (u *URLAPI) getProcessSummaryList(pids ...[]byte) ([]*ProcessSummary, error) {
	processes := []*ProcessSummary{}
	for _, p := range pids {
		procInfo, err := u.scrutinizer.ProcessInfo(p)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch process info: %w", err)
		}
		processes = append(processes, &ProcessSummary{
			ProcessID: procInfo.ID,
			Status:    models.ProcessStatus_name[procInfo.Status],
			StartDate: procInfo.CreationTime,
			EndDate:   u.vocinfo.HeightTime(int64(procInfo.EndBlock)),
		})
	}
	return processes, nil
}

func (u *URLAPI) formatProcessType(et *models.EnvelopeType) string {
	ptype := strings.Builder{}

	if et.Anonymous {
		ptype.WriteString("anonymous")
	} else {
		ptype.WriteString("poll")
	}
	if et.EncryptedVotes {
		ptype.WriteString(" encrypted")
	} else {
		ptype.WriteString(" open")
	}
	if et.Serial {
		ptype.WriteString(" serial")
	} else {
		ptype.WriteString(" single")
	}
	return ptype.String()
}
