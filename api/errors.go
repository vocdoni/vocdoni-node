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
	ErrMetadataProvidedButNoURI         = apirest.APIerror{Code: 4009, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("metadata provided but no metadata URI found in transaction")}
	ErrMetadataURINotMatchContent       = apirest.APIerror{Code: 4010, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("metadata URI does not match metadata content")}
	ErrMarshalingJSONFailed             = apirest.APIerror{Code: 4011, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("marshaling JSON failed")}
	ErrFileSizeTooBig                   = apirest.APIerror{Code: 4012, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("file size exceeds the maximum allowed")}
	ErrCantParseOrgID                   = apirest.APIerror{Code: 4013, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse organizationID")}
	ErrCantParseAccountID               = apirest.APIerror{Code: 4014, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse accountID")}
	ErrCantParseBearerToken             = apirest.APIerror{Code: 4015, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse bearer token")}
	ErrCantParseDataAsJSON              = apirest.APIerror{Code: 4016, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse data as JSON")}
	ErrCantParseElectionID              = apirest.APIerror{Code: 4017, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse electionID")}
	ErrCantParseMetadataAsJSON          = apirest.APIerror{Code: 4018, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse metadata (invalid format)")}
	ErrCantParsePageNumber              = apirest.APIerror{Code: 4019, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse page number")}
	ErrCantParsePayloadAsJSON           = apirest.APIerror{Code: 4020, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse payload as JSON")}
	ErrCantParseVoteID                  = apirest.APIerror{Code: 4021, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse voteID")}
	ErrCantExtractMetadataURI           = apirest.APIerror{Code: 4022, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot extract metadata URI")}
	ErrVoteIDMalformed                  = apirest.APIerror{Code: 4023, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("voteID is malformed")}
	ErrVoteNotFound                     = apirest.APIerror{Code: 4024, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("vote not found")}
	ErrCensusIDLengthInvalid            = apirest.APIerror{Code: 4025, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("censusID length is wrong")}
	ErrCensusRootIsNil                  = apirest.APIerror{Code: 4026, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census root is nil")}
	ErrCensusTypeUnknown                = apirest.APIerror{Code: 4027, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census type is unknown")}
	ErrCensusTypeMismatch               = apirest.APIerror{Code: 4028, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census type mismatch")}
	ErrCensusIndexedFlagMismatch        = apirest.APIerror{Code: 4029, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census indexed flag mismatch")}
	ErrCensusRootHashMismatch           = apirest.APIerror{Code: 4030, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("census root hash mismatch after importing dump")}
	ErrParamStatusMissing               = apirest.APIerror{Code: 4031, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (status) missing or invalid")}
	ErrParamParticipantsMissing         = apirest.APIerror{Code: 4032, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (participants) missing")}
	ErrParamParticipantsTooBig          = apirest.APIerror{Code: 4033, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (participants) exceeds max length per call")}
	ErrParamDumpOrRootMissing           = apirest.APIerror{Code: 4034, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (dump or root) missing")}
	ErrParamKeyOrProofMissing           = apirest.APIerror{Code: 4035, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (key or proof) missing")}
	ErrParamRootInvalid                 = apirest.APIerror{Code: 4036, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (root) invalid")}
	ErrParamNetworkInvalid              = apirest.APIerror{Code: 4037, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid network")}
	ErrParamToInvalid                   = apirest.APIerror{Code: 4038, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid address")}
	ErrParticipantKeyMissing            = apirest.APIerror{Code: 4039, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("missing participant key")}
	ErrIndexedCensusCantUseWeight       = apirest.APIerror{Code: 4040, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("indexed census cannot use weight")}
	ErrWalletNotFound                   = apirest.APIerror{Code: 4041, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("wallet not found")}
	ErrWalletPrivKeyAlreadyExists       = apirest.APIerror{Code: 4042, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("wallet private key already exists")}
	ErrElectionEndDateInThePast         = apirest.APIerror{Code: 4043, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("election end date cannot be in the past")}
	ErrElectionEndDateBeforeStart       = apirest.APIerror{Code: 4044, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("election end date must be after start date")}
	ErrElectionNotFound                 = apirest.APIerror{Code: 4045, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("election not found")}
	ErrCensusNotFound                   = apirest.APIerror{Code: 4046, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("census not found")}
	ErrNoElectionKeys                   = apirest.APIerror{Code: 4047, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("no election keys available")}
	ErrMissingParameter                 = apirest.APIerror{Code: 4048, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("one or more parameters are missing")}
	ErrKeyNotFoundInCensus              = apirest.APIerror{Code: 4049, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("key not found in census")}
	ErrInvalidStatus                    = apirest.APIerror{Code: 4050, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid status")}
	ErrInvalidCensusKeyLength           = apirest.APIerror{Code: 4051, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid census key length")}
	ErrUnmarshalingServerProto          = apirest.APIerror{Code: 4052, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error unmarshaling protobuf data")}
	ErrMarshalingServerProto            = apirest.APIerror{Code: 4053, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error marshaling protobuf data")}
	ErrSIKNotFound                      = apirest.APIerror{Code: 4054, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("SIK not found")}
	ErrVochainEmptyReply                = apirest.APIerror{Code: 5000, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain returned an empty reply")}
	ErrVochainSendTxFailed              = apirest.APIerror{Code: 5001, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain SendTx failed")}
	ErrVochainGetTxFailed               = apirest.APIerror{Code: 5002, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain GetTx failed")}
	ErrVochainReturnedErrorCode         = apirest.APIerror{Code: 5003, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain replied with error code")}
	ErrVochainReturnedInvalidElectionID = apirest.APIerror{Code: 5004, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain returned an invalid electionID after executing tx")}
	ErrVochainReturnedWrongMetadataCID  = apirest.APIerror{Code: 5005, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain returned an unexpected metadata CID after executing tx")}
	ErrMarshalingServerJSONFailed       = apirest.APIerror{Code: 5006, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("marshaling (server-side) JSON failed")}
	ErrCantFetchElectionList            = apirest.APIerror{Code: 5007, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch election list")}
	ErrCantFetchElection                = apirest.APIerror{Code: 5008, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch election")}
	ErrCantFetchElectionResults         = apirest.APIerror{Code: 5009, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch election results")} // unused as of 2023-06-28
	ErrCantFetchTokenTransfers          = apirest.APIerror{Code: 5010, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch token transfers")}
	ErrCantFetchEnvelopeHeight          = apirest.APIerror{Code: 5011, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch envelope height")}
	ErrCantFetchEnvelope                = apirest.APIerror{Code: 5012, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot fetch vote envelope")}
	ErrCantCheckTxType                  = apirest.APIerror{Code: 5013, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot check transaction type")}
	ErrCantABIEncodeResults             = apirest.APIerror{Code: 5014, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot abi.encode results")}
	ErrCantComputeKeyHash               = apirest.APIerror{Code: 5015, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot compute key hash")}
	ErrCantAddKeyAndValueToTree         = apirest.APIerror{Code: 5016, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot add key and value to tree")}
	ErrCantAddKeyToTree                 = apirest.APIerror{Code: 5017, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot add key to tree")}
	ErrCantGenerateFaucetPkg            = apirest.APIerror{Code: 5018, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot generate faucet package")}
	ErrCantEstimateBlockHeight          = apirest.APIerror{Code: 5019, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot estimate startDate block height")}
	ErrCantMarshalMetadata              = apirest.APIerror{Code: 5020, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot marshal metadata")}
	ErrCantPublishMetadata              = apirest.APIerror{Code: 5021, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot publish metadata file")}
	ErrTxTypeMismatch                   = apirest.APIerror{Code: 5022, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("transaction type mismatch")}
	ErrElectionIsNil                    = apirest.APIerror{Code: 5023, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election is nil")}
	ErrElectionResultsNotYetAvailable   = apirest.APIerror{Code: 5024, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results are not yet available")}
	ErrElectionResultsIsNil             = apirest.APIerror{Code: 5025, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results is nil")}
	ErrElectionResultsMismatch          = apirest.APIerror{Code: 5026, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results don't match reported ones")}
	ErrCantGetCircomSiblings            = apirest.APIerror{Code: 5027, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot get circom siblings")}
	ErrCensusProofVerificationFailed    = apirest.APIerror{Code: 5028, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("census proof verification failed")}
	ErrCantCountVotes                   = apirest.APIerror{Code: 5029, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot count votes")}
	ErrVochainOverloaded                = apirest.APIerror{Code: 5030, HTTPstatus: apirest.HTTPstatusServiceUnavailable, Err: fmt.Errorf("vochain overloaded")}
	ErrGettingSIK                       = apirest.APIerror{Code: 5031, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error getting SIK")}
)
