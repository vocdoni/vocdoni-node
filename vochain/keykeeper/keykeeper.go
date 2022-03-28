package keykeeper

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/badgerdb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

/*
 KV database shceme:
   p_{processId} = {[]processKeys} // index and stores the process keys by process ID
   b_{#block} = {[]processId} // index by block in order to reveal keys of the finished processes
*/

// TBD (pau): Remove the ProcessKeys storage, we do not need it since the
// keys are deterministic and can be re-created at any time.

const (
	encryptionKeySize = nacl.KeyLength
	dbPrefixProcess   = "p_"
	dbPrefixBlock     = "b_"
)

type KeyKeeper struct {
	vochain   *vochain.BaseApplication
	storage   db.Database
	keyPool   map[string]*processKeys
	blockPool map[string]int64
	signer    *ethereum.SignKeys
	lock      sync.Mutex
	myIndex   int8
}

type processKeys struct {
	pubKey  []byte
	privKey []byte
	index   int8
}

// Encode encodes processKeys to bytes
func (pk *processKeys) Encode() []byte {
	data := make([]byte, encryptionKeySize*2+1)
	i := 0

	copy(data, pk.pubKey)
	i += encryptionKeySize

	copy(data[i:], pk.privKey)
	i += encryptionKeySize

	data[i] = byte(pk.index)
	return data
}

// Decode decodes processKeys from data
func (pk *processKeys) Decode(data []byte) error {
	if len(data) < encryptionKeySize*2+1 {
		return fmt.Errorf("cannot decode, data too small")
	}
	i := 0

	pk.pubKey = append([]byte(nil), data[i:i+encryptionKeySize]...)
	i += encryptionKeySize

	pk.privKey = append([]byte(nil), data[i:i+encryptionKeySize]...)
	i += encryptionKeySize

	pk.index = int8(data[i])
	return nil
}

// NewKeyKeeper registers a new keyKeeper to the vochain
func NewKeyKeeper(dbPath string, v *vochain.BaseApplication,
	signer *ethereum.SignKeys, index int8) (*KeyKeeper, error) {
	if v == nil || signer == nil || len(dbPath) < 1 {
		return nil, fmt.Errorf("missing values for creating a key keeper")
	}
	if index == 0 {
		return nil, fmt.Errorf("index 0 cannot be used")
	}
	k := &KeyKeeper{
		vochain: v,
		signer:  signer,
	}
	var err error
	k.storage, err = badgerdb.New(db.Options{Path: dbPath})
	if err != nil {
		return nil, err
	}
	k.myIndex = index
	k.vochain.State.AddEventListener(k)
	return k, nil
}

// RevealUnpublished is a rescue function for revealing keys that should be already revealed.
// It should be callend once the Vochain is syncronized in order to have the correct height.
func (k *KeyKeeper) RevealUnpublished() {
	// wait for vochain sync?
	height, err := k.vochain.State.LastHeight()
	if err != nil {
		log.Errorf("cannot get blockchain last height, skipping RevealUnpublished operation")
		return
	}
	k.lock.Lock()
	defer k.lock.Unlock()

	var pids models.StoredKeys
	log.Infof("starting keykeeper reveal recovery")
	wTx := k.storage.WriteTx()
	defer wTx.Discard()

	// First get the scheduled reveal key process from the storage, iterate
	// over the prefix dbPrefixBlock
	if err := k.storage.Iterate([]byte(dbPrefixBlock), func(key, value []byte) bool {
		h, err := strconv.ParseInt(string(key[len(dbPrefixBlock):]), 10, 64)
		if err != nil {
			log.Errorf("cannot fetch block number from keykeeper database: (%s)", err)
			return true
		}
		if int64(height) <= h+1 {
			return true
		}
		if err := proto.Unmarshal(value, &pids); err != nil {
			log.Errorf("could not unmarshal value: %s", err)
			return true
		}
		log.Warnf("found pending keys for reveal")
		for _, p := range pids.GetPids() {
			if err := k.revealKeys(string(p)); err != nil {
				log.Error(err)
			}
		}
		if err := wTx.Delete(key); err != nil {
			log.Error(err)
		}
		return true
	}); err != nil {
		log.Error(err)
	}
	if err := wTx.Commit(); err != nil {
		log.Error(err)
	}

	var pid []byte
	var process *models.Process
	// Second take all existing processes and check if keys should be
	// revealed (if canceled), iterate over the prefix dbPrefixProcess
	if err := k.storage.Iterate([]byte(dbPrefixProcess), func(key, value []byte) bool {
		pid = key[len(dbPrefixProcess):]
		process, err = k.vochain.State.Process(pid, true)
		if err != nil {
			log.Error(err)
			return true
		}
		if process.Status == models.ProcessStatus_CANCELED ||
			process.Status == models.ProcessStatus_ENDED {
			log.Warnf("found pending keys for reveal on process %x", pid)
			if err := k.revealKeys(string(pid)); err != nil {
				log.Error(err)
			}
		}
		return true
	}); err != nil {
		log.Error(err)
	}
	log.Infof("keykeeper reveal recovery finished")
}

