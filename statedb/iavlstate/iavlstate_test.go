package iavlstate

import (
	"fmt"
	"strings"
	"testing"
)

func TestState(t *testing.T) {
	t.Parallel()

	s := &IavlState{}
	if err := s.Init(t.TempDir(), "disk"); err != nil {
		t.Fatal(err)
	}

	// Lets create two state trees
	if err := s.AddTree("t1"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTree("t2"); err != nil {
		t.Fatal(err)
	}

	// No previous version available, but this should be supported
	if err := s.LoadVersion(-1); err != nil {
		t.Fatal(err)
	}

	// Add 10 identical entries to both trees
	for i := 0; i < 10; i++ {
		s.Tree("t1").Add([]byte(fmt.Sprintf("%d", i)), []byte(fmt.Sprintf("number %d", i)))
		s.Tree("t2").Add([]byte(fmt.Sprintf("%d", i)), []byte(fmt.Sprintf("number %d", i)))
	}

	// Compare hash
	if string(s.Tree("t1").Hash()) != string(s.Tree("t2").Hash()) {
		t.Errorf("hash between two identical trees tree is different")
	}

	// Commit tree state (save to h1) (V1)
	if h, err := s.Commit(); err != nil {
		t.Error(err)
	} else {
		t.Logf("[v1] version %d hash %x", s.Version(), h)
	}

	// Add 10 different entries to both trees
	for i := 0; i < 10; i++ {
		s.Tree("t1").Add([]byte(fmt.Sprintf("%d", i+10)), []byte(fmt.Sprintf("number %d", i+10)))
		s.Tree("t2").Add([]byte(fmt.Sprintf("%d", i+20)), []byte(fmt.Sprintf("number %d", i+20)))
	}

	// Check Count
	if s.Tree("t1").Count() != 20 {
		t.Errorf("tree size must be 20, but it is %d", s.Tree("t1").Count())
	}

	// Check value persists and its OK
	v, err := s.Tree("t1").Get([]byte("1"))
	if err != nil {
		t.Error(err)
	}
	if string(v) != "number 1" {
		t.Errorf("stored data for key 1 is wrong")
	}

	// Commit tree state (save to h2) (V2)
	h2, err := s.Commit()
	if err != nil {
		t.Error(err)
	}
	t.Logf("[v2] version %d hash %x", s.Version(), h2)

	// Change a value and test rollback
	s.Tree("t1").Add([]byte("1"), []byte(("number changed")))
	s.Rollback()

	// Check value persists and its OK
	if v, err = s.Tree("t1").Get([]byte("1")); err != nil {
		t.Error(err)
	}
	if string(v) != "number 1" {
		t.Errorf("rollback do not reset the values")
	}

	// Change a value and test immutable (should be the previous version)
	s.Tree("t1").Add([]byte("1"), []byte(("number changed")))

	if v, err = s.ImmutableTree("t1").Get([]byte("1")); err != nil {
		t.Error(err)
	}
	if string(v) != "number 1" {
		t.Errorf("immutable tree has muted without commit")
	}

	// Commit changes, at this point "1" == "number changed" (V3)
	if h, err := s.Commit(); err != nil {
		t.Error(err)
	} else {
		t.Logf("[v3] version %d hash %x", s.Version(), h)
	}

	// Test immutable tree is updated
	if v, err = s.ImmutableTree("t1").Get([]byte("1")); err != nil {
		t.Error(err)
	}
	if vStr := string(v); vStr != "number changed" {
		t.Errorf("immutable tree has not been updated after commit, value is %s", vStr)
	}

	// Load previous version
	v1 := s.Version()
	if err = s.LoadVersion(-1); err != nil {
		t.Error(err)
	}
	v2 := s.Version()
	if v1 == v2 {
		t.Error("after load version -1, version number is still the same")
	}

	// Check value persists and its OK
	if v, err = s.Tree("t1").Get([]byte("1")); err != nil {
		t.Error(err)
	}
	if string(v) != "number 1" {
		t.Errorf("load version -1 does not work")
	}

	// Check hash is the same of h2 after load version -1
	if string(s.Hash()) != string(h2) {
		t.Errorf("after loading version -1 hash do not match: %x != %x", s.Hash(), h2)
	}

	// Test iteration and prefix
	for i := 0; i < 10; i++ {
		s.Tree("t1").Add([]byte(fmt.Sprintf("PREFIXED_%d", i)), []byte(fmt.Sprintf("number %d", i)))
	}

	s.Tree("t1").Iterate([]byte("PREFIXED_"), func(k, v []byte) bool {
		if !strings.HasPrefix(string(k), "PREFIXED_") {
			t.Errorf("prefix on iterate is not filtering")
			return true
		}
		return false
	})
}

func TestOrder(t *testing.T) {
	t.Parallel()

	s := &IavlState{}
	if err := s.Init(t.TempDir(), "disk"); err != nil {
		t.Fatal(err)
	}

	// Lets create two state trees
	if err := s.AddTree("t3"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTree("t4"); err != nil {
		t.Fatal(err)
	}

	// No previous version available, but this should be supported
	if err := s.LoadVersion(0); err != nil {
		t.Fatal(err)
	}

	// Add 10 identical entries to both trees but reversed order
	for i := 0; i < 1000; i++ {
		s.Tree("t3").Add([]byte(fmt.Sprintf("%d", i)), []byte(fmt.Sprintf("number %d", i)))
	}
	for i := 999; i >= 0; i-- {
		s.Tree("t4").Add([]byte(fmt.Sprintf("%d", i)), []byte(fmt.Sprintf("number %d", i)))
	}

	// Compare hash
	if string(s.Tree("t3").Hash()) != string(s.Tree("t4").Hash()) {
		t.Errorf("hash between two identical trees tree is different when using different order")
	}

	// Check Proof generation and validation
	_, err := s.Tree("t3").Proof([]byte("5"))
	if err != nil {
		t.Error(err)
	}

	/*	if ok := s.Tree("t3").Verify([]byte("5"), proof, nil); !ok {
			t.Errorf("proof is invalid, should be valid")
		}

		if ok := s.Tree("t3").Verify([]byte("_"), proof, nil); ok {
			t.Errorf("proof is valid, should be invalid")
		}
	*/
}
