package genesis

import (
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestSaveAsAndLoad(t *testing.T) {
	file := filepath.Join(t.TempDir(), "genesis.json")
	g := HardcodedForNetwork("test")
	t.Logf("%+v", g)
	err := g.SaveAs(file)
	qt.Assert(t, err, qt.IsNil)

	f, err := LoadFromFile(file)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("%+v", f)
	t.Logf("%+v", g.ConsensusParams)
	t.Logf("%+v", f.ConsensusParams)
	qt.Assert(t, g.Hash(), qt.DeepEquals, f.Hash())
}

func TestAvailableNetworks(t *testing.T) {
	nets := AvailableNetworks()
	for _, net := range nets {
		g := HardcodedForNetwork(net)
		qt.Assert(t, g, qt.IsNotNil)

		gc := HardcodedForChainID(g.ChainID)
		qt.Assert(t, g, qt.DeepEquals, gc)
	}
}
