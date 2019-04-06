package data

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"github.com/vocdoni/go-dvote/types"

	shell "github.com/ipfs/go-ipfs-api"
)

type IPFSHandle struct {
	s *types.DataStore
}

func (i *IPFSHandle) Init(s *types.DataStore) error {
	i.s = s
	//test that ipfs is running/working
	return nil
}

func (i *IPFSHandle) Publish(object []byte) string {
	sh := shell.NewShell("localhost:5001")
	cid, err := sh.Add(bytes.NewBuffer(object))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	i.pin(cid)
	return cid
}

func (i *IPFSHandle) pin(path string) {
	sh := shell.NewShell("localhost:5001")
	err := sh.Pin(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
}

func (i *IPFSHandle) Retrieve(hash string) []byte {
	sh := shell.NewShell("localhost:5001")
	reader, err := sh.Cat(hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	return content
}

