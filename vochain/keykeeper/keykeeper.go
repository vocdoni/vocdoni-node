package keykeeper

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/crypto/snarks"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
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
	commitmentKeySize = nacl.KeyLength
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
	pubKey        []byte
	privKey       []byte
	revealKey     []byte
	commitmentKey []byte
	index         int8
}

func (pk *processKeys) Encode() []byte {
	data := make([]byte, commitmentKeySize*2+encryptionKeySize*2+1)
	i := 0

	copy(data, pk.pubKey)
	i += encryptionKeySize

	copy(data[i:], pk.privKey)
	i += encryptionKeySize

	copy(data[i:], pk.revealKey)
	i += commitmentKeySize

	copy(data[i:], pk.commitmentKey)

	data[128] = byte(pk.index)
	return data
}

func (pk *processKeys) Decode(data []byte) error {
	if len(data) < commitmentKeySize*2+encryptionKeySize*2+1 {
		return fmt.Errorf("cannot decode, data too small")
	}
	i := 0

	pk.pubKey = append([]byte(nil), data[i:i+encryptionKeySize]...)
	i += encryptionKeySize

	pk.privKey = append([]byte(nil), data[i:i+encryptionKeySize]...)
	i += encryptionKeySize

	pk.revealKey = append([]byte(nil), data[i:i+commitmentKeySize]...)
	i += commitmentKeySize

	pk.commitmentKey = append([]byte(nil), data[i:i+commitmentKeySize]...)
	i += commitmentKeySize

	pk.index = int8(data[i])
	return nil
}

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
	k.storage, err = db.NewBadgerDB(dbPath)
	if err != nil {
		return nil, err
	}
	k.myIndex = index
	// k.vochain.Codec.RegisterConcrete(&processKeys{}, "vocdoni/keykeeper.processKeys", nil)
	// k.vochain.Codec.RegisterConcrete(processKeys{}, "processKeys", nil)
	k.vochain.State.AddEventListener(k)
	return k, nil
}

// PrintInfo print some log information every wait duration
func (k *KeyKeeper) PrintInfo(wait time.Duration) {
	for {
		time.Sleep(wait)
		iter := k.storage.NewIterator()
		nprocs := 0
		for iter.Next() {
			if strings.HasPrefix(string(iter.Key()), dbPrefixBlock) {
				nprocs++
			}
		}
		iter.Release()
		log.Infof("[keykeeper] scheduled keys %d", nprocs)
	}
}

