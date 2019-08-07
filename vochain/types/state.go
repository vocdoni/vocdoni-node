package vochain

// State represents a volatile state for Vochain
type State struct {
	store Store
	ctx   Context
}

// Context return the State context
func (st *State) Context() Context {
	return st.ctx
}

// Store returns the State store
func (st *State) Store() Store {
	return st.store
}
