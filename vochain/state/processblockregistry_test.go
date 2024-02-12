package state

import (
	"encoding/binary"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func mockEndBlockProcessOnState(s *State, pid []byte, startBlock, blockCount uint32) error {
	if err := s.AddProcess(&models.Process{
		ProcessId:    pid,
		StartBlock:   startBlock,
		BlockCount:   blockCount,
		EnvelopeType: &models.EnvelopeType{},
		Mode:         &models.ProcessMode{},
		VoteOptions:  &models.ProcessVoteOptions{},
	}); err != nil {
		return err
	}
	return s.ProcessBlockRegistry.SetStartBlock(pid, startBlock)
}

func TestSetStartBlock(t *testing.T) {
	c := qt.New(t)
	// create a state for testing
	dir := t.TempDir()
	s, err := New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	// set a start block for a random electionId
	pid := util.RandomBytes(32)
	c.Assert(s.ProcessBlockRegistry.SetStartBlock(pid, 100), qt.IsNil)
	// check the database to ensure that the start block has been created
	// succesfully
	encBlockNum, err := s.ProcessBlockRegistry.db.Get(toPrefixKey(pbrDBPrefix, pid))
	c.Assert(err, qt.IsNil)
	c.Assert(binary.LittleEndian.Uint32(encBlockNum), qt.Equals, uint32(100))
}

func TestDeleteStartBlock(t *testing.T) {
	c := qt.New(t)
	// create a state for testing
	dir := t.TempDir()
	s, err := New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	// set a start block for a random electionId and delete it then
	pid := util.RandomBytes(32)
	c.Assert(s.ProcessBlockRegistry.SetStartBlock(pid, 100), qt.IsNil)
	c.Assert(s.ProcessBlockRegistry.DeleteStartBlock(pid), qt.IsNil)
	// check the database to ensure that the start block has been deleted
	// succesfully
	_, err = s.ProcessBlockRegistry.db.Get(pid)
	c.Assert(err, qt.IsNotNil)
}

func TestMinStartBlock(t *testing.T) {
	c := qt.New(t)
	// create a state for testing
	dir := t.TempDir()
	s, err := New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	fromBlock := uint32(100)
	// check min start block without create any election, fromBlock value
	// expected
	min, err := s.ProcessBlockRegistry.MinStartBlock(fromBlock)
	c.Assert(err, qt.IsNil)
	c.Assert(min, qt.Equals, fromBlock)
	// create a startBlock greater than the fromBlock, fromBlock value expected
	c.Assert(s.ProcessBlockRegistry.SetStartBlock(util.RandomBytes(32), 200), qt.IsNil)
	min, err = s.ProcessBlockRegistry.MinStartBlock(fromBlock)
	c.Assert(err, qt.IsNil)
	c.Assert(min, qt.Equals, fromBlock)
	// create a startBlock lower than the fromBlock, startBlock value expected
	startBlock := fromBlock - 1
	c.Assert(s.ProcessBlockRegistry.SetStartBlock(util.RandomBytes(32), startBlock), qt.IsNil)
	min, err = s.ProcessBlockRegistry.MinStartBlock(fromBlock)
	c.Assert(err, qt.IsNil)
	c.Assert(min, qt.Equals, startBlock)
}

func TestMaxEndBlock(t *testing.T) {
	c := qt.New(t)
	// create a state for testing
	dir := t.TempDir()
	s, err := New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	fromBlock := uint32(100)
	// check max endBlock without create any election, fromBlock value
	// expected
	max, err := s.ProcessBlockRegistry.MaxEndBlock(fromBlock)
	c.Assert(err, qt.IsNil)
	c.Assert(max, qt.Equals, fromBlock)
	// create a process with an endBlock lower than the fromBlock, fromBlock
	// value expected
	pid1 := util.RandomBytes(32)
	c.Assert(mockEndBlockProcessOnState(s, pid1, fromBlock-2, 1), qt.IsNil)
	max, err = s.ProcessBlockRegistry.MaxEndBlock(fromBlock)
	c.Assert(err, qt.IsNil)
	c.Assert(max, qt.Equals, fromBlock)
	// create a process with an endBlock greater than the fromBlock, endBlock
	// value expected
	pid2 := util.RandomBytes(32)
	c.Assert(mockEndBlockProcessOnState(s, pid2, fromBlock, 1), qt.IsNil)
	max, err = s.ProcessBlockRegistry.MaxEndBlock(fromBlock)
	c.Assert(err, qt.IsNil)
	c.Assert(max, qt.Equals, fromBlock+1)
}
