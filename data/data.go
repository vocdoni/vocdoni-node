package data

import (
	"os"
	"fmt"
	"bytes"
	"io/ioutil"
	shell "github.com/ipfs/go-ipfs-api"
)

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
	if err != nil{
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

func PsSubscribe(topic string) *shell.PubSubSubscription {
	sh := shell.NewShell("localhost:5001")
	sub, err := sh.PubSubSubscribe(topic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	return sub
}

func PsPublish(topic, data string) error {
	sh := shell.NewShell("localhost:5001")
	err := sh.PubSubPublish(topic, data)
	if err != nil {
		return err
	}
	return nil
}