// Rollback removes the non committed pending operations.
// Rollback must be called before any other operation in order to allocate the pool queue memory.
func (k *KeyKeeper) Rollback() {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.keyPool = make(map[string]*processKeys)
	k.blockPool = make(map[string]int64)
}

// OnProcess creates the keys and add them to the pool queue, if the process requires it
func (k *KeyKeeper) OnProcess(pid, eid []byte, censusRoot, censusURI string, txindex int32) {
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !(p.EnvelopeType.Anonymous || p.EnvelopeType.EncryptedVotes) {
		return
	}
	// If keys already exist, do nothing (this happens on the start-up block replay)
	if len(p.EncryptionPublicKeys[k.myIndex]) > 0 {
		return
	}
	log.Debugf("generating key for process %x", pid)
	// Check if already created on this block process
	if _, exist := k.keyPool[string(pid)]; exist {
		log.Errorf("keys for process %x already exist in the pool queue", pid)
		return
	}

	// Generate keys
	if k.keyPool[string(pid)], err = k.generateKeys(pid); err != nil {
		log.Errorf("cannot generate process keys: (%s)", err)
		return
	}

	// Add keys to the pool queue
	k.blockPool[string(pid)] = int64(p.StartBlock + p.BlockCount)
}

// OnCancel will publish the private and reveal keys of the canceled process, if required
func (k *KeyKeeper) OnCancel(pid []byte, txindex int32) { // LEGACY
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !(p.EnvelopeType.Anonymous || p.EnvelopeType.EncryptedVotes) {
		return
	}

	if p.EncryptionPublicKeys[k.myIndex] != "" {
		log.Infof("process canceled, scheduling reveal keys for next block")
		k.blockPool[string(pid)] = int64(k.vochain.State.CurrentHeight()) + 1
	}
}

// Commit saves the pending operation
func (k *KeyKeeper) Commit(height uint32) error {
	k.scheduleRevealKeys()
	go k.checkRevealProcess(height)
	go k.publishPendingKeys()
	return nil
}

// OnVote is not used by the KeyKeeper
func (k *KeyKeeper) OnVote(v *models.Vote, txindex int32) {
	// do nothing
}

// OnNewTx is not used by the KeyKeeper
func (k *KeyKeeper) OnNewTx(hash []byte, blockHeight uint32, txIndex int32) {
	// do nothing
}

// OnProcessStatusChange will publish the private
// keys of the ended process, if required
func (k *KeyKeeper) OnProcessStatusChange(pid []byte, status models.ProcessStatus, txindex int32) {
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !(p.EnvelopeType.Anonymous || p.EnvelopeType.EncryptedVotes) {
		return
	}
	if p.EncryptionPublicKeys[k.myIndex] != "" {
		if status == models.ProcessStatus_ENDED {
			log.Infof("process ended, scheduling reveal keys for next block")
			k.blockPool[string(pid)] = int64(k.vochain.State.CurrentHeight()) + 1
		}
	}
}

