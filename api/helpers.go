package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (a *API) electionSummaryList(pids ...[]byte) ([]*ElectionSummary, error) {
	processes := []*ElectionSummary{}
	for _, p := range pids {
		procInfo, err := a.indexer.ProcessInfo(p)
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

// convertKeysToCamel converts all keys in a JSON object to camelCase.
func convertKeysToCamel(j json.RawMessage) json.RawMessage {
	m := make(map[string]json.RawMessage)
	var a []json.RawMessage
	// check if its a JSON object of map type '{}'
	if j[0] == '{' {
		if err := json.Unmarshal([]byte(j), &m); err != nil {
			// not a JSON object
			return j
		}
		// check if its a JSON object of array type '[]'
	} else if j[0] == '[' {
		if err := json.Unmarshal([]byte(j), &a); err != nil {
			// not a JSON object
			return j
		}
	} else {
		// not a JSON object, but an element
		return j
	}

	var b []byte
	var err error
	if len(a) == 0 {
		// if its a map, convert all keys recursively
		for k, v := range m {
			fixed := strcase.ToLowerCamel(k)
			delete(m, k)
			m[fixed] = convertKeysToCamel(v)
		}

		b, err = json.Marshal(m)
		if err != nil {
			return j
		}
	} else {
		// if its an array, convert all elements recursively
		for i, v := range a {
			a[i] = convertKeysToCamel(v)
		}

		b, err = json.Marshal(a)
		if err != nil {
			return j
		}

	}
	return json.RawMessage(b)
}
