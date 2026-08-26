package api

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/indexer"
)

// The election title and the organization name/avatar are not on chain: only the
// URI pointing at them is. They are cached into the indexer so that list
// endpoints can render them without one detail request per row.
//
// Of the two places that could fill that cache, this is the one that already has
// the data in hand. The detail handlers (electionHandler, accountHandler) fetch
// the metadata from off-chain storage and parse it into ElectionMetadata /
// AccountMetadata on every request already, so caching it costs a write and no
// extra fetch. The alternative, hooking vochain/offchaindatahandler, would mean
// teaching a component that today only pins bytes about both metadata schemas and
// giving it an indexer handle it currently has no reason to hold; and it only
// reacts to new on-chain events, so it would never populate anything historical.
// Indexing the account name from the SetAccount transaction is not an option
// either: the transaction carries the infoURI, not the name.
//
// The write is fire-and-forget on a separate goroutine, because it contends for
// the same single read-write connection that block commits use, and no request
// should ever wait on a cache write.

const (
	// metadataBackfillBatch is how many rows the backfill claims at a time. It
	// re-queries after each batch, so rows filled meanwhile are not revisited.
	metadataBackfillBatch = 500
	// metadataBackfillWorkers bounds the concurrent resolutions, so the backfill
	// cannot saturate the storage layer while the node is also serving requests.
	metadataBackfillWorkers = 4
	// metadataBackfillTimeout is the per-item budget for resolving metadata. It
	// is deliberately short: the backfill is meant to pick up what the local
	// store already has, not to wait on the network for what it does not.
	metadataBackfillTimeout = time.Second
	// metadataBackfillMaxFailures caps how many unresolved fetches a single
	// backfill pass tolerates before giving up early. Without a bound, a
	// backlog of permanently unreachable metadata documents gets re-fetched
	// over the network in full on every boot.
	metadataBackfillMaxFailures = 200
	// metadataCacheWriteWorkers bounds the number of detached goroutines that
	// may be writing request-triggered cache updates at once, so a burst of
	// requests cannot pile up writers against the single read-write connection
	// that block commits also use.
	metadataCacheWriteWorkers = 4
)

// metadataCacheWriteSem bounds concurrent request-triggered cache writes; see
// metadataCacheWriteWorkers.
var metadataCacheWriteSem = make(chan struct{}, metadataCacheWriteWorkers)

// cacheWrite runs fn on a detached goroutine if the concurrent-write bound
// allows it, and drops it otherwise: the cache will be filled by a future
// request or by the next boot's backfill, so no caller needs to wait or queue.
func cacheWrite(fn func()) {
	select {
	case metadataCacheWriteSem <- struct{}{}:
		go func() {
			defer func() { <-metadataCacheWriteSem }()
			fn()
		}()
	default:
	}
}

// languageString picks a displayable value out of a multi-language string,
// preferring the default language and falling back to any English variant.
// Returns "" if the map holds nothing usable.
func languageString(ls LanguageString) string {
	for _, key := range []string{"default", "en", "en-US", "en_US"} {
		if v := ls[key]; v != "" {
			return v
		}
	}
	return ""
}

// cacheElectionTitle stores an already resolved election title in the indexer.
func (a *API) cacheElectionTitle(electionID []byte, metadata *ElectionMetadata) {
	title := languageString(metadata.Title)
	if title == "" || a.indexer == nil {
		return
	}
	id := append([]byte(nil), electionID...)
	cacheWrite(func() {
		if err := a.indexer.SetProcessMetadataTitle(id, title); err != nil {
			log.Warnw("could not cache election title", "electionId", id, "err", err.Error())
		}
	})
}

// cacheAccountMetadata stores an already resolved organization name and avatar
// in the indexer.
func (a *API) cacheAccountMetadata(address []byte, metadata *AccountMetadata) {
	if metadata == nil || a.indexer == nil {
		return
	}
	name := languageString(metadata.Name)
	avatar := ""
	if metadata.Media != nil {
		avatar = metadata.Media.Avatar
	}
	if name == "" && avatar == "" {
		return
	}
	addr := append([]byte(nil), address...)
	cacheWrite(func() {
		if err := a.indexer.SetAccountMetadata(addr, name, avatar); err != nil {
			log.Warnw("could not cache account metadata", "account", addr, "err", err.Error())
		}
	})
}