// RevealUnpublished is a rescue function for revealing keys that should be already revealed.
// It should be callend once the Vochain is syncronized in order to have the correct height.
func (k *KeyKeeper) RevealUnpublished() {
	// wait for vochain sync?
	header := k.vochain.State.Header(true)
	if header == nil {
		log.Errorf("cannot get blockchain header, skipping reveal unpublished operation")
		return
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	iter := k.storage.NewIterator()
	defer iter.Release()
	var pids models.StoredKeys
	log.Infof("starting keykeeper reveal recovery")
	// First get the scheduled reveal key process from the storage
	for iter.Next() {
		// TODO(mvdan): use a prefixed iterator
		if !strings.HasPrefix(string(iter.Key()), dbPrefixBlock) {
			continue
		}
		h, err := strconv.ParseInt(string(iter.Key()[len(dbPrefixBlock):]), 10, 64)
		if err != nil {
			log.Errorf("cannot fetch block number from keykeeper database: (%s)", err)
			continue
		}
		if header.Height <= h+1 {
			continue
		}
		if err := proto.Unmarshal(iter.Value(), &pids); err != nil {
			log.Errorf("could not unmarshal value: %s", err)
			continue
		}
		log.Warnf("found pending keys for reveal")
		for _, p := range pids.GetPids() {
			if err := k.revealKeys(string(p)); err != nil {
				log.Error(err)
			}
		}
		if err := k.storage.Del(iter.Key()); err != nil {
			log.Error(err)
		}
	}
	iter.Release()
	iter = k.storage.NewIterator()
	var pid []byte
	var err error
	var process *models.Process
	// Second take all existing processes and check if keys should be revealed (if canceled)
	for iter.Next() {
		if !strings.HasPrefix(string(iter.Key()), dbPrefixProcess) {
			continue
		}
		pid = iter.Key()[len(dbPrefixProcess):]
		process, err = k.vochain.State.Process(pid, true)
		if err != nil {
			log.Error(err)
			continue
		}
		if process.Status == models.ProcessStatus_CANCELED ||
			process.Status == models.ProcessStatus_ENDED {
			log.Warnf("found pending keys for reveal on process %x", pid)
			if err := k.revealKeys(string(pid)); err != nil {
				log.Error(err)
			}
		}
	}
	log.Infof("keykeeper reveal recovery finished")
}

// Rollback removes the non committed pending operations.
// Rollback must be called before any other operation in order to allocate the pool queue memory.
func (k *KeyKeeper) Rollback() {
	k.keyPool = make(map[string]*processKeys)
	k.blockPool = make(map[string]int64)
}

// OnProcess creates the keys and add them to the pool queue, if the process requires it
func (k *KeyKeeper) OnProcess(pid, eid []byte, censusRoot, censusURI string) {
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot get process from state: (%s)", err)
		return
	}
	if !(p.EnvelopeType.Anonymous || p.EnvelopeType.EncryptedVotes) {
		return
	}
	// If keys already exist, do nothing (this happens on the start-up block replay)
	if len(p.EncryptionPublicKeys[k.myIndex])+len(p.CommitmentKeys[k.myIndex]) > 0 {
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
func (k *KeyKeeper) OnCancel(pid []byte) { // LEGACY
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
		k.blockPool[string(pid)] = k.vochain.State.Header(false).Height + 1
	}
}

// Commit saves the pending operation
func (k *KeyKeeper) Commit(height int64) (error, bool) {
	k.scheduleRevealKeys()
	go k.checkRevealProcess(height)
	go k.publishPendingKeys()
	return nil, false
}

// OnVote is not used by the KeyKeeper
func (k *KeyKeeper) OnVote(v *models.Vote) {
	// do nothing
}

// OnProcessStatusChange will publish the private
// and reveal keys of the ended process, if required
func (k *KeyKeeper) OnProcessStatusChange(pid []byte, status models.ProcessStatus) {
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
			k.blockPool[string(pid)] = k.vochain.State.Header(false).Height + 1
		}
	}
}

func (k *KeyKeeper) OnProcessKeys(pid []byte, pub, com string) {
	// do nothing
}

func (k *KeyKeeper) OnRevealKeys(pid []byte, priv, rev string) {
	// do nothing
}

func (k *KeyKeeper) OnProcessResults(pid []byte, results []*models.QuestionResult) error {
	// do nothing
	return nil
}

// Generate Keys generates a set of encryption/commitment keys for a process.
// Encryption private key = hash(signer.privKey + processId + keyIndex).
// Reveal key is hashPoseidon(key).
// Commitment key is hashPoseidon(revealKey)
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
	// Reveal and commitment keys
	ckb := snarks.Poseidon.Hash(priv.Bytes())
	ck := ckb[:commitmentKeySize]
	ckhash := snarks.Poseidon.Hash(ckb)[:commitmentKeySize]

	pk := &processKeys{
		privKey:       priv.Bytes(),
		pubKey:        priv.Public().Bytes(),
		revealKey:     ck,
		commitmentKey: ckhash,
		index:         k.myIndex,
	}
	return pk, nil
}

// scheduleRevealKeys takes the pids from the blockPool and add them to the schedule storage
func (k *KeyKeeper) scheduleRevealKeys() {
	k.lock.Lock()
	defer k.lock.Unlock()
	for pid, height := range k.blockPool {
		pids := models.StoredKeys{}
		pkey := []byte(dbPrefixBlock + fmt.Sprintf("%d", height))
		var data []byte
		var err error
		// TODO(mvdan): replace Has+Get with just Get
		if has, _ := k.storage.Has(pkey); has {
			data, err = k.storage.Get(pkey)
			if err != nil {
				log.Errorf("cannot get existing list of scheduled reveal processes for block %d", height)
				continue
			}
			if err := proto.Unmarshal(data, &pids); err != nil {
				log.Errorf("cannot unmarshal process pids for block %d: (%s)", height, err)
			}
		}
		pids.Pids = append(pids.Pids, []byte(pid))
		data, err = proto.Marshal(&pids)
		if err != nil {
			log.Errorf("cannot marshal new pid list for scheduling on block %d: (%s)", height, err)
			continue
		}
		if err := k.storage.Put(pkey, data); err != nil {
			log.Errorf("cannot save scheduled list of pids for block %d: (%s)", height, err)
			continue
		}
		log.Infof("scheduled reveal keys of process %x for block %d", pid, height)
	}
}

