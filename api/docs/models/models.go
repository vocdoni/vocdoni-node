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
