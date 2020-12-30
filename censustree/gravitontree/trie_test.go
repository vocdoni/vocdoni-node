package gravitontree

import (
	"bytes"
	"fmt"
	"testing"
)

func TestTree(t *testing.T) {
	censusSize := 1000
	storage := t.TempDir()
	tr1, err := NewTree("test1", storage)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < censusSize; i++ {
		if err = tr1.Add([]byte(fmt.Sprintf("number %d", i)), []byte(fmt.Sprintf("number %d", i))); err != nil {
			t.Fatal(err)
		}
	}
	root1 := tr1.Root()
	data, err := tr1.Dump(root1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("dumped data size is: %d bytes", len(data))

	tr2, err := NewTree("test2", storage)
	if err != nil {
		t.Fatal(err)
	}
	if err = tr2.ImportDump(data); err != nil {
		t.Fatal(err)
	}
	root2 := tr2.Root()
	if !bytes.Equal(root1, root2) {
		t.Errorf("roots are diferent but they should be equal (%x != %x)", root1, root2)
	}
	tr2.(*Tree).store.Commit()

	// Try closing the storage and creating the tree again
	tr2.(*Tree).store.Close()
	tr2, err = NewTree("test2", storage)
	if err != nil {
		t.Fatal(err)
	}

	// Get the size
	s, err := tr2.Size(nil)
	if err != nil {
		t.Errorf("cannot get te size of the tree after reopen: (%s)", err)
	}
	if s != int64(censusSize) {
		t.Errorf("Size is wrong (have %d, expexted %d)", s, censusSize)
	}

	// Check Root is still the same
	if !bytes.Equal(tr2.Root(), root2) {
		t.Fatalf("after closing and opening the tree, the root is diferent")
	}

	// Generate a proof on tr1 and check validity on snapshot and tr2
	proof1, err := tr1.GenProof([]byte("number 5"), []byte{})
	if err != nil {
		t.Error(err)
	}
	tr1s, err := tr1.Snapshot(root1)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := tr1s.CheckProof([]byte("number 5"), []byte{}, nil, proof1)
	if err != nil {
		t.Error(err)
	}
	if !valid {
		t.Errorf("proof is invalid on snapshot")
	}
	valid, err = tr2.CheckProof([]byte("number 5"), []byte{}, nil, proof1)
	if err != nil {
		t.Error(err)
	}
	if !valid {
		t.Errorf("proof is invalid on tree2")
	}

}
