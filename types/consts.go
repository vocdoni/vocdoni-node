package types

func Bool(b bool) *bool { return &b }

// These exported variables should be treated as constants, to be used in API
// responses which require *bool fields.
var (
	False = Bool(false)
	True  = Bool(true)
)

const (
	// ScrutinizerProcessPrefix is the prefix for the levelDB process keys
	ScrutinizerProcessPrefix = "p_"
	// ScrutinizerEntityPrefix is the prefix for the levelDB entity keys
	ScrutinizerEntityPrefix = "e_"
)
