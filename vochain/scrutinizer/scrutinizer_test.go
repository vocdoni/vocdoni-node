package scrutinizer

import (
	"testing"

	"github.com/tendermint/go-amino"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func TestEntityList(t *testing.T) {
	testEntityList(t, 2)
	testEntityList(t, 100)
	testEntityList(t, 155)
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
	}
	if len(entities) < entityCount {
		t.Fatalf("expected %d entityes, got %d", entityCount, len(entities))
	}
}

func TestProcessList(t *testing.T) {
	testProcessList(t, 10)
	testProcessList(t, 20)
	testProcessList(t, 155)
}

func testProcessList(t *testing.T, procsCount int) {
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

	// Add 10 entities and process for storing random content
	eid := ""
	for i := 0; i < 10; i++ {
		eid = util.RandomHex(20)
		sc.addEntity(util.Hex2byte(t, eid), util.Hex2byte(t, util.RandomHex(32)))
	}

	// For a entity, add 25 processes (this will be the queried entity)
	eidTest := util.Hex2byte(t, util.RandomHex(20))
	for i := 0; i < procsCount; i++ {
		sc.addEntity(eidTest, util.Hex2byte(t, util.RandomHex(32)))
	}

	procs := make(map[string]bool)
	last := []byte{}
	var list [][]byte
	for len(procs) < procsCount {
		list, err = sc.ProcessList(eidTest, last, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, p := range list {
			if procs[string(p)] {
				t.Fatalf("found duplicated entity: %x", p)
			}
			procs[string(p)] = true
		}
		last = list[len(list)-1]
	}
	if len(procs) != procsCount {
		t.Fatalf("expected %d processes, got %d", procsCount, len(procs))
	}
}
