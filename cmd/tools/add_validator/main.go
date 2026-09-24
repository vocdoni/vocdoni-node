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

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
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
	// Normalize to 33-byte compressed secp256k1: registration eventually
	// hands this into CometBFT's secp256k1 pub-key type, which rejects the
	// 65-byte uncompressed form even though ethereum.AddrFromPublicKey
	// accepts both. Reject other lengths outright.
	switch len(pubKey) {
	case 33:
		if _, err := ethcrypto.DecompressPubkey(pubKey); err != nil {
			log.Fatalf("invalid 33-byte compressed pubkey: %v", err)
		}
	case 65:
		ecdsaPub, err := ethcrypto.UnmarshalPubkey(pubKey)
		if err != nil {
			log.Fatalf("cannot parse 65-byte uncompressed pubkey: %v", err)
		}
		pubKey = ethcrypto.CompressPubkey(ecdsaPub)
	default:
		log.Fatalf("-pubkey must be 33-byte compressed or 65-byte uncompressed secp256k1; got %d bytes", len(pubKey))
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

	// Check the plan against the live chain before spending anything. The
	// -transfer default encodes dev-network arithmetic; stage/lts/prod price
	// SET_ACCOUNT_VALIDATOR far higher, so pointing -api elsewhere with
	// default flags would strand both faucet claims. Nothing is submitted
	// until the check passes: once the bootstrap txs land, the funds sit
	// behind ephemeral keys that die with this process, and the faucet is
	// rate-limited per address, so a late failure means waiting out the
	// limiter as well.
	createCost, err := base.TransactionCost(models.TxType_CREATE_ACCOUNT)
	if err != nil {
		log.Fatalf("cannot fetch CREATE_ACCOUNT cost: %v", err)
	}
	sendCost, err := base.TransactionCost(models.TxType_SEND_TOKENS)
	if err != nil {
		log.Fatalf("cannot fetch SEND_TOKENS cost: %v", err)
	}
	validatorCost, err := base.TransactionCost(models.TxType_SET_ACCOUNT_VALIDATOR)
	if err != nil {
		log.Fatalf("cannot fetch SET_ACCOUNT_VALIDATOR cost: %v", err)
	}
	pkgA, amountA, err := faucetPackage(clientA, "A")
	if err != nil {
		log.Fatalf("faucet A: %v", err)
	}
	pkgB, amountB, err := faucetPackage(clientB, "B")
	if err != nil {
		log.Fatalf("faucet B: %v", err)
	}
	log.Printf("costs on %s: CREATE_ACCOUNT=%d SEND_TOKENS=%d SET_ACCOUNT_VALIDATOR=%d; faucet: A=%d B=%d",
		base.ChainID(), createCost, sendCost, validatorCost, amountA, amountB)

	// CREATE_ACCOUNT is paid out of the faucet amount it redeems.
	if amountA < createCost || amountB < createCost {
		log.Fatalf("faucet amounts (A=%d B=%d) do not cover CREATE_ACCOUNT cost %d", amountA, amountB, createCost)
	}
	balA, balB := amountA-createCost, amountB-createCost
	if balB+*transferAmount < validatorCost {
		log.Fatalf("B would hold %d (%d claimed + %d transferred) but SET_ACCOUNT_VALIDATOR costs %d; "+
			"re-run with -transfer %d",
			balB+*transferAmount, balB, *transferAmount, validatorCost,
			validatorCost-balB)
	}
	if balA < *transferAmount+sendCost {
		log.Fatalf("A would hold %d but sending %d costs %d more in fees (%d total); "+
			"lower -transfer or use a network with a larger faucet",
			balA, *transferAmount, sendCost, *transferAmount+sendCost)
	}

	if err := bootstrap(clientA, "A", pkgA, *timeout); err != nil {
		log.Fatalf("bootstrap A: %v", err)
	}
	if err := bootstrap(clientB, "B", pkgB, *timeout); err != nil {
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

	// Log the normalized key, not the raw flag: a 65-byte uncompressed input
	// was compressed above, so the flag value would misreport what went on-chain.
	log.Printf("submitting SET_ACCOUNT_VALIDATOR for pubkey=%x name=%q ...", pubKey, *name)
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

// faucetPackage fetches a faucet package for c's address and returns it with
// the amount it carries. Fetching is off-chain; nothing is spent until
// bootstrap redeems the package.
func faucetPackage(c *apiclient.HTTPclient, label string) (*models.FaucetPackage, uint64, error) {
	addr := c.MyAddress().Hex()
	log.Printf("requesting faucet for %s (%s)", label, addr)
	pkg, err := apiclient.GetFaucetPackageFromDefaultService(addr, c.ChainID())
	if err != nil {
		return nil, 0, err
	}
	payload := &models.FaucetPayload{}
	if err := proto.Unmarshal(pkg.Payload, payload); err != nil {
		return nil, 0, fmt.Errorf("cannot decode faucet payload: %w", err)
	}
	return pkg, payload.Amount, nil
}

func bootstrap(c *apiclient.HTTPclient, label string, pkg *models.FaucetPackage, timeout time.Duration) error {
	addr := c.MyAddress().Hex()
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
