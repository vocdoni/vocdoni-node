package api

// You need to run "go generate ./..." after making changes to this file.
// Also, error codes depend on the line order due to iota,
// so don't interleave new errors, append them at the end.
//go:generate go run golang.org/x/tools/cmd/stringer@v0.5.0 -linecomment -type APIError

// APIError is an int that satisfies the error interface.
// Error() returns a human-readable description of the error.
//
// Error codes in the 4001-4999 range are the user's fault,
// and error codes 5001-5999 are the server's fault, mimicking HTTP.
type APIError int

const (
	ErrAddressMalformed           APIError = 4000 + iota // address malformed
	ErrDstAddressMalformed                               // destination address malformed
	ErrDstAccountUnknown                                 // destination account is unknown
	ErrAccountNotFound                                   // account not found
	ErrAccountAlreadyExists                              // account already exists
	ErrTreasurerNotFound                                 // treasurer account not found
	ErrOrgNotFound                                       // organization not found
	ErrTransactionNotFound                               // transaction hash not found
	ErrBlockNotFound                                     // block not found
	ErrMetadataProvidedButNoURI                          // metadata provided but no metadata URI found in transaction
	ErrMetadataURINotMatchContent                        // metadata URI does not match metadata content
	ErrMarshalingJSONFailed                              // marshaling JSON failed
	ErrFileSizeTooBig                                    // file size exceeds the maximum allowed
	ErrCantParseOrgID                                    // cannot parse organizationID
	ErrCantParseAccountID                                // cannot parse accountID
	ErrCantParseBearerToken                              // cannot parse bearer token
	ErrCantParseDataAsJSON                               // cannot parse data as JSON
	ErrCantParseElectionID                               // cannot parse electionID
	ErrCantParseMetadataAsJSON                           // cannot parse metadata (invalid format)
	ErrCantParsePageNumber                               // cannot parse page number
	ErrCantParsePayloadAsJSON                            // cannot parse payload as JSON
	ErrCantParseVoteID                                   // cannot parse voteID
	ErrCantExtractMetadataURI                            // cannot extract metadata URI
	ErrVoteIDMalformed                                   // voteID is malformed
	ErrVoteNotRegistered                                 // vote not registered
	ErrCensusIDLengthInvalid                             // censusID length is wrong
	ErrCensusRootIsNil                                   // census root is nil
	ErrCensusTypeUnknown                                 // census type is unknown
	ErrCensusTypeMismatch                                // census type mismatch
	ErrCensusIndexedFlagMismatch                         // census indexed flag mismatch
	ErrCensusRootHashMismatch                            // census root hash mismatch after importing dump
	ErrParamStatusMissing                                // parameter (status) missing or invalid
	ErrParamParticipantsMissing                          // parameter (participants) missing
	ErrParamParticipantsTooBig                           // parameter (participants) exceeds max length per call
	ErrParamDumpOrRootMissing                            // parameter (dump or root) missing
	ErrParamKeyOrProofMissing                            // parameter (key or proof) missing
	ErrParamRootInvalid                                  // parameter (root) invalid
	ErrParamNetworkInvalid                               // invalid network
	ErrParamToInvalid                                    // invalid address
	ErrParticipantKeyMissing                             // missing participant key
	ErrIndexedCensusCantUseWeight                        // indexed census cannot use weight
	ErrWalletNotFound                                    // wallet not found
	ErrWalletPrivKeyAlreadyExists                        // wallet private key already exists
	ErrElectionEndDateInThePast                          // election end date cannot be in the past
	ErrElectionEndDateBeforeStart                        // election end date must be after start date
)

const (
	ErrVochainEmptyReply                APIError = 5000 + iota // vochain returned an empty reply
	ErrVochainSendTxFailed                                     // vochain SendTx failed
	ErrVochainGetTxFailed                                      // vochain GetTx failed
	ErrVochainReturnedErrorCode                                // vochain replied with error code
	ErrVochainReturnedInvalidElectionID                        // vochain returned an invalid electionID after executing tx
	ErrVochainReturnedWrongMetadataCID                         // vochain returned an unexpected metadata CID after executing tx
	ErrMarshalingServerJSONFailed                              // marshaling (server-side) JSON failed
	ErrCantFetchElectionList                                   // cannot fetch election list
	ErrCantFetchElection                                       // cannot fetch election
	ErrCantFetchElectionResults                                // cannot fetch election results
	ErrCantFetchTokenTransfers                                 // cannot fetch token transfers
	ErrCantFetchEnvelopeHeight                                 // cannot fetch envelope height
	ErrCantFetchEnvelope                                       // cannot fetch vote envelope
	ErrCantCheckTxType                                         // cannot check transaction type
	ErrCantABIEncodeResults                                    // cannot abi.encode results
	ErrCantComputeKeyHash                                      // cannot compute key hash
	ErrCantAddKeyAndValueToTree                                // cannot add key and value to tree
	ErrCantAddKeyToTree                                        // cannot add key to tree
	ErrCantGenerateFaucetPkg                                   // cannot generate faucet package
	ErrCantEstimateBlockHeight                                 // cannot estimate startDate block height
	ErrCantMarshalMetadata                                     // cannot marshal metadata
	ErrCantPublishMetadata                                     // cannot publish metadata file
	ErrTxTypeMismatch                                          // transaction type mismatch
	ErrElectionIsNil                                           // election is nil
	ErrElectionResultsNotYetAvailable                          // election results are not yet available
	ErrElectionResultsIsNil                                    // election results is nil
	ErrElectionResultsMismatch                                 // election results don't match reported ones
	ErrMissingModulesForHandler                                // missing modules attached for enabling handler
	ErrHandlerUnknown                                          // handler unknown
	ErrHTTPRouterIsNil                                         // httprouter is nil
	ErrBaseRouteInvalid                                        // base route must start with /
)

func (e APIError) Error() string {
	return e.String()
}
