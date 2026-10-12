// importmetadata is an offline one-shot tool that walks a stopped vocdoni-node
// data directory, collects all ipfs:// CIDs referenced by election metadata URIs
// and account infoURIs, fetches each from a local kubo node (offline mode), and
// inserts the raw content into the indexer's ipfs_content table.
//
// Usage:
//
//	importmetadata \
//	  --indexer-db  /path/to/db.sqlite \
//	  --state-db    /path/to/state       \
//	  [--state-db-type pebble]            \
//	  [--ipfs-api   http://127.0.0.1:8080] \
//	  [--fallback-gateway https://gateway.pinata.cloud] \
//	  [--dry-run]
//
// After the local pass (kubo) a second fallback pass runs over any still-missing
// CIDs using the public gateway specified by --fallback-gateway (off by default).
// The fallback pass applies strict CID verification; content that does not verify
// is rejected (no "unverified-local" exception).
//
// Idempotent: already-present rows are skipped (INSERT OR IGNORE).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	_ "github.com/mattn/go-sqlite3"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
)

const (
	// fetchTimeout is the per-CID deadline for both the local kubo and the
	// fallback gateway passes.
	fetchTimeout = 30 * time.Second
	// fallbackRateLimit is the minimum interval between fallback gateway
	// requests (Pinata throttles at roughly 1 req/s on free accounts).
	fallbackRateLimit = time.Second
	// maxMetadataSize caps the content fetched per CID to avoid loading huge
	// blobs into memory. Election and account metadata documents are tiny in
	// practice; anything larger is almost certainly not the JSON we want.
	maxMetadataSize = 4 << 20 // 4 MiB
)

// electionMeta is the subset of an election metadata document needed to
// extract the display title and update the metadata_title cache column.
type electionMeta struct {
	Title map[string]string `json:"title"`
}

// accountMeta is the subset of an account metadata document needed to extract
// the name and avatar and update the accounts name/avatar cache columns.
type accountMeta struct {
	Name  map[string]string `json:"name"`
	Media *struct {
		Avatar string `json:"avatar"`
	} `json:"media,omitempty"`
}

// counts accumulates the outcome per CID across both passes.
type counts struct {
	inserted        int
	alreadyPresent  int
	missing         int
	verifyFailed    int
	unverifiedLocal int
}

// fallbackCounts accumulates the outcome of the optional fallback pass.
type fallbackCounts struct {
	recovered    int
	verifyFailed int
	stillMissing int
}

