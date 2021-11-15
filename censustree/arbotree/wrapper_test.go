package arbotree

import (
	"testing"
)

func TestGenProof(t *testing.T) {
	storage := t.TempDir()
	tr1 := &Tree{}
	if err := tr1.Init("test1", storage); err != nil {
		t.Fatal(err)
	}

	var keys, values [][]byte
	for i := 0; i < 10; i++ {
		keys = append(keys, []byte{byte(i)})
		values = append(values, []byte{byte(i)})
		if err := tr1.Add([]byte{byte(i)}, []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 10; i++ {
		p, err := tr1.GenProof(keys[i])
		if err != nil {
			t.Fatal(err)
		}
		v, err := tr1.CheckProof(keys[i], values[i], tr1.Root(), p)
		if err != nil {
			t.Fatal(err)
		}
		if !v {
			t.Fatal("CheckProof failed")
		}
	}
}
