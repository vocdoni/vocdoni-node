package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt" // required for evm encoding
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iancoleman/strcase"
	"go.vocdoni.io/dvote/types"
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
// Note that the keys are also sorted.
func convertKeysToCamel(data []byte) []byte {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return data // not valid JSON
	}
	m2 := convertKeysToCamelInner(m)
	data2, err := json.Marshal(m2)
	if err != nil {
		panic(err) // should never happen
	}
	return data2
}

func convertKeysToCamelInner(val any) any {
	switch val := val.(type) {
	case map[string]any: // convert the keys and recurse
		for k, v := range val {
			k2 := strcase.ToLowerCamel(k)
			if k2 != k {
				if _, ok := val[k2]; ok {
					panic(fmt.Sprintf("duplicate camel case key: %q", k2))
				}
				delete(val, k)
			}
			val[k2] = convertKeysToCamelInner(v)
		}
	case []any: // recurse
		for i, v := range val {
			val[i] = convertKeysToCamelInner(v)
		}
	}
	return val
}

// encodeEVMResultsArgs encodes the arguments for the EVM mimicking the Solidity built-in abi.encode(args...)
// in this case we encode the organizationId the censusRoot and the results that will be translated in the EVM
// contract to the corresponding struct{address, bytes32, uint256[][]}
func encodeEVMResultsArgs(electionId common.Hash,
	organizationId common.Address,
	censusRoot common.Hash,
	sourceContractAddress common.Address,
	results [][]*types.BigInt,
) (string, error) {
	address, _ := abi.NewType("address", "", nil)
	bytes32, _ := abi.NewType("bytes32", "", nil)
	uint256SliceNested, _ := abi.NewType("uint256[][]", "", nil)
	args := abi.Arguments{
		{Type: bytes32},
		{Type: address},
		{Type: bytes32},
		{Type: address},
		{Type: uint256SliceNested},
	}
	// change results from *types.BigInt to *bigInt as args.Pack requires math/big.Int type
	resultsStd := make([][]*big.Int, len(results))
	for i, r := range results {
		resultsStd[i] = make([]*big.Int, len(r))
		for j, v := range r {
			resultsStd[i][j] = v.ToStdBigInt()
		}
	}
	abiEncodedResultsBytes, err := args.Pack(electionId, organizationId, censusRoot, sourceContractAddress, resultsStd)
	if err != nil {
		return "", fmt.Errorf("error encoding abi: %w", err)
	}
	return fmt.Sprintf("0x%s", hex.EncodeToString(abiEncodedResultsBytes)), nil
}
