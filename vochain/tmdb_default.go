//go:build !badgerdb && !cleveldb
// +build !badgerdb,!cleveldb

// If neither -tags=badger nor -tags=cleveldb are used, we must fall back to
// something included by tm-db by default. goleveldb is an OK choice, since it's
// pure Go and it's also their default.

package vochain

import tmdb "github.com/tendermint/tm-db"

const tmdbBackend = tmdb.GoLevelDBBackend
