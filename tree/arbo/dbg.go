package arbo

import "fmt"

//lint:file-ignore U1000 this code is for debugging

// dbgStats is for debug purposes
type dbgStats struct {
	hash  int // TODO use atomics for all ints in dbgStats
	dbGet int
	dbPut int
}

func (t *Tree) dbgInit() {
	t.dbg = newDbgStats()
}

func newDbgStats() *dbgStats {
	return &dbgStats{
		hash:  0,
		dbGet: 0,
		dbPut: 0,
	}
}

func (d *dbgStats) incHash() {
	if d == nil {
		return
	}
	d.hash++
}

func (d *dbgStats) incDbGet() {
	if d == nil {
		return
	}
	d.dbGet++
}

func (d *dbgStats) incDbPut() {
	if d == nil {
		return
	}
	d.dbPut++
}

func (d *dbgStats) add(d2 *dbgStats) {
	if d == nil || d2 == nil {
		return
	}
	d.hash += d2.hash
	d.dbGet += d2.dbGet
	d.dbPut += d2.dbPut
}

func (d *dbgStats) print(prefix string) {
	if d == nil {
		return
	}
	fmt.Printf("%sdbgStats(hash: %s, dbGet: %s, dbPut: %s)\n",
		prefix, formatK(d.hash), formatK(d.dbGet), formatK(d.dbPut))
}

func formatK(v int) string {
	if v/1000 > 0 {
		return fmt.Sprintf("%.3fk", float64(v)/1000)
	}
	return fmt.Sprintf("%d", v)
}
