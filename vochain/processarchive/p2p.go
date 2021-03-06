package processarchive

import (
	"context"
	"encoding/base64"
	"time"

	ipfscrypto "github.com/libp2p/go-libp2p-core/crypto"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
)

const (
	publishInterval = 10 * time.Minute
	ipnsKeyAlias    = "processarchivekey"
)

// AddKey adds a base64 encoded ECDSA 256bit private key or generates
// a new one.
func (p *ProcessArchive) AddKey(b64key string) error {
	var ipnsPk []byte
	if b64key == "" {
		// if key already exist, just return
		if _, err := p.GetKey(); err == nil {
			return nil
		}
		// else generate a new key
		ipnsPk = data.NewIPFSkey()
	} else {
		var err error
		ipnsPk, err = base64.StdEncoding.DecodeString(b64key)
		if err != nil {
			return err
		}
	}
	return p.ipfs.AddKeyToKeystore(ipnsKeyAlias, ipnsPk)
}

// GetKey fetch the base64 encoded IPFS private key used to
// publish the IPNS record.
func (p *ProcessArchive) GetKey() (string, error) {
	pk, err := p.ipfs.Node.Repo.Keystore().Get(ipnsKeyAlias)
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
func (p *ProcessArchive) publishLoop() {
	log.Infof("starting process archive IPNS publish daemon with interval %s",
		publishInterval)
	for {
		select {
		case <-p.publish:
			if time.Since(p.lastUpdate) < publishInterval {
				break
			}
			p.lastUpdate = time.Now()
			log.Infof("publishing process archive")
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), publishInterval)
			ipnsentry, err := p.ipfs.PublishIPNSpath(ctx, p.storage.datadir, ipnsKeyAlias)
			defer cancel()
			if err != nil {
				log.Warnf("could not publish to IPFS: %v", err)
				return
			}
			log.Infof("published to /ipns/%s with value %s, took %s",
				ipnsentry.Name(), ipnsentry.Value(), time.Since(start))
		case <-p.close:
			return
		}
	}
}
