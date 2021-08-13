package factory

import (
	"encoding/hex"
	"testing"
)

func TestNewCensusTree(t *testing.T) {
	tree0, err := NewCensusTree(TreeTypeArboBlake2b, "test", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := tree0.Add([]byte("test"), []byte("test")); err != nil {
		t.Fatal(err)
	}
	root0, err := tree0.Root()
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(root0) !=
		"8d844f0a09036743f42901c40bde5731eb0f8023fb3d6070dd65e9c0fca6ab72" {
		t.Fatal("tree0 (TreeTypeArboBlake2b) root different from expected value")
	}

	tree1, err := NewCensusTree(TreeTypeArboPoseidon, "test", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := tree1.Add([]byte("test"), []byte("test")); err != nil {
		t.Fatal(err)
	}
	root1, err := tree1.Root()
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(root1) !=
		"482ac342dfc14bb69be2b2ec7ba3fed1fbfe5cc7d388ce972d6251da5bf2280e" {
		t.Fatal("tree1 (TreeTypeArboPoseidon) root different from expected value")
	}

	tree2, err := NewCensusTree(TreeTypeGraviton, "test", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := tree2.Add([]byte("test"), []byte("test")); err != nil {
		t.Fatal(err)
	}
	root2, err := tree2.Root()
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(root2) !=
		"735c8c1ea64ee125f5f1b26a21d8a32ca32e704e402c726f8e6917988af1dadf" {
		t.Fatal("tree2 (TreeTypeGraviton) root different from expected value")
	}
}