// OnProcessKeys does nothing
func (k *KeyKeeper) OnProcessKeys(pid []byte, pub string, txindex int32) {
	// do nothing
}

// OnRevealKeys does nothing
func (k *KeyKeeper) OnRevealKeys(pid []byte, priv string, txindex int32) {
	// do nothing
}

// OnProcessResults does nothing
func (k *KeyKeeper) OnProcessResults(pid []byte,
	results *models.ProcessResult, txindex int32) error {
	// do nothing
	return nil
}

// OnProcessesStart does nothing
func (k *KeyKeeper) OnProcessesStart(pids [][]byte) {}

// Generate Keys generates a set of encryption/commitment keys for a process.
// Encryption private key = hash(signer.privKey + processId + keyIndex).
func (k *KeyKeeper) generateKeys(pid []byte) (*processKeys, error) {
	// Generate keys
	// Add the index in order to win some extra entropy
	pb := append(pid, byte(k.myIndex))
	// Private ed25519 key
	priv, err := nacl.DecodePrivate(fmt.Sprintf("%x",
		ethereum.HashRaw(append(k.signer.Private.D.Bytes(), pb...))))
	if err != nil {
		return nil, fmt.Errorf("cannot generate encryption key: (%s)", err)
	}
	pk := &processKeys{
		privKey: priv.Bytes(),
		pubKey:  priv.Public().Bytes(),
		index:   k.myIndex,
	}
	return pk, nil
}

// scheduleRevealKeys takes the pids from the blockPool and add them to the schedule storage
func (k *KeyKeeper) scheduleRevealKeys() {
	k.lock.Lock()
	defer k.lock.Unlock()

	wTx := k.storage.WriteTx()
	defer wTx.Discard()

	for pid, height := range k.blockPool {
		pids := models.StoredKeys{}
		pkey := []byte(dbPrefixBlock + fmt.Sprintf("%d", height))
		var data []byte
		var err error

		data, err = wTx.Get(pkey)
		if err == nil {
			// case where key does exist
			if err := proto.Unmarshal(data, &pids); err != nil {
				log.Errorf("cannot unmarshal process pids for block %d: (%s)", height, err)
			}
		} else if err != db.ErrKeyNotFound {
			// case where Get returns an error, and is not that ErrKeyNotFound
			log.Errorf("cannot get existing list of scheduled reveal processes for block %d", height)
			continue
		}

		pids.Pids = append(pids.Pids, []byte(pid))
		data, err = proto.Marshal(&pids)
		if err != nil {
			log.Errorf("cannot marshal new pid list for scheduling on block %d: (%s)", height, err)
			continue
		}
		if err := wTx.Set(pkey, data); err != nil {
			log.Errorf("cannot save scheduled list of pids for block %d: (%s)", height, err)
			continue
		}
		log.Infof("scheduled reveal keys of process %x for block %d", pid, height)
	}
	if err := wTx.Commit(); err != nil {
		log.Error(err)
	}
}

// checkRevealProcess check if keys should be revealed for height
// and deletes the entry from the storage
func (k *KeyKeeper) checkRevealProcess(height uint32) {
	k.lock.Lock()
	defer k.lock.Unlock()
	pKey := []byte(dbPrefixBlock + fmt.Sprintf("%d", height))

	wTx := k.storage.WriteTx()
	defer wTx.Discard()

	data, err := wTx.Get(pKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return
	}
	if err != nil {
		log.Errorf("cannot get reveal process for block %d", height)
		return
	}

	var pids models.StoredKeys
	if err := proto.Unmarshal(data, &pids); err != nil {
		log.Errorf("cannot unmarshal process pids for block %d: (%s)", height, err)
		return
	}
	for _, p := range pids.GetPids() {
		process, err := k.vochain.State.Process(p, false)
		if err != nil {
			log.Errorf("cannot get process from state: (%s)", err)
			continue
		}
		if !(process.EnvelopeType.Anonymous || process.EnvelopeType.EncryptedVotes) {
			return
		}
		if process.EncryptionPublicKeys[k.myIndex] != "" {
			log.Infof("revealing keys for process %x on block %d", p, height)
			if err := k.revealKeys(string(p)); err != nil {
				log.Errorf("cannot reveal process keys for %x: (%s)", p, err)
			}
		}
	}
	if err := wTx.Delete(pKey); err != nil {
		log.Errorf("cannot delete revealed keys for block %d: (%s)", height, err)
	}
	if err := wTx.Commit(); err != nil {
		log.Error(err)
		return
	}
}

