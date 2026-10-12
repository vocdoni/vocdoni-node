package genesis

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestSaveAsAndLoad(t *testing.T) {
	file := filepath.Join(t.TempDir(), "genesis.json")
	appState, err := json.Marshal(&AppState{TxCost: DefaultTransactionCosts(), MaxElectionSize: 100})
	qt.Assert(t, err, qt.IsNil)
	g := &Doc{
		GenesisTime:   time.Now().UTC().Truncate(time.Second),
		ChainID:       "test-chain-1",
		InitialHeight: 0,
		AppState:      appState,
	}
	qt.Assert(t, g.SaveAs(file), qt.IsNil)

	f, err := LoadFromFile(file)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, f.Hash(), qt.DeepEquals, g.Hash())

	_, err = LoadFromFile(filepath.Join(t.TempDir(), "missing.json"))
	qt.Assert(t, err, qt.IsNotNil)
}
