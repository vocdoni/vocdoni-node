package processarchive

import (
	"context"
	"encoding/base64"
	"time"

	ipfscrypto "github.com/libp2p/go-libp2p/core/crypto"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/log"
)

const (
	publishInterval = 10 * time.Minute
	ipnsKeyAlias    = "processarchivekey"
)

// AddKey adds a base64 encoded ECDSA 256bit private key or generates
// a new one.
func (pa *ProcessArchive) AddKey(b64key string) error {
	var ipnsPk []byte
	if b64key == "" {
		// if key already exist, just return
		if _, err := pa.GetKey(); err == nil {
			return nil
		}
		// else generate a new key
		ipnsPk = ipfs.NewIPFSkey()
	} else {
		var err error
		ipnsPk, err = base64.StdEncoding.DecodeString(b64key)
		if err != nil {
			return err
		}
	}
	return pa.ipfs.AddKeyToKeystore(ipnsKeyAlias, ipnsPk)
}

// GetKey fetch the base64 encoded IPFS private key used to
// publish the IPNS record.
func (pa *ProcessArchive) GetKey() (string, error) {
	pk, err := pa.ipfs.Node.Repo.Keystore().Get(ipnsKeyAlias)
	if err != nil {
		return "", err
	}
	pkb, err := ipfscrypto.MarshalPrivateKey(pk)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pkb), nil
}

// Publish is a blocking routine that reads ProcessArchive.publish channel
// in order to trigger a new IPNS record announcement publishing the
// process archive directory.
func (pa *ProcessArchive) publishLoop() {
	log.Infof("starting process archive IPNS publish daemon with interval %s",
		publishInterval)
	for {
		select {
		case <-pa.publish:
			if time.Since(pa.lastUpdate) < publishInterval {
				break
			}
			pa.lastUpdate = time.Now()
			log.Infof("publishing process archive")
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), publishInterval)
			ipnsentry, err := pa.ipfs.PublishIPNSpath(ctx, pa.storage.datadir, ipnsKeyAlias)
			defer cancel()
			if err != nil {
				log.Warnf("could not publish to IPFS: %v", err)
				return
			}
			log.Infof("published to /ipns/%s with value %s, took %s",
				ipnsentry.Name(), ipnsentry.Value(), time.Since(start))
		case <-pa.close:
			return
		}
	}
}
