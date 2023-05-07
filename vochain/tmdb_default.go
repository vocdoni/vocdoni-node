//go:build !cleveldb
// +build !cleveldb

// If -tags=cleveldb is not used, we must fall back to something included by
// tm-db by default. goleveldb is an OK choice, since it's pure Go and it's also
// their default.

package vochain

import tmdb "github.com/cometbft/cometbft-db"

const tmdbBackend = tmdb.GoLevelDBBackend
