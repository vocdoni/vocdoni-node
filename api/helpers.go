package api

import (
	"fmt"
	"reflect"
	"strings"

	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (a *API) electionSummaryList(pids ...[]byte) ([]*ElectionSummary, error) {
	processes := []*ElectionSummary{}
	for _, p := range pids {
		procInfo, err := a.scrutinizer.ProcessInfo(p)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch election info: %w", err)
		}
		processes = append(processes, &ElectionSummary{
			ElectionID: procInfo.ID,
			Status:     strings.ToLower(models.ProcessStatus_name[procInfo.Status]),
			StartDate:  procInfo.CreationTime,
			EndDate:    a.vocinfo.HeightTime(int64(procInfo.EndBlock)),
		})
	}
	return processes, nil
}

func protoFormat(tx []byte) string {
	ptx := models.Tx{}
	if err := proto.Unmarshal(tx, &ptx); err != nil {
		return ""
	}
	pj := protojson.MarshalOptions{
		Multiline:       false,
		Indent:          "",
		EmitUnpopulated: true,
	}
	return pj.Format(&ptx)
}

// isTransactionType checks if the given transaction is of the given type.
// t is expected to be a pointer to a protobuf transaction message.
func isTransactionType(signedTxBytes []byte, t any) (bool, error) {
	stx := &models.SignedTx{}
	if err := proto.Unmarshal(signedTxBytes, stx); err != nil {
		return false, err
	}
	tx := &models.Tx{}
	if err := proto.Unmarshal(stx.GetTx(), tx); err != nil {
		return false, err
	}
	return reflect.TypeOf(tx.Payload) == reflect.TypeOf(t), nil
}
