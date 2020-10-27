package scrutinizer

import (
	"testing"

	"github.com/tendermint/go-amino"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func TestList(t *testing.T) {
	log.Init("info", "stdout")
	c := amino.NewCodec()
	state, err := vochain.NewState(t.TempDir(), c)
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		sc.addEntity(util.Hex2byte(t, util.RandomHex(20)), util.Hex2byte(t, util.RandomHex(32)))
	}

	entities := make(map[string]bool)
	last := ""
	iterations := 0
	for len(entities) < 100 {
		list, err := sc.EntityList(10, last)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) < 1 {
			t.Fatalf("list size is smaller than 1")
		}
		for _, e := range list {
			entities[e] = true
		}
		last = list[len(list)-1]
		iterations++
	}
	if iterations != 10 {
		t.Fatalf("expected  10 iterations, got %d", iterations)
	}
	t.Logf("got complete list of entities with %d iterations", iterations)
}
