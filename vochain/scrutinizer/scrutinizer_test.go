package scrutinizer

import (
	"testing"

	"github.com/tendermint/go-amino"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func TestList(t *testing.T) {
	testEntityList(t, 2)
	testEntityList(t, 100)
	testEntityList(t, 110)
}

func testEntityList(t *testing.T, entityCount int) {
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
	eid := ""
	for i := 0; i < entityCount; i++ {
		eid = util.RandomHex(20)
		sc.addEntity(util.Hex2byte(t, eid), util.Hex2byte(t, util.RandomHex(32)))
	}

	entities := make(map[string]bool)
	last := ""
	iterations := 0
	if sc.entityCount != int64(entityCount) {
		t.Fatalf("entity count is wrong, got %d expected %d", sc.entityCount, entityCount)
	}
	var list []string
	for len(entities) <= entityCount {
		list, err = sc.EntityList(10, last)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, e := range list {
			if entities[e] {
				t.Fatalf("found duplicated entity: %s", e)
			}
			entities[e] = true
		}
		last = list[len(list)-1]
		iterations++
	}
	if iterations != (entityCount/10) && entityCount/10 > 0 {
		t.Fatalf("expected %d iterations, got %d", (entityCount / 10), iterations)
	}
	if len(entities) < entityCount {
		t.Fatalf("expected %d entityes, got %d", entityCount, len(entities))
	}
	t.Logf("got complete list of entities with %d iterations", iterations)

}
