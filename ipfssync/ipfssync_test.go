package ipfssync

// import (
// 	"bytes"
// 	"testing"
//
// 	"go.vocdoni.io/dvote/log"
// 	"go.vocdoni.io/dvote/test/testcommon"
// 	"go.vocdoni.io/dvote/util"
// 	"go.vocdoni.io/proto/build/go/models"
// )
//
// func TestInternalPins(t *testing.T) {
// 	server := testcommon.DvoteAPIServer{}
// 	server.Start(t, "file")
// 	log.Init("debug", "stdout")
// 	t.Log("blah")
// 	is := NewIPFSsync(t.TempDir(), "test1", util.RandomHex(32), "libp2p", server.Storage)
// 	t.Log("so new")
// 	is.Start()
// 	t.Log("after start")
//
// 	// Add three pins
// 	var pins []*models.IpfsPin
// 	pins = append(pins, &models.IpfsPin{Uri: "/ipld/QmbpdFgAQXosfkxyX7LSrV6Cx9viL8RrZgfH2Zy6zB89Ge"})
// 	pins = append(pins, &models.IpfsPin{Uri: "/ipld/QmNrowfEhPbn1EtNzF5gXoYUb9siLVURsJQVV7hqybhmgH"})
// 	pins = append(pins, &models.IpfsPin{Uri: "/ipld/QmPXJf5Z5Sjd4CGcV1QydYq4xJhJXdH7jBW7UHkw1EFNBV"})
// 	if err := is.addPins(pins); err != nil {
// 		t.Fatal(err)
// 	}
//
// 	// Query from our last hash, no list should be provided
// 	listp, err := is.listPins(is.hashTree.Hash())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if len(listp) > 0 {
// 		t.Errorf("list pins error: expected 0, got %d", len(listp))
// 	}
//
// 	// Query from a nil hash, the full list should be provided
// 	listp, err = is.listPins(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if len(listp) != len(pins) {
// 		t.Fatalf("pin list is not correct: %v", listp)
// 	}
// 	for _, p := range pins {
// 		found := false
// 		for _, p2 := range listp {
// 			if p.Uri == p2.Uri {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			t.Fatal("pin list does not match")
// 		}
// 	}
//
// 	// Add a new pin and check the diff
// 	oldHash := is.hashTree.Hash()
// 	pins = []*models.IpfsPin{}
// 	p := &models.IpfsPin{Uri: "/ipld/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"}
// 	if err := is.addPins(append(pins, p)); err != nil {
// 		t.Fatal(err)
// 	}
// 	if bytes.Equal(is.hashTree.Hash(), oldHash) {
// 		t.Errorf("hash has not changed")
// 	}
//
// 	listp, err = is.listPins(oldHash)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if len(listp) != 1 {
// 		t.Errorf("pint list is not correct, expected 1, got %d", len(listp))
// 	}
// 	if listp[0].Uri != p.Uri {
// 		t.Errorf("pin content does not match")
// 	}
// }
