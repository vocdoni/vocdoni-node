//nolint:lll
package api

import "go.vocdoni.io/dvote/httprouter/apirest"

// APIerror satisfies the error interface.
// Error() returns a human-readable description of the error.
//
// Error codes in the 4001-4999 range are the user's fault,
// and error codes 5001-5999 are the server's fault, mimicking HTTP.
//
// Do note that HTTPstatus 204 No Content implies the response body will be empty,
// so the Code and Message will actually be discarded, never sent to the client
var (
	ErrAddressMalformed           = apirest.APIerror{Code: 4000, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "address malformed"}
	ErrDstAddressMalformed        = apirest.APIerror{Code: 4001, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "destination address malformed"}
	ErrDstAccountUnknown          = apirest.APIerror{Code: 4002, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "destination account is unknown"}
	ErrAccountNotFound            = apirest.APIerror{Code: 4003, HTTPstatus: apirest.HTTPstatusNotFound, Message: "account not found"}
	ErrAccountAlreadyExists       = apirest.APIerror{Code: 4004, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "account already exists"}
	ErrTreasurerNotFound          = apirest.APIerror{Code: 4005, HTTPstatus: apirest.HTTPstatusNotFound, Message: "treasurer account not found"}
	ErrOrgNotFound                = apirest.APIerror{Code: 4006, HTTPstatus: apirest.HTTPstatusNotFound, Message: "organization not found"}
	ErrTransactionNotFound        = apirest.APIerror{Code: 4007, HTTPstatus: apirest.HTTPstatusNoContent, Message: "transaction hash not found"}
	ErrBlockNotFound              = apirest.APIerror{Code: 4008, HTTPstatus: apirest.HTTPstatusNotFound, Message: "block not found"}
	ErrMetadataProvidedButNoURI   = apirest.APIerror{Code: 4009, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "metadata provided but no metadata URI found in transaction"}
	ErrMetadataURINotMatchContent = apirest.APIerror{Code: 4010, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "metadata URI does not match metadata content"}
	ErrMarshalingJSONFailed       = apirest.APIerror{Code: 4011, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "marshaling JSON failed"}
	ErrFileSizeTooBig             = apirest.APIerror{Code: 4012, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "file size exceeds the maximum allowed"}
	ErrCantParseOrgID             = apirest.APIerror{Code: 4013, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse organizationID"}
	ErrCantParseAccountID         = apirest.APIerror{Code: 4014, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse accountID"}
	ErrCantParseBearerToken       = apirest.APIerror{Code: 4015, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse bearer token"}
	ErrCantParseDataAsJSON        = apirest.APIerror{Code: 4016, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse data as JSON"}
	ErrCantParseElectionID        = apirest.APIerror{Code: 4017, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse electionID"}
	ErrCantParseMetadataAsJSON    = apirest.APIerror{Code: 4018, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse metadata (invalid format)"}
	ErrCantParsePageNumber        = apirest.APIerror{Code: 4019, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse page number"}
	ErrCantParsePayloadAsJSON     = apirest.APIerror{Code: 4020, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse payload as JSON"}
	ErrCantParseVoteID            = apirest.APIerror{Code: 4021, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot parse voteID"}
	ErrCantExtractMetadataURI     = apirest.APIerror{Code: 4022, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "cannot extract metadata URI"}
	ErrVoteIDMalformed            = apirest.APIerror{Code: 4023, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "voteID is malformed"}
	ErrVoteNotFound               = apirest.APIerror{Code: 4024, HTTPstatus: apirest.HTTPstatusNotFound, Message: "vote not found"}
	ErrCensusIDLengthInvalid      = apirest.APIerror{Code: 4025, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "censusID length is wrong"}
	ErrCensusRootIsNil            = apirest.APIerror{Code: 4026, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "census root is nil"}
	ErrCensusTypeUnknown          = apirest.APIerror{Code: 4027, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "census type is unknown"}
	ErrCensusTypeMismatch         = apirest.APIerror{Code: 4028, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "census type mismatch"}
	ErrCensusIndexedFlagMismatch  = apirest.APIerror{Code: 4029, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "census indexed flag mismatch"}
	ErrCensusRootHashMismatch     = apirest.APIerror{Code: 4030, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "census root hash mismatch after importing dump"}
	ErrParamStatusMissing         = apirest.APIerror{Code: 4031, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "parameter (status) missing or invalid"}
	ErrParamParticipantsMissing   = apirest.APIerror{Code: 4032, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "parameter (participants) missing"}
	ErrParamParticipantsTooBig    = apirest.APIerror{Code: 4033, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "parameter (participants) exceeds max length per call"}
	ErrParamDumpOrRootMissing     = apirest.APIerror{Code: 4034, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "parameter (dump or root) missing"}
	ErrParamKeyOrProofMissing     = apirest.APIerror{Code: 4035, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "parameter (key or proof) missing"}
	ErrParamRootInvalid           = apirest.APIerror{Code: 4036, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "parameter (root) invalid"}
	ErrParamNetworkInvalid        = apirest.APIerror{Code: 4037, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "invalid network"}
	ErrParamToInvalid             = apirest.APIerror{Code: 4038, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "invalid address"}
	ErrParticipantKeyMissing      = apirest.APIerror{Code: 4039, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "missing participant key"}
	ErrIndexedCensusCantUseWeight = apirest.APIerror{Code: 4040, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "indexed census cannot use weight"}
	ErrWalletNotFound             = apirest.APIerror{Code: 4041, HTTPstatus: apirest.HTTPstatusNotFound, Message: "wallet not found"}
	ErrWalletPrivKeyAlreadyExists = apirest.APIerror{Code: 4042, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "wallet private key already exists"}
	ErrElectionEndDateInThePast   = apirest.APIerror{Code: 4043, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "election end date cannot be in the past"}
	ErrElectionEndDateBeforeStart = apirest.APIerror{Code: 4044, HTTPstatus: apirest.HTTPstatusBadRequest, Message: "election end date must be after start date"}
	ErrElectionNotFound           = apirest.APIerror{Code: 4045, HTTPstatus: apirest.HTTPstatusNotFound, Message: "election not found"}
	ErrCensusNotFound             = apirest.APIerror{Code: 4046, HTTPstatus: apirest.HTTPstatusNotFound, Message: "census not found"}

	ErrVochainEmptyReply                = apirest.APIerror{Code: 5000, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "vochain returned an empty reply"}
	ErrVochainSendTxFailed              = apirest.APIerror{Code: 5001, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "vochain SendTx failed"}
	ErrVochainGetTxFailed               = apirest.APIerror{Code: 5002, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "vochain GetTx failed"}
	ErrVochainReturnedErrorCode         = apirest.APIerror{Code: 5003, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "vochain replied with error code"}
	ErrVochainReturnedInvalidElectionID = apirest.APIerror{Code: 5004, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "vochain returned an invalid electionID after executing tx"}
	ErrVochainReturnedWrongMetadataCID  = apirest.APIerror{Code: 5005, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "vochain returned an unexpected metadata CID after executing tx"}
	ErrMarshalingServerJSONFailed       = apirest.APIerror{Code: 5006, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "marshaling (server-side) JSON failed"}
	ErrCantFetchElectionList            = apirest.APIerror{Code: 5007, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot fetch election list"}
	ErrCantFetchElection                = apirest.APIerror{Code: 5008, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot fetch election"}
	ErrCantFetchElectionResults         = apirest.APIerror{Code: 5009, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot fetch election results"}
	ErrCantFetchTokenTransfers          = apirest.APIerror{Code: 5010, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot fetch token transfers"}
	ErrCantFetchEnvelopeHeight          = apirest.APIerror{Code: 5011, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot fetch envelope height"}
	ErrCantFetchEnvelope                = apirest.APIerror{Code: 5012, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot fetch vote envelope"}
	ErrCantCheckTxType                  = apirest.APIerror{Code: 5013, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot check transaction type"}
	ErrCantABIEncodeResults             = apirest.APIerror{Code: 5014, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot abi.encode results"}
	ErrCantComputeKeyHash               = apirest.APIerror{Code: 5015, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot compute key hash"}
	ErrCantAddKeyAndValueToTree         = apirest.APIerror{Code: 5016, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot add key and value to tree"}
	ErrCantAddKeyToTree                 = apirest.APIerror{Code: 5017, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot add key to tree"}
	ErrCantGenerateFaucetPkg            = apirest.APIerror{Code: 5018, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot generate faucet package"}
	ErrCantEstimateBlockHeight          = apirest.APIerror{Code: 5019, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot estimate startDate block height"}
	ErrCantMarshalMetadata              = apirest.APIerror{Code: 5020, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot marshal metadata"}
	ErrCantPublishMetadata              = apirest.APIerror{Code: 5021, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "cannot publish metadata file"}
	ErrTxTypeMismatch                   = apirest.APIerror{Code: 5022, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "transaction type mismatch"}
	ErrElectionIsNil                    = apirest.APIerror{Code: 5023, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "election is nil"}
	ErrElectionResultsNotYetAvailable   = apirest.APIerror{Code: 5024, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "election results are not yet available"}
	ErrElectionResultsIsNil             = apirest.APIerror{Code: 5025, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "election results is nil"}
	ErrElectionResultsMismatch          = apirest.APIerror{Code: 5026, HTTPstatus: apirest.HTTPstatusInternalErr, Message: "election results don't match reported ones"}
)