// startMetadataBackfill launches, once per boot, a best-effort pass filling the
// cached titles and names of everything indexed before they were captured.
//
// It never blocks startup and never blocks on the network: the data is off-chain,
// so a missing or unreachable metadata document simply leaves the row empty and
// the client falls back to its own per-row lookup. What is resolvable is normally
// already in the local store, which the off-chain data handler pins as elections
// and accounts appear.
func (a *API) startMetadataBackfill() {
	if a.indexer == nil || a.storage == nil {
		return
	}
	go func() {
		startTime := time.Now()
		elections := a.backfillElectionTitles()
		accounts := a.backfillAccountMetadata()
		if elections+accounts > 0 {
			log.Infow("metadata backfill finished",
				"elections", elections, "accounts", accounts, "took", time.Since(startTime).String())
		}
	}()
}

// backfillWorkers runs fn over items with a bounded number of goroutines, and
// reports how many calls reported having filled something.
func backfillWorkers[T any](items []T, fn func(T) bool) int {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		filled  int
		itemsCh = make(chan T)
	)
	for range metadataBackfillWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemsCh {
				if fn(item) {
					mu.Lock()
					filled++
					mu.Unlock()
				}
			}
		}()
	}
	for _, item := range items {
		itemsCh <- item
	}
	close(itemsCh)
	wg.Wait()
	return filled
}

// retrieveMetadata fetches an off-chain metadata document with the short
// backfill budget, and unmarshals it into v. Reports whether it succeeded.
func (a *API) retrieveMetadata(uri string, v any) bool {
	if uri == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), metadataBackfillTimeout)
	defer cancel()
	data, err := a.storage.Retrieve(ctx, uri, MaxOffchainFileSize)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

// backfillElectionTitles resolves the title of the elections which don't have one
// cached, and returns how many were filled.
func (a *API) backfillElectionTitles() int {
	filled := 0
	failures := 0
	var after []byte
	for {
		pending, err := a.indexer.ProcessesMissingMetadataTitle(after, metadataBackfillBatch)
		if err != nil {
			log.Warnw("could not list elections missing a title", "err", err.Error())
			return filled
		}
		if len(pending) == 0 {
			return filled
		}
		after = pending[len(pending)-1].ProcessID
		batchFilled := backfillWorkers(pending, func(p indexer.ProcessMetadataURI) bool {
			metadata := ElectionMetadata{}
			if !a.retrieveMetadata(p.URI, &metadata) {
				return false
			}
			title := languageString(metadata.Title)
			if title == "" {
				return false
			}
			if err := a.indexer.SetProcessMetadataTitle(p.ProcessID, title); err != nil {
				log.Warnw("could not backfill election title", "electionId", p.ProcessID, "err", err.Error())
				return false
			}
			return true
		})
		filled += batchFilled
		failures += len(pending) - batchFilled
		if failures >= metadataBackfillMaxFailures {
			log.Infow("metadata backfill stopping early, too many unresolved election titles",
				"filled", filled, "failures", failures)
			return filled
		}
	}
}

// backfillAccountMetadata resolves the name and avatar of the accounts which
// don't have them cached, and returns how many were filled. The metadata URI
// comes from the account state, which is the only place that holds it.
func (a *API) backfillAccountMetadata() int {
	if a.vocapp == nil {
		return 0
	}
	filled := 0
	failures := 0
	var after []byte
	for {
		pending, err := a.indexer.AccountsMissingName(after, metadataBackfillBatch)
		if err != nil {
			log.Warnw("could not list accounts missing a name", "err", err.Error())
			return filled
		}
		if len(pending) == 0 {
			return filled
		}
		after = pending[len(pending)-1]
		batchFilled := backfillWorkers(pending, func(address []byte) bool {
			acc, err := a.vocapp.State.GetAccount(common.BytesToAddress(address), true)
			if err != nil || acc == nil {
				return false
			}
			metadata := AccountMetadata{}
			if !a.retrieveMetadata(acc.GetInfoURI(), &metadata) {
				return false
			}
			name := languageString(metadata.Name)
			avatar := ""
			if metadata.Media != nil {
				avatar = metadata.Media.Avatar
			}
			if name == "" && avatar == "" {
				return false
			}
			if err := a.indexer.SetAccountMetadata(address, name, avatar); err != nil {
				log.Warnw("could not backfill account metadata", "account", address, "err", err.Error())
				return false
			}
			return true
		})
		filled += batchFilled
		failures += len(pending) - batchFilled
		if failures >= metadataBackfillMaxFailures {
			log.Infow("metadata backfill stopping early, too many unresolved account metadata fetches",
				"filled", filled, "failures", failures)
			return filled
		}
	}
}
