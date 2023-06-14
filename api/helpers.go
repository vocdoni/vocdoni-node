package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt" // required for evm encoding
	"math/big"
	"reflect"
	"strings"

	cmtpool "github.com/cometbft/cometbft/mempool"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
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
	for _, pid := range pids {
		// TODO(mvdan): We construct ElectionSummaries in many places, deduplicate.
		// TODO(mvdan): ProcessInfo could give us the results envelope height as well.
		procInfo, err := a.indexer.ProcessInfo(pid)
		if err != nil {
			return nil, ErrCantFetchElection.WithErr(err)
		}
		count, err := a.indexer.CountVotes(pid)
		if err != nil {
			return nil, ErrCantFetchEnvelopeHeight.WithErr(err)
		}
		processes = append(processes, &ElectionSummary{
			ElectionID:     procInfo.ID,
			OrganizationID: procInfo.EntityID,
			Status:         strings.ToLower(models.ProcessStatus_name[procInfo.Status]),
			StartDate:      procInfo.CreationTime,
			EndDate:        a.vocinfo.HeightTime(int64(procInfo.EndBlock)),
			FinalResults:   procInfo.FinalResults,
			VoteCount:      count,
		})
	}
	return processes, nil
}

// sendTx wraps a.vocapp.SendTx(). If an error is returned, it's wrapped into an apirest.APIerror
func (a *API) sendTx(tx []byte) (*ctypes.ResultBroadcastTx, error) {
	resp, err := a.vocapp.SendTx(tx)
	switch {
	case errors.As(err, &cmtpool.ErrMempoolIsFull{}):
		return nil, ErrVochainOverloaded.WithErr(err)
	case err != nil:
		return nil, ErrVochainSendTxFailed.WithErr(err)
	case resp == nil:
		return nil, ErrVochainEmptyReply
	case resp.Code != 0:
		return nil, ErrVochainReturnedErrorCode.Withf("(%d) %s", resp.Code, string(resp.Data))
	}
	return resp, nil
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
			resultsStd[i][j] = v.MathBigInt()
		}
	}
	abiEncodedResultsBytes, err := args.Pack(electionId, organizationId, censusRoot, sourceContractAddress, resultsStd)
	if err != nil {
		return "", ErrCantABIEncodeResults.WithErr(err)
	}
	return fmt.Sprintf("0x%s", hex.EncodeToString(abiEncodedResultsBytes)), nil
}
