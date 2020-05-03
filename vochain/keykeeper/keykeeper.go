package keykeeper

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
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
	encryptionKeySize = 32
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
	pubKey        [encryptionKeySize]byte
	privKey       [encryptionKeySize]byte
	revealKey     [commitmentKeySize]byte
	commitmentKey [commitmentKeySize]byte
	index         int8
}

func (pk *processKeys) Encode() []byte {
	data := make([]byte, commitmentKeySize*2+encryptionKeySize*2+1)
	copy(data[:], pk.pubKey[:])
	copy(data[encryptionKeySize:], pk.privKey[:])
	i := encryptionKeySize * 2
	copy(data[i:], pk.revealKey[:])
	i = i + commitmentKeySize
	copy(data[i:], pk.commitmentKey[:])
	i = i + commitmentKeySize
	data[128] = byte(pk.index)
	return data
}

func (pk *processKeys) Decode(data []byte) error {
	if len(data) < commitmentKeySize*2+encryptionKeySize*2+1 {
		return fmt.Errorf("cannot decode, data too small")
	}
	copy(pk.pubKey[:], data[:])
	copy(pk.privKey[:], data[encryptionKeySize:])
	i := encryptionKeySize * 2
	copy(pk.revealKey[:], data[i:])
	i = i + commitmentKeySize
	copy(pk.commitmentKey[:], data[i:])
	i = i + commitmentKeySize
	pk.index = int8(data[i])
	return nil
}

// TBD garbage collector function run at init for revealing all these keys that should have beeen revealed

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
	//k.vochain.Codec.RegisterConcrete(&processKeys{}, "vocdoni/keykeeper.processKeys", nil)
	//k.vochain.Codec.RegisterConcrete(processKeys{}, "processKeys", nil)
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

// RevealUnpublished is a rescue function for revealing keys that should be already revealed.
// It should be callend once the Vochain is syncronized in order to have the correct height.
func (k *KeyKeeper) RevealUnpublished() {
	// wait for vochain sync?
	// This function can be probably deleted because the replay of blocks do this job automatically.
	header := k.vochain.State.Header(true)
	if header == nil {
		log.Errorf("cannot get blockchain header, skipping reveal unpublished operation")
		return
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	iter := k.storage.NewIterator()
	defer iter.Release()
	var pids []string
	for iter.Next() {
		if strings.HasPrefix(string(iter.Key()), dbPrefixBlock) {
			h, err := strconv.ParseInt(string(iter.Key()[len(dbPrefixBlock):]), 10, 64)
			if err != nil {
				log.Errorf("cannot fetch block number from keykeeper database: (%s)", err)
				continue
			}
			if header.Height > h+2 { // give some extra blocks to avoid collition with normal operation
				k.vochain.State.Codec.UnmarshalBinaryBare(iter.Value(), &pids)
				log.Warnf("found pending keys for reveal on process %s", pids)
				for _, p := range pids {
					if err := k.revealKeys(p); err != nil {
						log.Error(err)
					}
				}

			}
		}
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
	var ck [commitmentKeySize]byte
	ckb := make([]byte, commitmentKeySize)
	if n, err := rand.Read(ckb); n != commitmentKeySize || err != nil {
		log.Errorf("cannot generate commitment key: (%s)", err)
	}
	copy(ck[:], ckb[:])
	var ckhash [commitmentKeySize]byte
	copy(ckhash[:], signature.HashPoseidon(ckb)[:])
	k.keyPool[pid] = &processKeys{
		privKey:       ek.Private,
		pubKey:        ek.Public,
		revealKey:     ck,
		commitmentKey: ckhash,
		index:         int8(util.RandomInt(1, 16)), // TBD check index key does not exist
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
			if err = k.vochain.State.Codec.UnmarshalBinaryBare(data, &pids); err != nil {
				log.Errorf("cannot unmarshal process pids for block %d: (%s)", height, err)
			}
		}
		pids = append(pids, pid)
		data, err = k.vochain.Codec.MarshalBinaryBare(pids)
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
	defer k.lock.Unlock()
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

	var pids []string
	if err := k.vochain.Codec.UnmarshalBinaryBare(data, &pids); err != nil {
		log.Errorf("cannot unmarshal process pids for block %d: (%s)", height, err)
		return
	}
	for _, p := range pids {
		log.Infof("revealing keys for process %s on block %d", p, height)
		if err := k.revealKeys(p); err != nil {
			log.Errorf("cannot reveal proces keys for %s: (%s)", p, err)
		}
	}
	if err := k.storage.Del(pKey); err != nil {
		log.Errorf("cannot delete revealed keys for block %d: (%s)", height, err)
	}
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
		KeyIndex:            int(pk.index), // TBD check it does not exist
		Nonce:               util.RandomHex(32),
		ProcessID:           pid,
		EncryptionPublicKey: fmt.Sprintf("%x", pk.pubKey),
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
	data := pk.Encode()
	return k.storage.Put(dbKey, data)
}

// Insecure
func (k *KeyKeeper) revealKeys(pid string) error {
	dbKey := []byte(dbPrefixProcess + pid)
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
	tx := &types.AdminTx{
		Type:                 types.TxRevealProcessKeys,
		KeyIndex:             int(pk.index),
		Nonce:                util.RandomHex(32),
		ProcessID:            pid,
		EncryptionPrivateKey: fmt.Sprintf("%x", pk.privKey),
		RevealKey:            fmt.Sprintf("%x", pk.revealKey),
	}
	if err := k.signAndSendTx(tx); err != nil {
		return err
	}
	if len(pk.privKey) > 0 {
		log.Infof("revealing encryption key for process %s", pid)
	}
	if len(pk.revealKey) > 0 {
		log.Infof("revealing commitment key for process %s", pid)
	}
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
		return fmt.Errorf("error sending transaction: (%s)", result.Data.Bytes())
	}
	return nil
}
