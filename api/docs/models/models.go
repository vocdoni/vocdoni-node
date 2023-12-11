package dummycode

import (
	_ "go.vocdoni.io/proto/build/go/models"
	_ "google.golang.org/protobuf/proto"
)

// generate transaction models
//
//	@Success	200	{object}	models.Tx_Vote
//	@Success	200	{object}	models.Tx_NewProcess
//	@Success	200	{object}	models.Tx_Admin
//	@Success	200	{object}	models.Tx_SetProcess
//	@Success	200	{object}	models.Tx_RegisterKey
//	@Success	200	{object}	models.Tx_SendTokens
//	@Success	200	{object}	models.Tx_SetTransactionCosts
//	@Success	200	{object}	models.Tx_SetAccount
//	@Success	200	{object}	models.Tx_CollectFaucet
//	@Success	200	{object}	models.Tx_SetKeykeeper
func ChainTxHandler() {
}

// ElectionListByStatusHandler
//
//	Add multiple router on swagger generation has this bug https://github.com/swaggo/swag/issues/1267
//
//	@Summary		List organization elections by status
//	@Description	List the elections of an organization by status
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			organizationID	path		string	true	"Specific organizationID"
//	@Param			status			path		string	true	"Status of the election"	Enums(ready, paused, canceled, ended, results)
//	@Param			page			path		number	true	"Define de page number"
//	@Success		200				{object}	object{elections=[]api.ElectionSummary}
//	@Router			/accounts/{organizationID}/elections/status/{status}/page/{page} [get]
func ElectionListByStatusHandler() {
}

// CensusPublishRootHandler
//
//	Add multiple router on swagger generation has this bug https://github.com/swaggo/swag/issues/1267
//
//	@Summary				Publish census at root
//	@Description.markdown	censusPublishHandler
//	@Tags					Censuses
//	@Accept					json
//	@Produce				json
//	@Security				BasicAuth
//	@Success				200			{object}	object{census=object{censusID=string,uri=string}}	"It return published censusID and the ipfs uri where its uploaded"
//	@Param					censusID	path		string												true	"Census id"
//	@Param					root		path		string												true	"Specific root where to publish the census. Not required"
//	@Router					/censuses/{censusID}/publish/{root} [post]
func CensusPublishRootHandler() {
}
