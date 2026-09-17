// add_validator promotes an existing external public key into a Vocdoni
// validator via a SET_ACCOUNT_VALIDATOR transaction.
//
// Bootstrapping challenge: on some networks (notably dev) the default faucet
// hands out fewer tokens per address than SetAccountValidator costs, and is
// rate-limited per destination address, so a single freshly-claimed wallet
// cannot afford the tx. This tool works around it by spinning up two ephemeral
// wallets, claiming from the faucet for both, transferring the pooled balance
// into the second, and signing the SetValidator with the pooled wallet.
//
// Usage:
//
//	go run ./cmd/tools/add_validator \
//	  -api https://api-dev.vocdoni.net/v2 \
//	  -pubkey 03503c0872bdcd804b1635cf187577ca1caddbbb14ec8eb3af68579fe4bedcf071 \
//	  -name miner3
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"go.vocdoni.io/dvote/apiclient"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
)

func main() {
	apiURL := flag.String("api", "https://api-dev.vocdoni.net/v2", "Vocdoni API URL")
	pubKeyHex := flag.String("pubkey", "", "hex-encoded secp256k1 public key of the validator to add")
	name := flag.String("name", "", "display name for the new validator")
	transferAmount := flag.Uint64("transfer", 9998, "tokens to move from signer A to signer B (must cover SetValidator cost after B's own faucet claim)")
	timeout := flag.Duration("timeout", 90*time.Second, "per-tx wait timeout")
	flag.Parse()

	if *pubKeyHex == "" {
		log.Fatal("-pubkey is required")
	}
	pubKey, err := hex.DecodeString(*pubKeyHex)
	if err != nil {
		log.Fatalf("invalid -pubkey hex: %v", err)
	}

	base, err := apiclient.New(*apiURL)
	if err != nil {
		log.Fatalf("cannot connect to API %s: %v", *apiURL, err)
	}
	log.Printf("connected: chainID=%s", base.ChainID())

	signerA := &ethereum.SignKeys{}
	if err := signerA.Generate(); err != nil {
		log.Fatalf("cannot generate signer A: %v", err)
	}
	signerB := &ethereum.SignKeys{}
	if err := signerB.Generate(); err != nil {
		log.Fatalf("cannot generate signer B: %v", err)
	}
	log.Printf("signer A = %s", signerA.AddressString())
	log.Printf("signer B = %s", signerB.AddressString())

	clientA := base.Clone(fmt.Sprintf("%x", signerA.PrivateKey()))
	clientB := base.Clone(fmt.Sprintf("%x", signerB.PrivateKey()))

	if err := bootstrap(clientA, "A", *timeout); err != nil {
		log.Fatalf("bootstrap A: %v", err)
	}
	if err := bootstrap(clientB, "B", *timeout); err != nil {
		log.Fatalf("bootstrap B: %v", err)
	}

	log.Printf("transferring %d tokens A -> B ...", *transferAmount)
	hash, err := clientA.Transfer(signerB.Address(), *transferAmount)
	if err != nil {
		log.Fatalf("Transfer A->B: %v", err)
	}
	if err := waitTx(clientA, hash, *timeout); err != nil {
		log.Fatalf("wait Transfer A->B: %v", err)
	}
	log.Printf("Transfer confirmed, hash=%s", hash.String())

	log.Printf("submitting SET_ACCOUNT_VALIDATOR for pubkey=%s name=%q ...", *pubKeyHex, *name)
	hash, err = clientB.AccountSetValidator(pubKey, *name)
	if err != nil {
		log.Fatalf("AccountSetValidator: %v", err)
	}
	if err := waitTx(clientB, hash, *timeout); err != nil {
		log.Fatalf("wait SetValidator: %v", err)
	}
	log.Printf("SetValidator confirmed, hash=%s", hash.String())

	fmt.Fprintln(os.Stdout, hash.String())
}

func bootstrap(c *apiclient.HTTPclient, label string, timeout time.Duration) error {
	addr := c.MyAddress().Hex()
	log.Printf("requesting faucet for %s (%s)", label, addr)
	pkg, err := apiclient.GetFaucetPackageFromDefaultService(addr, c.ChainID())
	if err != nil {
		return fmt.Errorf("faucet: %w", err)
	}
	meta := &vapi.AccountMetadata{
		Name:        map[string]string{"default": "add_validator " + label + " " + addr},
		Description: map[string]string{"default": "ephemeral signer for SET_ACCOUNT_VALIDATOR"},
		Version:     "1.0",
	}
	hash, err := c.AccountBootstrap(pkg, meta, nil)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	if err := waitTx(c, hash, timeout); err != nil {
		return fmt.Errorf("wait bootstrap: %w", err)
	}
	acc, err := c.Account("")
	if err != nil {
		return fmt.Errorf("account: %w", err)
	}
	log.Printf("signer %s ready: balance=%d nonce=%d", label, acc.Balance, acc.Nonce)
	return nil
}

func waitTx(c *apiclient.HTTPclient, hash []byte, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := c.WaitUntilTxIsMined(ctx, hash)
	return err
}