// publishPendingKeys publishes each key in the keyPool
func (k *KeyKeeper) publishPendingKeys() {
	k.lock.Lock()
	defer k.lock.Unlock()
	for pid, pk := range k.keyPool {
		if err := k.publishKeys(pk, pid); err != nil {
			log.Errorf("cannot execute commit on publish keys for process %x: (%s)", pid, err)
		}
	}
}

func (k *KeyKeeper) publishKeys(pk *processKeys, pid string) error {
	log.Infof("publishing keys for process %x", []byte(pid))
	kindex := new(uint32)
	*kindex = uint32(pk.index)
	tx := &models.AdminTx{
		Txtype:              models.TxType_ADD_PROCESS_KEYS,
		KeyIndex:            kindex,
		Nonce:               0,
		ProcessId:           []byte(pid),
		EncryptionPublicKey: pk.pubKey,
	}
	if err := k.signAndSendTx(tx); err != nil {
		return err
	}
	dbKey := []byte(dbPrefixProcess + pid)

	wTx := k.storage.WriteTx()
	defer wTx.Discard()

	var err error
	if _, err = wTx.Get(dbKey); errors.Is(err, db.ErrKeyNotFound) {
		// key does not exist yet, store it
		data := pk.Encode()
		if err = wTx.Set(dbKey, data); err != nil {
			return err
		}
		return wTx.Commit()
	}
	// key exists or there is an error fetching storage
	return fmt.Errorf("keys for process %x already exist or error fetching storage: (%s)", pid, err)
}

// Insecure
// revealKeys reveals the keys for a given process
func (k *KeyKeeper) revealKeys(pid string) error {
	pk, err := k.generateKeys([]byte(pid))
	if err != nil {
		return err
	}
	kindex := new(uint32)
	*kindex = uint32(pk.index)
	tx := &models.AdminTx{
		Txtype:               models.TxType_REVEAL_PROCESS_KEYS,
		KeyIndex:             kindex,
		Nonce:                0,
		ProcessId:            []byte(pid),
		EncryptionPrivateKey: pk.privKey,
	}
	if err := k.signAndSendTx(tx); err != nil {
		return err
	}
	if len(pk.privKey) > 0 {
		log.Infof("revealing encryption key for process %x", pid)
	}

	wTx := k.storage.WriteTx()
	defer wTx.Discard()
	if err = wTx.Delete([]byte(dbPrefixProcess + pid)); err != nil {
		log.Warnf("cannot delete pid %x, for some reason it does not exist", pid)
	}
	return wTx.Commit()
}

func (k *KeyKeeper) signAndSendTx(tx *models.AdminTx) error {
	var err error
	stx := &models.SignedTx{}
	// sign the transaction
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Admin{Admin: tx}}); err != nil {
		return err
	}
	if stx.Signature, err = k.signer.SignVocdoniTx(stx.Tx, k.vochain.ChainID()); err != nil {
		return err
	}
	vtxBytes, err := proto.Marshal(stx)
	if err != nil {
		return err
	}
	// Send the transaction to the mempool
	result, err := k.vochain.SendTx(vtxBytes)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("error sending transaction: (%s)", result.Data.Bytes())
	}
	return nil
}
