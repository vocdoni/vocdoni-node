package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func writeKeyFile(filename string, k []byte) error {
	// Create the keystore directory with appropriate permissions
	// in case it is not present yet.
	const dirPerm = 0700
	if err := os.MkdirAll(filepath.Dir(filename), dirPerm); err != nil {
		return err
	}

	// Here we diverge from geth. No need for atomic write? just write it out
	// already
	if err := ioutil.WriteFile(filename, k, 0600); err != nil {
		return err
	}
	return nil
}

func getKeysDir(root string) (string, error) {
	return path.Join(home, "keys"), nil
}

func generateKeyFilename(address common.Address) (string, error) {
	keysDir, err := getKeysDir(home)
	if err != nil {
		return "", fmt.Errorf("couldn't get a suitable datadir (normally ~/.dvote/keys) - %v", err)
	}

	t := time.Now()
	keyFilename := fmt.Sprintf("%s-%d-%d-%d%s", address.Hex(), t.Year(), t.Month(), t.Day(), keyExt) // 0xDad23752bB8F80Bb82E6A2422A762fdeAf8dAb74-2022-1-13.vokey
	return path.Join(keysDir, keyFilename), nil
}
