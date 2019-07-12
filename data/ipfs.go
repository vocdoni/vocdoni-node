package data

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os/exec"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"

	shell "github.com/ipfs/go-ipfs-api"
)

type IPFSConfig struct {
	Start       bool
	Binary      string
	InitTimeout int
}

func (c *IPFSConfig) Type() StorageID {
	return IPFS
}

type IPFSHandle struct {
	d *types.DataStore
	s *shell.Shell
	c *IPFSConfig
	//can we add a shell here for use by all methods?
}

func IPFSNewConfig() *IPFSConfig {
	cfg := new(IPFSConfig)
	cfg.Start = true
	cfg.Binary = "ipfs"
	cfg.InitTimeout = 10
	return cfg
}

// check if ipfs base dir exists
func checkIPFSinit(bin string) error {
	initCmd := exec.Command(bin, "config", "show")
	return initCmd.Run()
}

// init IPFS daemon
func startIPFSDaemon(ipfsBinPath string) error {
	err := checkIPFSinit(ipfsBinPath)
	if err != nil {
		log.Warn(err.Error())
		initCmd := exec.Command(ipfsBinPath, "init")
		log.Info("initializing IPFS for first time ... wait until completed")
		err := initCmd.Run()
		if err != nil {
			log.Warn(err.Error())
			return err
		}
		err = checkIPFSinit(ipfsBinPath)
		if err == nil {
			log.Info("ipfs init done!")
		} else {
			log.Warn(err.Error())
			return err
		}
	}

	cmd := exec.Command(ipfsBinPath, "daemon")
	if err := cmd.Start(); err != nil {
		return errors.New("Cannot init the IPFS daemon")
	}
	return nil
}

func (i *IPFSHandle) Init(d *types.DataStore) error {
	if i.c.Start {
		err := startIPFSDaemon(i.c.Binary)
		if err != nil {
			return err
		}
	}
	i.d = d
	i.s = shell.NewShell("localhost:5001")
	for timeout := i.c.InitTimeout; timeout > 0; timeout-- {
		if i.s.IsUp() {
			log.Info("IPFS daemon started")
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	//test that ipfs is running/working
	return errors.New("Could not connect to IPFS daemon")
}

func (i *IPFSHandle) Publish(object []byte) (string, error) {
	cid, err := i.s.Add(bytes.NewBuffer(object))
	if err != nil {
		return "", err
	}
	return cid, nil
}

func (i *IPFSHandle) Pin(path string) error {
	sh := shell.NewShell("localhost:5001")
	err := sh.Pin(path)
	if err != nil {
		return err
	}
	return nil
}

func (i *IPFSHandle) Unpin(path string) error {
	sh := shell.NewShell("localhost:5001")
	err := sh.Unpin(path)
	if err != nil {
		return err
	}
	return nil
}

func (i *IPFSHandle) ListPins() (map[string]string, error) {
	sh := shell.NewShell("localhost:5001")
	info, err := sh.Pins()
	if err != nil {
		return nil, err
	}
	var pinMap map[string]string
	pinMap = make(map[string]string)
	for k, c := range info {
		pinMap[k] = c.Type
	}
	return pinMap, nil
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