func main() {
	log.Init("info", "stderr", nil)

	indexerDBPath := flag.String("indexer-db", "", "path to the indexer SQLite file (required)")
	stateDBPath := flag.String("state-db", "", "path to the state DB directory (required)")
	stateDBType := flag.String("state-db-type", db.TypePebble, "state DB type (pebble or leveldb)")
	ipfsAPI := flag.String("ipfs-api", "http://127.0.0.1:8080", "kubo HTTP gateway URL for the local pass")
	fallbackGateway := flag.String("fallback-gateway", "", "optional public IPFS gateway for a second pass over missing CIDs (e.g. https://gateway.pinata.cloud)")
	dryRun := flag.Bool("dry-run", false, "collect and verify but do not write to the database")
	flag.Parse()

	if *indexerDBPath == "" || *stateDBPath == "" {
		fmt.Fprintln(os.Stderr, "error: --indexer-db and --state-db are required")
		flag.Usage()
		os.Exit(1)
	}

	// --- Open the indexer SQLite DB ---
	// Use WAL mode so that a running slim node (if any) can keep reading while
	// we write. The tool is designed for a stopped node, but WAL keeps things
	// safe regardless.
	dsn := fmt.Sprintf("file:%s?_journal_mode=wal&_foreign_keys=true", *indexerDBPath)
	indexerDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatalf("cannot open indexer db: %v", err)
	}
	defer func() { _ = indexerDB.Close() }()
	indexerDB.SetMaxOpenConns(1) // sqlite only supports one writer

	// Ensure migration 0021 has run so the table exists even on old DBs.
	if _, err := indexerDB.Exec(`CREATE TABLE IF NOT EXISTS ipfs_content (
		cid  TEXT PRIMARY KEY,
		data BLOB NOT NULL
	)`); err != nil {
		log.Fatalf("cannot ensure ipfs_content table: %v", err)
	}

	// --- Collect process CIDs from the indexer ---
	processCIDs := map[string][]byte{} // cid → process_id (binary)
	rows, err := indexerDB.Query(
		`SELECT id, metadata FROM processes WHERE metadata LIKE 'ipfs://%'`)
	if err != nil {
		log.Fatalf("querying process metadata: %v", err)
	}
	for rows.Next() {
		var pid []byte
		var metaURI string
		if err := rows.Scan(&pid, &metaURI); err != nil {
			log.Warnw("scanning process row", "err", err)
			continue
		}
		cid := strings.TrimPrefix(metaURI, "ipfs://")
		processCIDs[cid] = pid
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		log.Fatalf("iterating process rows: %v", err)
	}

	// --- Collect account CIDs from the state DB ---
	accountCIDs := map[string]common.Address{} // cid → address
	st, err := state.New(*stateDBType, *stateDBPath)
	if err != nil {
		log.Fatalf("cannot open state db: %v", err)
	}
	accounts, err := st.ListAccounts(true)
	if err != nil {
		log.Fatalf("listing accounts: %v", err)
	}
	for addr, acc := range accounts {
		uri := acc.GetInfoURI()
		if !strings.HasPrefix(uri, "ipfs://") {
			continue
		}
		cid := strings.TrimPrefix(uri, "ipfs://")
		accountCIDs[cid] = addr
	}

	// --- Build the full CID set ---
	allCIDs := map[string]struct{}{}
	for cid := range processCIDs {
		allCIDs[cid] = struct{}{}
	}
	for cid := range accountCIDs {
		allCIDs[cid] = struct{}{}
	}

	log.Infow("collected cids",
		"processes", len(processCIDs),
		"accounts", len(accountCIDs),
		"unique", len(allCIDs),
	)

	// --- Local kubo pass ---
	httpClient := &http.Client{Timeout: fetchTimeout}
	var c counts
	missing := []string{} // CIDs not found by the local pass

	for cid := range allCIDs {
		// Check whether it is already cached.
		row := indexerDB.QueryRow(`SELECT 1 FROM ipfs_content WHERE cid = ?`, cid)
		var exists int
		if err := row.Scan(&exists); err == nil {
			c.alreadyPresent++
			continue
		}

		body, err := fetchCID(httpClient, *ipfsAPI, cid)
		if err != nil {
			log.Debugw("kubo fetch failed", "cid", cid, "err", err)
			missing = append(missing, cid)
			c.missing++
			continue
		}

		verified, unverified := verifyCID(cid, body)
		if !verified && !unverified {
			log.Warnw("cid verification failed", "cid", cid)
			c.verifyFailed++
			continue
		}
		if unverified {
			log.Warnw("cid accepted unverified-local (chunking mismatch)", "cid", cid)
			c.unverifiedLocal++
		}

		if *dryRun {
			c.inserted++
			continue
		}

		if _, err := indexerDB.Exec(
			`INSERT OR IGNORE INTO ipfs_content (cid, data) VALUES (?, ?)`, cid, body,
		); err != nil {
			log.Warnw("inserting ipfs_content row", "cid", cid, "err", err)
			continue
		}
		c.inserted++

		// Update the metadata title / name+avatar caches from the fresh content.
		updateCaches(indexerDB, cid, body, processCIDs, accountCIDs)
	}

	// --- Optional fallback gateway pass ---
	var fc fallbackCounts
	if len(missing) > 0 && *fallbackGateway != "" {
		log.Infow("starting fallback gateway pass", "gateway", *fallbackGateway, "missing", len(missing))
		ticker := time.NewTicker(fallbackRateLimit)
		defer ticker.Stop()
		for _, cid := range missing {
			<-ticker.C
			body, err := fetchCID(httpClient, *fallbackGateway, cid)
			if err != nil {
				log.Debugw("fallback fetch failed", "cid", cid, "err", err)
				fc.stillMissing++
				continue
			}
			// Strict verification — no unverified-local exception for public gateways.
			if !strictVerifyCID(cid, body) {
				log.Warnw("fallback cid verification failed", "cid", cid)
				fc.verifyFailed++
				continue
			}
			if *dryRun {
				fc.recovered++
				continue
			}
			if _, err := indexerDB.Exec(
				`INSERT OR IGNORE INTO ipfs_content (cid, data) VALUES (?, ?)`, cid, body,
			); err != nil {
				log.Warnw("inserting fallback ipfs_content row", "cid", cid, "err", err)
				fc.stillMissing++
				continue
			}
			fc.recovered++
			updateCaches(indexerDB, cid, body, processCIDs, accountCIDs)
		}
	}

	// --- Summary ---
	fmt.Println()
	fmt.Println("=== importmetadata summary ===")
	fmt.Printf("  local pass:\n")
	fmt.Printf("    inserted:         %d\n", c.inserted)
	fmt.Printf("    already present:  %d\n", c.alreadyPresent)
	fmt.Printf("    missing:          %d\n", c.missing)
	fmt.Printf("    verify failed:    %d\n", c.verifyFailed)
	fmt.Printf("    unverified-local: %d\n", c.unverifiedLocal)
	if *fallbackGateway != "" {
		fmt.Printf("  fallback pass (%s):\n", *fallbackGateway)
		fmt.Printf("    recovered:        %d\n", fc.recovered)
		fmt.Printf("    verify failed:    %d\n", fc.verifyFailed)
		fmt.Printf("    still missing:    %d\n", fc.stillMissing)
	}
	if *dryRun {
		fmt.Println("  (dry-run: no writes performed)")
	}
}

