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

func (i *IPFSHandle) Publish(object []byte) (string, error) {
	sh := shell.NewShell("localhost:5001")
	cid, err := sh.Add(bytes.NewBuffer(object))
	if err != nil {
		return "", err
	}
	return cid, nil
}

func (i *IPFSHandle) Pin(path string) {
	sh := shell.NewShell("localhost:5001")
	err := sh.Pin(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
	}
}

func (i *IPFSHandle) Unpin(path string) {
	sh := shell.NewShell("localhost:5001")
	err := sh.Unpin(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
	}
}

func (i *IPFSHandle) Pins() map[string]shell.PinInfo {
	sh := shell.NewShell("localhost:5001")
	info, err := sh.Pins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
	}
	return info
}

func (i *IPFSHandle) Retrieve(hash string) ([]byte, error) {
	sh := shell.NewShell("localhost:5001")
	reader, err := sh.Cat(hash)
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return content, nil
}
