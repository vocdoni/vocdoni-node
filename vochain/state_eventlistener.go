package vochain

import "go.vocdoni.io/proto/build/go/models"

// EventListener is an interface used for executing custom functions during the
// events of the block creation process.
// The order in which events are executed is: Rollback, OnVote, Onprocess, On..., Commit.
// The process is concurrency safe, meaning that there cannot be two sequences
// happening in parallel.
//
// If Commit() returns ErrHaltVochain, the error is considered a consensus
// failure and the blockchain will halt.
//
// If OncProcessResults() returns an error, the results transaction won't be included
// in the blockchain. This event relays on the event handlers to decide if results are
// valid or not since the Vochain State do not validate results.
type EventListener interface {
	OnVote(vote *models.Vote, voterID VoterID, txIndex int32)
	OnNewTx(hash []byte, blockHeight uint32, txIndex int32)
	OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32)
	OnProcessStatusChange(pid []byte, status models.ProcessStatus, txIndex int32)
	OnCancel(pid []byte, txIndex int32)
	OnProcessKeys(pid []byte, encryptionPub string, txIndex int32)
	OnRevealKeys(pid []byte, encryptionPriv string, txIndex int32)
	OnProcessResults(pid []byte, results *models.ProcessResult, txIndex int32)
	OnProcessesStart(pids [][]byte)
	OnSetAccount(addr []byte, account *Account)
	OnTransferTokens(from, to []byte, amount uint64)
	Commit(height uint32) (err error)
	Rollback()
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (v *State) AddEventListener(l EventListener) {
	v.eventListeners = append(v.eventListeners, l)
}
