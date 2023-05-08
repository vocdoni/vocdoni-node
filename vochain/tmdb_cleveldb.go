//go:build cleveldb
// +build cleveldb

package vochain

import tmdb "github.com/cometbft/cometbft-db"

const tmdbBackend = tmdb.CLevelDBBackend
