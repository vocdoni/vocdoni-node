package data

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	shell "github.com/ipfs/go-ipfs-api"
)

//maybe should expose 'durablepublish' (publish + pin)
type Storage interface {
	Publish(o []byte) string
	Retrieve(id string) []byte
}

func Publish(object []byte) string {
	sh := shell.NewShell("localhost:5001")
	cid, err := sh.Add(bytes.NewBuffer(object))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	return cid
}

func Pin(path string) {
	sh := shell.NewShell("localhost:5001")
	err := sh.Pin(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
}

func Retrieve(hash string) []byte {
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
