package keykeeper

import (
	"encoding/hex"
	"fmt"
	"slices"
	"sync"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// KeyKeeper is a vochain event listener that manages the encryption keys for the processes.
// Only validators that have a key index greater than 0 can use this.
// Once a new process is created, the key keeper will generate a new encryption key pair and add it to the process
// via a transaction. When the process ends, the key keeper will reveal the private key.
//
// The key keeper is a deterministic process, meaning that the keys are deterministic and can be re-created at any time.
// This is useful for recovering the keys in case of a failure.
// A key is generated by hashing the signer private key, the process ID and the key index.
type KeyKeeper struct {
	vochain          *vochain.BaseApplication
	pidsToRevealKeys []types.HexBytes
	pidsToAddKeys    []types.HexBytes
	signer           *ethereum.SignKeys
	lock             sync.Mutex
	myIndex          int8
}

type processKeys struct {
	pubKey  []byte
	privKey []byte
	index   int8
}

// NewKeyKeeper registers a new keyKeeper to the vochain application. If index is 0, it will return an error.
func NewKeyKeeper(v *vochain.BaseApplication, signer *ethereum.SignKeys, index int8) (*KeyKeeper, error) {
	if v == nil || signer == nil {
		return nil, fmt.Errorf("missing values for creating a key keeper")
	}
	if index == 0 {
		return nil, fmt.Errorf("index 0 cannot be used")
	}
	k := &KeyKeeper{
		vochain: v,
		signer:  signer,
	}
	k.myIndex = index
	k.vochain.State.AddEventListener(k)
	return k, nil
}

// RevealUnpublished is a rescue function for revealing keys that should be already revealed.
// This is useful for recovering keys that should have been revealed but were not.
// This can happen when the node is stopped while there are pending keys to reveal.
func (k *KeyKeeper) RevealUnpublished() {
	// Wait for the node to be synchronized
	<-k.vochain.WaitUntilSynced()

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
			if err := k.revealKeys(pid); err != nil {
				log.Errorw(err, fmt.Sprintf("cannot reveal keys for process %x", hex.EncodeToString(pid)))
			}
		}
	}
	log.Infof("keykeeper reveal recovery finished")
}

// Rollback removes the non committed pending operations.
// Rollback must be called before any other operation in order to reset the pool queue memory.
func (k *KeyKeeper) Rollback() {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.pidsToRevealKeys = []types.HexBytes{}
	k.pidsToAddKeys = []types.HexBytes{}
}

// OnProcess creates the keys and add them to the pool queue, if the process requires it
func (k *KeyKeeper) OnProcess(p *models.Process, _ int32) {
	k.lock.Lock()
	defer k.lock.Unlock()
	if !p.EnvelopeType.EncryptedVotes {
		return
	}
	// If keys already exist, do nothing (this happens on the start-up block replay)
	if len(p.EncryptionPublicKeys)-1 >= int(k.myIndex) && len(p.EncryptionPublicKeys[k.myIndex]) > 0 {
		return
	}

	// Add the process to the pool queue
	k.pidsToAddKeys = append(k.pidsToAddKeys, p.GetProcessId())
}

// OnProcessStatusChange will publish the private
// keys of the ended process, if required
func (k *KeyKeeper) OnProcessStatusChange(pid []byte, status models.ProcessStatus, _ int32) {
	k.lock.Lock()
	defer k.lock.Unlock()
	p, err := k.vochain.State.Process(pid, false)
	if err != nil {
		log.Errorw(err, "cannot get process from state")
		return
	}
	if !p.EnvelopeType.EncryptedVotes {
		return
	}
	if len(p.EncryptionPublicKeys)-1 >= int(k.myIndex) && p.EncryptionPublicKeys[k.myIndex] != "" {
		if status == models.ProcessStatus_ENDED {
			k.pidsToRevealKeys = append(k.pidsToRevealKeys, pid)
		}
	}
}

// Commit will publish the pending keys to the vochain.
func (k *KeyKeeper) Commit(_ uint32) error {
	// Important notice: the addKeys and revealKeys action must be executed within a goroutine because
	// both send transactions and would block the block production.
	k.lock.Lock()
	defer k.lock.Unlock()
	for _, pid := range k.pidsToAddKeys {
		p := slices.Clone(pid)
		go func() {
			if err := k.addKeys(p); err != nil {
				log.Errorw(err, fmt.Sprintf("cannot add encryption keys to process %x", pid))
			}
		}()
	}
	for _, pid := range k.pidsToRevealKeys {
		p := slices.Clone(pid)
		go func() {
			if err := k.revealKeys(p); err != nil {
				log.Errorw(err, fmt.Sprintf("cannot reveal encryption keys for process %x", pid))
			}
		}()
	}
	return nil
}

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

// addKeys publishes the public key of the given encrypted process.
func (k *KeyKeeper) addKeys(pid types.HexBytes) error {
	log.Infow("add encryption public key", "processId", pid.String(), "keyIndex", k.myIndex)
	pk, err := k.generateKeys(pid)
	if err != nil {
		return err
	}
	kindex := new(uint32)
	*kindex = uint32(pk.index)
	tx := &models.AdminTx{
		Txtype:              models.TxType_ADD_PROCESS_KEYS,
		KeyIndex:            kindex,
		Nonce:               uint32(util.RandomInt(0, 1000000000)),
		ProcessId:           pid,
		EncryptionPublicKey: pk.pubKey,
	}
	return k.signAndSendTx(tx)
}

// revealKeys reveals the private keys for the given encrypted process.
func (k *KeyKeeper) revealKeys(pid types.HexBytes) error {
	log.Infow("revealing encryption key", "processId", pid.String(), "keyIndex", k.myIndex)
	pk, err := k.generateKeys(pid)
	if err != nil {
		return err
	}
	kindex := new(uint32)
	*kindex = uint32(pk.index)
	tx := &models.AdminTx{
		Txtype:               models.TxType_REVEAL_PROCESS_KEYS,
		KeyIndex:             kindex,
		Nonce:                uint32(util.RandomInt(0, 1000000000)),
		ProcessId:            pid,
		EncryptionPrivateKey: pk.privKey,
	}
	return k.signAndSendTx(tx)
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
	log.Debugw("sending keykeeper transaction", "tx", log.FormatProto(tx))
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

// OnVote is not used by the KeyKeeper
func (*KeyKeeper) OnVote(_ *state.Vote, _ int32) {}

// OnNewTx is not used by the KeyKeeper
func (*KeyKeeper) OnNewTx(_ *vochaintx.Tx, _ uint32, _ int32) {}

// OnBeginBlock is not used by the KeyKeeper
func (*KeyKeeper) OnBeginBlock(_ state.BeginBlock) {}

// OnCensusUpdate is not used by the KeyKeeper
func (*KeyKeeper) OnCensusUpdate(_, _ []byte, _ string, _ uint64) {}

// OnCancel does nothing
func (k *KeyKeeper) OnCancel(_ []byte, _ int32) {}

// OnProcessDurationChange does nothing
func (k *KeyKeeper) OnProcessDurationChange(_ []byte, _ uint32, _ int32) {}