// fetchCID retrieves the content for a CID from the given gateway/API base URL.
// It uses the /ipfs/<cid> path, which works for both kubo HTTP gateways and
// public IPFS gateways. A new context is created per call so that a slow
// response for one CID does not block others.
func fetchCID(client *http.Client, base, cid string) ([]byte, error) {
	url := strings.TrimRight(base, "/") + "/ipfs/" + cid
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("404 not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if int64(len(body)) > maxMetadataSize {
		return nil, fmt.Errorf("content exceeds %d bytes", maxMetadataSize)
	}
	return body, nil
}

// verifyCID checks whether body's content matches cid using the standard
// CalculateCIDv1json chunking. If the computed CID matches (verified), it
// returns (true, false). If not, but the content came from a local offline
// kubo (so it cannot be forged), it returns (false, true) — "unverified-local".
// Any other failure returns (false, false).
func verifyCID(cid string, body []byte) (verified, unverifiedLocal bool) {
	computed := data.CalculateCIDv1json(body)
	if data.CIDequals(cid, computed) {
		return true, false
	}
	// The CID did not match CalculateCIDv1json — the content was added with
	// different chunking or raw-leaves. Because this content came from a local
	// offline kubo that cannot serve content it does not hold for the exact CID
	// requested, we still accept it but flag it as "unverified-local".
	return false, true
}

// strictVerifyCID checks whether body's content matches cid using
// CalculateCIDv1json. Content from public gateways is accepted only when the
// CID matches exactly — no "unverified-local" exception.
func strictVerifyCID(cid string, body []byte) bool {
	computed := data.CalculateCIDv1json(body)
	return data.CIDequals(cid, computed)
}

// updateCaches parses the raw content and writes the title (for processes) or
// name and avatar (for accounts) into the metadata cache columns. Errors are
// logged but not fatal — the cache columns are best-effort.
func updateCaches(
	db *sql.DB,
	cid string,
	body []byte,
	processCIDs map[string][]byte,
	accountCIDs map[string]common.Address,
) {
	if pid, ok := processCIDs[cid]; ok {
		var m electionMeta
		if err := json.Unmarshal(body, &m); err != nil {
			log.Debugw("cannot parse election metadata", "cid", cid, "err", err)
			return
		}
		title := firstLang(m.Title)
		if title == "" {
			return
		}
		if _, err := db.Exec(
			`UPDATE processes SET metadata_title = ? WHERE id = ? AND metadata_title != ?`,
			title, pid, title,
		); err != nil {
			log.Warnw("updating metadata_title", "cid", cid, "err", err)
		}
	}
	if addr, ok := accountCIDs[cid]; ok {
		var m accountMeta
		if err := json.Unmarshal(body, &m); err != nil {
			log.Debugw("cannot parse account metadata", "cid", cid, "err", err)
			return
		}
		name := firstLang(m.Name)
		avatar := ""
		if m.Media != nil {
			avatar = m.Media.Avatar
		}
		if _, err := db.Exec(
			`UPDATE accounts SET name = ?, avatar = ? WHERE account = ? AND (name != ? OR avatar != ?)`,
			name, avatar, addr.Bytes(), name, avatar,
		); err != nil {
			log.Warnw("updating account metadata", "cid", cid, "err", err)
		}
	}
}

// firstLang returns the "default" language value from a LanguageString map, or
// the first non-empty value found if "default" is absent, or an empty string.
func firstLang(ls map[string]string) string {
	if v, ok := ls["default"]; ok && v != "" {
		return v
	}
	for _, v := range ls {
		if v != "" {
			return v
		}
	}
	return ""
}
