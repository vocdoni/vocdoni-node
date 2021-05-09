//go:build cleveldb
// +build cleveldb

package vochain

import tmdb "github.com/tendermint/tm-db"

const tmdbBackend = tmdb.CLevelDBBackend
