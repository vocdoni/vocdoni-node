package gravitontree

import (
	"bytes"
	"fmt"
	"testing"
)

func TestTree(t *testing.T) {
	censusSize := 1089
	storage := t.TempDir()
	tr1, err := NewTree("test1", storage)
	if err != nil {
		t.Fatal(err)
	}
	keys := [][]byte{}
	values := [][]byte{}
	for i := 0; i < censusSize; i++ {
		keys = append(keys, []byte(fmt.Sprintf("number %d", i)))
		values = append(values, []byte(fmt.Sprintf("number %d value", i)))
	}
	i := 0
	for i < len(keys)-256 {
		failed, err := tr1.AddBatch(keys[i:i+256], values[i:i+256])
		if err != nil {
			t.Fatal(err)
		}
		if len(failed) > 0 {
			t.Fatalf("some census keys failed: %v", failed)
		}
		i += 256
	}
	if censusSize-i > 0 {
		failed, err := tr1.AddBatch(keys[i:], values[i:])
		if err != nil {
			t.Fatal(err)
		}
		if len(failed) > 0 {
			t.Fatalf("some census keys failed")
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
		t.Errorf("roots are different but they should be equal (%x != %x)", root1, root2)
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
		t.Fatalf("after closing and opening the tree, the root is different")
	}

	// Generate a proof on tr1 and check validity on snapshot and tr2
	proof1, err := tr1.GenProof([]byte("number 5"), []byte("number 5 value"))
	if err != nil {
		t.Error(err)
	}
	t.Logf("Proof Length: %d", len(proof1))
	tr1s, err := tr1.Snapshot(root1)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := tr1s.CheckProof([]byte("number 5"), []byte("number 5 value"), nil, proof1)
	if err != nil {
		t.Error(err)
	}
	if !valid {
		t.Errorf("proof is invalid on snapshot")
	}
	valid, err = tr2.CheckProof([]byte("number 5"), []byte("number 5 value"), nil, proof1)
	if err != nil {
		t.Error(err)
	}
	if !valid {
		t.Errorf("proof is invalid on tree2")
	}

}
