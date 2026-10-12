//nolint:lll
package api

import (
	"fmt"

	"go.vocdoni.io/dvote/httprouter/apirest"
)

// APIerror satisfies the error interface.
// Error() returns a human-readable description of the error.
//
// Error codes in the 4001-4999 range are the user's fault,
// and they return HTTP Status 400 or 404 (or even 204), whatever is most appropriate.
//
// Error codes 5001-5999 are the server's fault
// and they return HTTP Status 500 or 503, or something else if appropriate.
//
// The initial list of errors were more or less grouped by topic, but the list grows with time in a random fashion.
// NEVER change any of the current error codes, only append new errors after the current last 4XXX or 5XXX
// If you notice there's a gap (say, error code 4010, 4011 and 4013 exist, 4012 is missing) DON'T fill in the gap,
// that code was used in the past for some error (not anymore) and shouldn't be reused.
// There's no correlation between Code and HTTP Status,
// for example the fact that Code 4045 returns HTTP Status 404 Not Found is just a coincidence
//
// Do note that HTTPstatus 204 No Content implies the response body will be empty,
// so the Code and Message will actually be discarded, never sent to the client
var (
	ErrAddressMalformed                 = apirest.APIerror{Code: 4000, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("address malformed")}
	ErrDstAddressMalformed              = apirest.APIerror{Code: 4001, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("destination address malformed")}
	ErrDstAccountUnknown                = apirest.APIerror{Code: 4002, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("destination account is unknown")}
	ErrAccountNotFound                  = apirest.APIerror{Code: 4003, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("account not found")}
	ErrAccountAlreadyExists             = apirest.APIerror{Code: 4004, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("account already exists")}
	ErrOrgNotFound                      = apirest.APIerror{Code: 4006, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("organization not found")}
	ErrTransactionNotFound              = apirest.APIerror{Code: 4007, HTTPstatus: apirest.HTTPstatusNoContent, Err: fmt.Errorf("transaction not found")}
	ErrBlockNotFound                    = apirest.APIerror{Code: 4008, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("block not found")}
	ErrMarshalingJSONFailed             = apirest.APIerror{Code: 4011, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("marshaling JSON failed")}
	ErrCantParseOrgID                   = apirest.APIerror{Code: 4013, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse organizationId")}
	ErrCantParseAccountID               = apirest.APIerror{Code: 4014, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse accountId")}
	ErrCantParseBearerToken             = apirest.APIerror{Code: 4015, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse bearer token")}
	ErrCantParseDataAsJSON              = apirest.APIerror{Code: 4016, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse data as JSON")}
	ErrCantParseElectionID              = apirest.APIerror{Code: 4017, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse electionId")}
	ErrCantParseNumber                  = apirest.APIerror{Code: 4019, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse number")}
	ErrCantParsePayloadAsJSON           = apirest.APIerror{Code: 4020, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse payload as JSON")}
	ErrCantParseVoteID                  = apirest.APIerror{Code: 4021, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse voteId")}
	ErrCantExtractMetadataURI           = apirest.APIerror{Code: 4022, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot extract metadata URI")}
	ErrVoteIDMalformed                  = apirest.APIerror{Code: 4023, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("voteId is malformed")}
	ErrVoteNotFound                     = apirest.APIerror{Code: 4024, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("vote not found")}
	ErrCensusRootIsNil                  = apirest.APIerror{Code: 4026, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census root is nil")}
	ErrCensusTypeUnknown                = apirest.APIerror{Code: 4027, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census type is unknown")}
	ErrCensusIndexedFlagMismatch        = apirest.APIerror{Code: 4029, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census indexed flag mismatch")}
	ErrParamStatusInvalid               = apirest.APIerror{Code: 4031, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (status) invalid")}
	ErrParamNetworkInvalid              = apirest.APIerror{Code: 4037, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid network")}
	ErrParamToInvalid                   = apirest.APIerror{Code: 4038, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid address")}
	ErrIndexedCensusCantUseWeight       = apirest.APIerror{Code: 4040, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("indexed census cannot use weight")}
	ErrWalletNotFound                   = apirest.APIerror{Code: 4041, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("wallet not found")}
	ErrWalletPrivKeyAlreadyExists       = apirest.APIerror{Code: 4042, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("wallet private key already exists")}
	ErrElectionNotFound                 = apirest.APIerror{Code: 4045, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("election not found")}
	ErrNoElectionKeys                   = apirest.APIerror{Code: 4047, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("no election keys available")}
	ErrMissingParameter                 = apirest.APIerror{Code: 4048, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("one or more parameters are missing")}
	ErrInvalidStatus                    = apirest.APIerror{Code: 4050, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid status")}
	ErrUnmarshalingServerProto          = apirest.APIerror{Code: 4052, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error unmarshaling protobuf data")}
	ErrMarshalingServerProto            = apirest.APIerror{Code: 4053, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error marshaling protobuf data")}
	ErrCantParseBoolean                 = apirest.APIerror{Code: 4055, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse string into boolean")}
	ErrCantParseHexString               = apirest.APIerror{Code: 4056, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse string into hex bytes")}
	ErrPageNotFound                     = apirest.APIerror{Code: 4057, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("page not found")}
	ErrCantParseDate                    = apirest.APIerror{Code: 4058, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse date")}
	ErrTransactionBatchTooLarge         = apirest.APIerror{Code: 4059, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("transaction batch too large")}
	ErrTransactionBatchEmpty            = apirest.APIerror{Code: 4060, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("transaction batch is empty")}
	ErrParamBucketInvalid               = apirest.APIerror{Code: 4061, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (bucket) invalid, must be hour or day")}
	ErrTooManyBucketsRequested          = apirest.APIerror{Code: 4062, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("too many time buckets requested, use a coarser bucket or narrow the time window with the (from) and (to) params")}
	ErrParamSortByInvalid               = apirest.APIerror{Code: 4063, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (sortBy) invalid")}
	ErrParamOrderInvalid                = apirest.APIerror{Code: 4064, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (order) invalid, must be asc or desc")}
	ErrVochainEmptyReply                = apirest.APIerror{Code: 5000, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain returned an empty reply")}
	ErrVochainSendTxFailed              = apirest.APIerror{Code: 5001, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain SendTx failed")}
	ErrVochainGetTxFailed               = apirest.APIerror{Code: 5002, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain GetTx failed")}
	ErrVochainReturnedErrorCode         = apirest.APIerror{Code: 5003, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain replied with error code")}
	ErrVochainReturnedInvalidElectionID = apirest.APIerror{Code: 5004, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain returned an invalid electionId after executing tx")}
	ErrMarshalingServerJSONFailed       = apirest.APIerror{Code: 5006, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("marshaling (server-side) JSON failed")}
	ErrCantFetchElection                = apirest.APIerror{Code: 5008, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch election")}
	ErrCantFetchTokenTransfers          = apirest.APIerror{Code: 5010, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch token transfers")}
	ErrCantFetchEnvelopeHeight          = apirest.APIerror{Code: 5011, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch envelope height")}
	ErrCantFetchEnvelope                = apirest.APIerror{Code: 5012, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch vote envelope")}
	ErrCantCheckTxType                  = apirest.APIerror{Code: 5013, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot check transaction type")}
	ErrCantABIEncodeResults             = apirest.APIerror{Code: 5014, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot abi.encode results")}
	ErrCantAddKeyToTree                 = apirest.APIerror{Code: 5017, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot add key to tree")}
	ErrCantGenerateFaucetPkg            = apirest.APIerror{Code: 5018, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot generate faucet package")}
	ErrCantEstimateBlockHeight          = apirest.APIerror{Code: 5019, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot estimate startDate block height")}
	ErrTxTypeMismatch                   = apirest.APIerror{Code: 5022, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("transaction type mismatch")}
	ErrElectionIsNil                    = apirest.APIerror{Code: 5023, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election is nil")}
	ErrElectionResultsNotYetAvailable   = apirest.APIerror{Code: 5024, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results are not yet available")}
	ErrElectionResultsIsNil             = apirest.APIerror{Code: 5025, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results is nil")}
	ErrElectionResultsMismatch          = apirest.APIerror{Code: 5026, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results don't match reported ones")}
	ErrCantCountVotes                   = apirest.APIerror{Code: 5029, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot count votes")}
	ErrVochainOverloaded                = apirest.APIerror{Code: 5030, HTTPstatus: apirest.HTTPstatusServiceUnavailable, Err: fmt.Errorf("vochain overloaded")}
	ErrIndexerQueryFailed               = apirest.APIerror{Code: 5033, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("indexer query failed")}
	ErrCantFetchTokenFees               = apirest.APIerror{Code: 5034, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch token fees")}
)
