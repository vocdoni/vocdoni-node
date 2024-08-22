package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	cometpool "github.com/cometbft/cometbft/mempool"
	cometcoretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iancoleman/strcase"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (a *API) electionSummary(pi *indexertypes.Process) *ElectionSummary {
	return &ElectionSummary{
		ElectionID:     pi.ID,
		OrganizationID: pi.EntityID,
		Status:         models.ProcessStatus_name[pi.Status],
		StartDate:      pi.StartDate,
		EndDate:        pi.EndDate,
		FinalResults:   pi.FinalResults,
		VoteCount:      pi.VoteCount,
		ManuallyEnded:  pi.ManuallyEnded,
		ChainID:        pi.ChainID,
	}
}

// sendTx wraps a.vocapp.SendTx(). If an error is returned, it's wrapped into an apirest.APIerror
func (a *API) sendTx(tx []byte) (*cometcoretypes.ResultBroadcastTx, error) {
	resp, err := a.vocapp.SendTx(tx)
	switch {
	case errors.As(err, &cometpool.ErrMempoolIsFull{}):
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
func isTransactionType[T any](signedTxBytes []byte) (bool, error) {
	stx := &models.SignedTx{}
	if err := proto.Unmarshal(signedTxBytes, stx); err != nil {
		return false, err
	}
	tx := &models.Tx{}
	if err := proto.Unmarshal(stx.GetTx(), tx); err != nil {
		return false, err
	}
	_, ok := tx.Payload.(T)
	return ok, nil
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
func encodeEVMResultsArgs(electionId common.Hash, organizationId common.Address, censusRoot common.Hash,
	sourceContractAddress common.Address, results [][]*types.BigInt,
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

// decryptVotePackage decrypts a vote package using the given private keys and indexes.
func decryptVotePackage(vp []byte, privKeys []string, indexes []uint32) ([]byte, error) {
	for i := len(indexes) - 1; i >= 0; i-- {
		if indexes[i] >= uint32(len(privKeys)) {
			return nil, fmt.Errorf("invalid key index %d", indexes[i])
		}
		priv, err := nacl.DecodePrivate(privKeys[indexes[i]])
		if err != nil {
			return nil, fmt.Errorf("cannot decode encryption key with index %d: (%s)", indexes[i], err)
		}
		vp, err = priv.Decrypt(vp)
		if err != nil {
			return nil, fmt.Errorf("cannot decrypt votePackage: (%s)", err)
		}
	}
	return vp, nil
}

// marshalAndSend marshals any passed struct and sends it over ctx.Send()
func marshalAndSend(ctx *httprouter.HTTPContext, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// parseNumber parses a string into an int.
//
// If the string is not parseable, returns an APIerror.
//
// The empty string "" is treated specially, returns 0 with no error.
func parseNumber(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	page, err := strconv.Atoi(s)
	if err != nil {
		return 0, ErrCantParseNumber.With(s)
	}
	return page, nil
}

// parsePage parses a string into an int.
//
// If the resulting int is negative, returns ErrNoSuchPage.
// If the string is not parseable, returns an APIerror.
//
// The empty string "" is treated specially, returns 0 with no error.
func parsePage(s string) (int, error) {
	page, err := parseNumber(s)
	if err != nil {
		return 0, err
	}
	if page < 0 {
		return 0, ErrPageNotFound
	}
	return page, nil
}

// parseLimit parses a string into an int.
//
// The empty string "" is treated specially, returns DefaultItemsPerPage with no error.
// If the resulting int is higher than MaxItemsPerPage, returns MaxItemsPerPage.
// If the resulting int is 0 or negative, returns DefaultItemsPerPage.
//
// If the string is not parseable, returns an APIerror.
func parseLimit(s string) (int, error) {
	limit, err := parseNumber(s)
	if err != nil {
		return 0, err
	}
	if limit > MaxItemsPerPage {
		limit = MaxItemsPerPage
	}
	if limit <= 0 {
		limit = DefaultItemsPerPage
	}
	return limit, nil
}

// parseStatus converts a string ("READY", "ready", "PAUSED", etc)
// to a models.ProcessStatus.
//
// If the string doesn't map to a value, returns an APIerror.
//
// The empty string "" is treated specially, returns 0 with no error.
func parseStatus(s string) (models.ProcessStatus, error) {
	if s == "" {
		return 0, nil
	}
	status, found := models.ProcessStatus_value[strings.ToUpper(s)]
	if !found {
		return 0, ErrParamStatusInvalid.With(s)
	}
	return models.ProcessStatus(status), nil
}

// parseHexString converts a string like 0x1234cafe (or 1234cafe)
// to a types.HexBytes.
//
// If the string can't be parsed, returns an APIerror.
func parseHexString(s string) (types.HexBytes, error) {
	orgID, err := hex.DecodeString(util.TrimHex(s))
	if err != nil {
		return nil, ErrCantParseHexString.Withf("%q", s)
	}
	return orgID, nil
}

// parseBool parses a string into a boolean value.
//
// The empty string "" is treated specially, returns a nil pointer with no error.
func parseBool(s string) (*bool, error) {
	if s == "" {
		return nil, nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return nil, ErrCantParseBoolean.With(s)
	}
	return &b, nil
}

// parseDate parses an RFC3339 string into a time.Time value.
// As a convenience, accepts also time.DateOnly format (i.e. 2006-01-02).
//
// The empty string "" is treated specially, returns a nil pointer with no error.
func parseDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	b, err := time.Parse(time.RFC3339, s)
	if err != nil {
		if b, err := time.Parse(time.DateOnly, s); err == nil {
			return &b, nil
		}
		return nil, ErrCantParseDate.WithErr(err)
	}
	return &b, nil
}

// parsePaginationParams returns a PaginationParams filled with the passed params
func parsePaginationParams(paramPage, paramLimit string) (PaginationParams, error) {
	page, err := parsePage(paramPage)
	if err != nil {
		return PaginationParams{}, err
	}

	limit, err := parseLimit(paramLimit)
	if err != nil {
		return PaginationParams{}, err
	}

	return PaginationParams{
		Page:  page,
		Limit: limit,
	}, nil
}

// calculatePagination calculates PreviousPage, NextPage and LastPage.
//
// If page is negative or higher than LastPage, returns an APIerror (ErrPageNotFound)
func calculatePagination(page int, limit int, totalItems uint64) (*Pagination, error) {
	// pages start at 0 index, for legacy reasons
	lastp := int(math.Ceil(float64(totalItems)/float64(limit)) - 1)
	if totalItems == 0 {
		lastp = 0
	}

	if page > lastp || page < 0 {
		return nil, ErrPageNotFound
	}

	var prevp, nextp *uint64
	if page > 0 {
		prevPage := uint64(page - 1)
		prevp = &prevPage
	}
	if page < lastp {
		nextPage := uint64(page + 1)
		nextp = &nextPage
	}

	return &Pagination{
		TotalItems:   totalItems,
		PreviousPage: prevp,
		CurrentPage:  uint64(page),
		NextPage:     nextp,
		LastPage:     uint64(lastp),
	}, nil
}

// paramsFromCtxFunc calls f(key) for each key passed, and the resulting value is saved in map[key] of the returned map
func paramsFromCtxFunc(f func(key string) string, keys ...string) map[string]string {
	m := make(map[string]string)
	for _, key := range keys {
		m[key] = f(key)
	}
	return m
}