// checkRevealProcess check if keys should be revealed for height
// and deletes the entry from the storage
func (k *KeyKeeper) checkRevealProcess(height int64) {
	k.lock.Lock()
	defer k.lock.Unlock()
	pKey := []byte(dbPrefixBlock + fmt.Sprintf("%d", height))
	// TODO(mvdan): replace Has+Get with just Get
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
	if err := k.storage.Del(pKey); err != nil {
		log.Errorf("cannot delete revealed keys for block %d: (%s)", height, err)
	}
}

func (k *KeyKeeper) publishPendingKeys() {
	for pid, pk := range k.keyPool {
		if err := k.publishKeys(pk, pid); err != nil {
			log.Errorf("cannot execute commit on publish keys for process %x: (%s)", pid, err)
		}
	}
}

// This functions must be async in order to avoid a deadlock on the block creation
func (k *KeyKeeper) publishKeys(pk *processKeys, pid string) error {
	log.Infof("publishing keys for process %x", []byte(pid))
	kindex := new(uint32)
	*kindex = uint32(pk.index)
	tx := &models.AdminTx{
		Txtype:              models.TxType_ADD_PROCESS_KEYS,
		KeyIndex:            kindex,
		Nonce:               util.RandomBytes(32),
		ProcessId:           []byte(pid),
		EncryptionPublicKey: pk.pubKey,
		CommitmentKey:       pk.commitmentKey,
	}
	if err := k.signAndSendTx(tx); err != nil {
		return err
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	dbKey := []byte(dbPrefixProcess + pid)
	// TODO(mvdan): replace Has with just Get
	if exists, err := k.storage.Has(dbKey); exists || err != nil {
		return fmt.Errorf("keys for process %x already exist or error fetching storage: (%s)", pid, err)
	}
	data := pk.Encode()
	return k.storage.Put(dbKey, data)
}

// Insecure
func (k *KeyKeeper) revealKeys(pid string) error {
	/*	dbKey := []byte(dbPrefixProcess + pid)
		data, err := k.storage.Get(dbKey)
		if err != nil {
			return fmt.Errorf("cannot fetch reveal keys from storage: (%s)", err)
		}
		if len(data) == 0 {
			return fmt.Errorf("no keys data found on storage")
		}
		var pk processKeys
		if err := pk.Decode(data); err != nil {
			return fmt.Errorf("cannot unmarshal process keys: (%s)", err)
		}
		if len(pk.privKey) < 32 && len(pk.revealKey) < commitmentKeySize {
			return fmt.Errorf("empty process keys")
		}
	*/
	pk, err := k.generateKeys([]byte(pid))
	if err != nil {
		return err
	}
	kindex := new(uint32)
	*kindex = uint32(pk.index)
	tx := &models.AdminTx{
		Txtype:               models.TxType_REVEAL_PROCESS_KEYS,
		KeyIndex:             kindex,
		Nonce:                util.RandomBytes(32),
		ProcessId:            []byte(pid),
		EncryptionPrivateKey: pk.privKey,
		RevealKey:            pk.revealKey,
	}
	if err := k.signAndSendTx(tx); err != nil {
		return err
	}
	if len(pk.privKey) > 0 {
		log.Infof("revealing encryption key for process %x", pid)
	}
	if len(pk.revealKey) > 0 {
		log.Infof("revealing commitment key for process %x", pid)
	}
	if err = k.storage.Del([]byte(dbPrefixProcess + pid)); err != nil {
		log.Warnf("cannot delete pid %x, for some reason it does not exist", pid)
	}
	return nil
}

func (k *KeyKeeper) signAndSendTx(tx *models.AdminTx) error {
	var err error
	stx := &models.SignedTx{}
	// sign the transaction
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Admin{Admin: tx}}); err != nil {
		return err
	}
	if stx.Signature, err = k.signer.Sign(stx.Tx); err != nil {
		return err
	}
	vtxBytes, err := proto.Marshal(stx)
	if err != nil {
		return err
	}
	// Send the transaction to the mempool
	result, err := k.vochain.SendTX(vtxBytes)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("error sending transaction: (%s)", result.Data.Bytes())
	}
	return nil
}
