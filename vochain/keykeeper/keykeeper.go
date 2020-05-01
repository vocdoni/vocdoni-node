package keykeeper

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/db"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

/*
 KV database shceme:
   p_{processId} = {[]processKeys} // index and stores the process keys by process ID
   b_{#block} = {[]processId} // index by block in order to reveal keys of the finished processes
*/

const (
	commitmentKeySize = 32
	dbPrefixProcess   = "p_"
	dbPrefixBlock     = "b_"
)

type KeyKeeper struct {
	vochain   *vochain.BaseApplication
	storage   db.Database
	keyPool   map[string]*processKeys
	blockPool map[string]int64
	signer    *signature.SignKeys
	lock      sync.Mutex
}

type processKeys struct {
	encryptionKey *nacl.KeyPair
	commitmentKey []byte
	index         int
}

func NewKeyKeeper(dbPath string, v *vochain.BaseApplication, signer *signature.SignKeys) (*KeyKeeper, error) {
	var err error
	var k KeyKeeper
	if v == nil || signer == nil || len(dbPath) < 1 {
		return nil, fmt.Errorf("missing values for creating a key keeper")
	}
	k.vochain = v
	k.signer = signer
	k.storage, err = db.NewBadgerDB(dbPath)
	if err != nil {
		return nil, err
	}
	k.vochain.State.AddEvent("rollback", &k)
	k.vochain.State.AddEvent("addProcess", &k)
	k.vochain.State.AddEvent("cancelProcess", &k)
	k.vochain.State.AddEvent("commit", &k)
	return &k, nil
}

// PrintInfo print some log information every wait duration
func (k *KeyKeeper) PrintInfo(wait time.Duration) {
	for {
		time.Sleep(wait)
		iter := k.storage.NewIterator()
		defer iter.Release()
		nprocs := 0
		for iter.Next() {
			if strings.HasPrefix(string(iter.Key()), dbPrefixProcess) {
				nprocs++
			}
		}
		log.Infof("[keykeeper] stored keys %d", nprocs)
	}
}

// Rollback removes the non commited pending operations.
// Rollback must be called before any other operation in order to allocate the pool queue memory.
func (k *KeyKeeper) Rollback() {
	k.keyPool = make(map[string]*processKeys)
	k.blockPool = make(map[string]int64)
}

// OnProcess creates the keys and add them to the pool queue, if the process requires it
func (k *KeyKeeper) OnProcess(pid, eid string) {
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !p.RequireKeys() {
		return
	}
	if _, exist := k.keyPool[pid]; exist {
		log.Errorf("keys for process %s already exist in the pool queue", pid)
		return
	}
	// Generate keys
	ek, err := nacl.Generate(rand.Reader)
	if err != nil {
		log.Errorf("cannot generate encryption key: (%s)", err)
		return
	}
	log.Infof("generated process encryption pubkey: %x", ek.Public)

	ck := make([]byte, commitmentKeySize)
	if n, err := rand.Read(ck); n != commitmentKeySize || err != nil {
		log.Errorf("cannot generate commitment key: (%s)", err)
	}
	log.Infof("generated commitment key: %x", ck)

	k.keyPool[pid] = &processKeys{
		encryptionKey: ek,
		commitmentKey: ck,
		index:         util.RandomInt(1, 16), // TBD check index key does not exist
	}
	// Add keys to the pool queue
	k.blockPool[pid] = p.StartBlock + p.NumberOfBlocks
}

// OnCancel will publish the private and reveal keys of the canceled process, if required
func (k *KeyKeeper) OnCancel(pid string) {
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !p.RequireKeys() {
		return
	}
	if err = k.revealKeys(pid); err != nil {
		log.Error(err)
	}
}

// Commit saves the pending operation
func (k *KeyKeeper) Commit(height int64) {
	k.scheduleRevealKeys()
	go k.checkRevealProcess(height)
	go k.publishPendingKeys()
}

// OnVote is not used by the KeyKeeper
func (k *KeyKeeper) OnVote(v *types.Vote) {
	// do nothing
}

// scheduleRevealKeys takes the pids from the blockPool and add them to the schedule storage
func (k *KeyKeeper) scheduleRevealKeys() {
	var err error
	var has bool
	var pkey, data []byte
	var pids []string
	k.lock.Lock()
	defer k.lock.Unlock()
	for pid, height := range k.blockPool {
		pids = []string{}
		pkey = []byte(dbPrefixBlock + fmt.Sprintf("%d", height))
		if has, err = k.storage.Has(pkey); has {
			data, err = k.storage.Get(pkey)
			if err != nil {
				log.Errorf("cannot get existing list of scheduled reveal processes for block %d", height)
				continue
			}
			if err = k.unmarshal(data, pids); err != nil {
				log.Errorf("cannot unmarshal process pids for block %d: (%s)", height, err)
			}
		}
		pids = append(pids, pid)
		data, err = k.marshal(pids)
		if err != nil {
			log.Errorf("cannot marshal new pid list for scheduling on block %d: (%s)", height, err)
			continue
		}
		if err = k.storage.Put(pkey, data); err != nil {
			log.Errorf("cannot save scheduled list of pids for block %d: (%s)", height, err)
			continue
		}
		log.Infof("scheduled reveal keys of process %s for block %d", pid, height)
	}
}

