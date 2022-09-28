package urlapi

import (
	"fmt"
	"strings"

	"go.vocdoni.io/proto/build/go/models"
)

func (u *URLAPI) getProcessSummaryList(pids ...[]byte) ([]*ElectionSummary, error) {
	processes := []*ElectionSummary{}
	for _, p := range pids {
		procInfo, err := u.scrutinizer.ProcessInfo(p)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch election info: %w", err)
		}
		processes = append(processes, &ElectionSummary{
			ElectionID: procInfo.ID,
			Status:     models.ProcessStatus_name[procInfo.Status],
			StartDate:  procInfo.CreationTime,
			EndDate:    u.vocinfo.HeightTime(int64(procInfo.EndBlock)),
		})
	}
	return processes, nil
}

func (u *URLAPI) formatElectionType(et *models.EnvelopeType) string {
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

func censusTypeToOrigin(ctype CensusTypeDescription) (models.CensusOrigin, []byte, error) {
	var origin models.CensusOrigin
	var root []byte
	switch ctype.Type {
	case "csp":
		origin = models.CensusOrigin_OFF_CHAIN_CA
		root = ctype.PublicKey
	case "tree":
		origin = models.CensusOrigin_OFF_CHAIN_TREE
		root = ctype.RootHash
	case "treeWeighted":
		origin = models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED
		root = ctype.RootHash
	default:
		return 0, nil, fmt.Errorf("census type %q is unknown", ctype)
	}
	if root == nil {
		return 0, nil, fmt.Errorf("census root is not correctyl specified")
	}
	return origin, root, nil
}
