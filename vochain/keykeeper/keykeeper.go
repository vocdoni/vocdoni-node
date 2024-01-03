package keykeeper

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
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
func NewKeyKeeper(dbPath string, v *vochain.BaseApplication, signer *ethereum.SignKeys, index int8) (*KeyKeeper, error) {
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
	k.storage, err = pebbledb.New(db.Options{Path: dbPath})
	if err != nil {
		return nil, err
	}
	k.myIndex = index
	k.vochain.State.AddEventListener(k)
	return k, nil
}

// RevealUnpublished is a rescue function for revealing keys that should be already revealed.
func (k *KeyKeeper) RevealUnpublished() {
	// Wait for the node to be synchronized
	for k.vochain.IsSynchronizing() {
		time.Sleep(10 * time.Second)
	}
	// Acquire the lock
	k.lock.Lock()
	defer k.lock.Unlock()

	// Check for all if we have pending keys to reveal
	pids, err := k.vochain.State.ListProcessIDs(true)
	if err != nil {
		log.Errorw(err, "cannot get process list to reveal unpublished keykeeper keys")
	}
	for _, pid := range pids {
		process, err := k.vochain.State.Process(pid, true)
		if err != nil {
			log.Warnw("cannot get process from state", "pid", hex.EncodeToString(pid), "err", err)
			continue
		}
		if process.Status == models.ProcessStatus_ENDED && process.EnvelopeType.EncryptedVotes &&
			len(process.EncryptionPublicKeys)-1 >= int(k.myIndex) && process.EncryptionPublicKeys[k.myIndex] != "" {
			log.Warnw("found pending keys", "processId", hex.EncodeToString(pid))
			if err := k.revealKeys(string(pid)); err != nil {
				log.Errorw(err, fmt.Sprintf("cannot reveal keys for process %x", hex.EncodeToString(pid)))
			}
		}
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
func (k *KeyKeeper) OnProcess(pid, _ []byte, _, _ string, _ int32) {
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !p.EnvelopeType.EncryptedVotes {
		return
	}
	// If keys already exist, do nothing (this happens on the start-up block replay)
	if len(p.EncryptionPublicKeys)-1 >= int(k.myIndex) && len(p.EncryptionPublicKeys[k.myIndex]) > 0 {
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
func (k *KeyKeeper) OnCancel(pid []byte, _ int32) { // LEGACY
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !p.EnvelopeType.EncryptedVotes {
		return
	}

	if len(p.EncryptionPublicKeys)-1 >= int(k.myIndex) && p.EncryptionPublicKeys[k.myIndex] != "" {
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
func (*KeyKeeper) OnVote(_ *state.Vote, _ int32) {
	// do nothing
}

// OnNewTx is not used by the KeyKeeper
func (*KeyKeeper) OnNewTx(_ *vochaintx.Tx, _ uint32, _ int32) {
	// do nothing
}

func (*KeyKeeper) OnBeginBlock(state.BeginBlock) {
	// do nothing
}

// OnProcessStatusChange will publish the private
// keys of the ended process, if required
func (k *KeyKeeper) OnProcessStatusChange(pid []byte, status models.ProcessStatus, _ int32) {
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !p.EnvelopeType.EncryptedVotes {
		return
	}
	if len(p.EncryptionPublicKeys)-1 >= int(k.myIndex) && p.EncryptionPublicKeys[k.myIndex] != "" {
		if status == models.ProcessStatus_ENDED {
			log.Infof("process ended, scheduling reveal keys for next block")
			k.blockPool[string(pid)] = int64(k.vochain.State.CurrentHeight()) + 1
		}
	}
}

func (*KeyKeeper) OnCensusUpdate(_, _ []byte, _ string) {}

// Generate Keys generates a set of encryption/commitment keys for a process.
// Encryption private key = hash(signer.privKey + processId + keyIndex).
func (k *KeyKeeper) generateKeys(pid []byte) (*processKeys, error) {
	// Generate keys
	// Add the index in order to win some extra entropy
	pb := append(pid, byte(k.myIndex))
	// Private secp256k1 key
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
		if !process.EnvelopeType.EncryptedVotes {
			return
		}
		if len(process.EncryptionPublicKeys)-1 >= int(k.myIndex) && process.EncryptionPublicKeys[k.myIndex] != "" {
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
		Nonce:               uint32(util.RandomInt(0, 1000000000)),
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
		Nonce:                uint32(util.RandomInt(0, 1000000000)),
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

// OnProcessKeys does nothing
func (*KeyKeeper) OnProcessKeys(_ []byte, _ string, _ int32) {}

// OnRevealKeys does nothing
func (*KeyKeeper) OnRevealKeys(_ []byte, _ string, _ int32) {}

// OnProcessResults does nothing
func (*KeyKeeper) OnProcessResults(_ []byte, _ *models.ProcessResult, _ int32) {}

// OnProcessesStart does nothing
func (*KeyKeeper) OnProcessesStart(_ [][]byte) {}

// OnSetAccount does nothing
func (*KeyKeeper) OnSetAccount(_ []byte, _ *state.Account) {}

// OnTransferTokens does nothing
func (*KeyKeeper) OnTransferTokens(_ *vochaintx.TokenTransfer) {}

// OnSpendTokens does nothing
func (*KeyKeeper) OnSpendTokens(_ []byte, _ models.TxType, _ uint64, _ string) {}
