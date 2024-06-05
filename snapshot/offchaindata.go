package snapshot

import (
	"sync"

	"go.vocdoni.io/dvote/vochain/state"
)

// fnImportOffChainData holds the func that imports the offchain data
var fnImportOffChainData struct {
	sync.Mutex
	fn func(*state.State) error
}

// SetFnImportOffChainData sets the func that imports the offchain data
func SetFnImportOffChainData(fn func(*state.State) error) {
	fnImportOffChainData.Lock()
	defer fnImportOffChainData.Unlock()
	fnImportOffChainData.fn = fn
}

// FnImportOffChainData returns the func that imports the offchain data
func FnImportOffChainData() (fn func(*state.State) error) {
	fnImportOffChainData.Lock()
	defer fnImportOffChainData.Unlock()
	return fnImportOffChainData.fn
}