// checkRevealProcess check if keys should be revealed for height and deletes the entry from the storage
func (k *KeyKeeper) checkRevealProcess(height int64) {
	k.lock.Lock()
	pKey := []byte(dbPrefixBlock + fmt.Sprintf("%d", height))
	if has, err := k.storage.Has(pKey); !has {
		return
	} else if err != nil {
		log.Errorf("cannot check existence of reveal processes for block %d", height)
		return
	}
	data, err := k.storage.Get(pKey)
	if err != nil {
		log.Errorf("cannot get revel process for block %d", height)
		return
	}
	k.lock.Unlock()

	var pids []string
	if err := k.unmarshal(data, pids); err != nil {
		log.Errorf("cannot unmarshal process pids for block %d", height)
		return
	}
	for _, p := range pids {
		log.Infof("revealing keys for process %s on block %d", p, height)
		if err := k.revealKeys(p); err != nil {
			log.Errorf("cannot reveal proces keys for %s: (%s)", p, err)
		}
	}
	k.lock.Lock()
	if err := k.storage.Del(pKey); err != nil {
		log.Errorf("cannot delete revealed keys for block %d: (%s)", height, err)
	}
	k.lock.Unlock()
}

func (k *KeyKeeper) publishPendingKeys() {
	var err error
	for pid, pk := range k.keyPool {
		if err = k.publishKeys(pk, pid); err != nil {
			log.Errorf("cannot execute commit on publish keys for process %s: (%s)", pid, err)
		}
	}
}

// This functions must be async in order to avoid a deadlock on the block creation
func (k *KeyKeeper) publishKeys(pk *processKeys, pid string) error {
	log.Infof("publishing keys for process %s", pid)
	tx := &types.AdminTx{
		Type:                types.TxAddProcessKeys,
		KeyIndex:            pk.index, // TBD check it does not exist
		Nonce:               util.RandomHex(32),
		ProcessID:           pid,
		EncryptionPublicKey: fmt.Sprintf("%x", pk.encryptionKey.Public),
		CommitmentKey:       fmt.Sprintf("%x", pk.commitmentKey),
	}
	if err := k.signAndSendTx(tx); err != nil {
		return err
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	dbKey := []byte(dbPrefixProcess + pid)
	if exists, err := k.storage.Has(dbKey); exists || err != nil {
		return fmt.Errorf("keys for process %s already exist or error fetching storage: (%s)", pid, err)
	}
	data, err := k.marshal(pk)
	if err != nil {
		return err
	}
	return k.storage.Put(dbKey, data)
}

func (k *KeyKeeper) revealKeys(pid string) error {
	k.lock.Lock()
	dbKey := []byte(dbPrefixProcess + pid)
	data, err := k.storage.Get(dbKey)
	k.lock.Unlock()
	if err != nil {
		return fmt.Errorf("cannot fetch reveal keys from storage: (%s)", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("no keys data found on storage")
	}
	var pk *processKeys
	if err := k.unmarshal(data, pk); err != nil {
		return fmt.Errorf("cannot unmarshal process keys: (%s)", err)
	}
	tx := &types.AdminTx{
		Type:                types.TxRevealProcessKeys,
		KeyIndex:            pk.index,
		Nonce:               util.RandomHex(32),
		ProcessID:           pid,
		EncryptionPublicKey: fmt.Sprintf("%x", pk.encryptionKey.Public),
		CommitmentKey:       fmt.Sprintf("%x", pk.commitmentKey),
	}
	if err := k.signAndSendTx(tx); err != nil {
		return err
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	return k.storage.Del(dbKey)
}

func (k *KeyKeeper) signAndSendTx(tx *types.AdminTx) error {
	// sign the transaction
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	if tx.Signature, err = k.signer.Sign(txBytes); err != nil {
		return err
	}
	if txBytes, err = json.Marshal(tx); err != nil {
		return err
	}
	// Send the transaction to the mempool
	result, err := k.vochain.SendTX(txBytes)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("error sending addProcessKeys transaction: (%s)", result.Data)
	}
	return nil
}

func (k *KeyKeeper) marshal(data interface{}) ([]byte, error) {
	return k.vochain.State.Codec.MarshalBinaryBare(data)
}

func (k *KeyKeeper) unmarshal(data []byte, container interface{}) error {
	return k.vochain.State.Codec.UnmarshalBinaryBare(data, container)
}
