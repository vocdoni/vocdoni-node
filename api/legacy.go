package api

//
// Legacy lists used to return an empty list (instead of null or an error)
//

func emptyElectionsList() any {
	return struct {
		List []any `json:"elections"`
	}{
		List: []any{},
	}
}

func emptyOrganizationsList() any {
	return struct {
		List []any `json:"organizations"`
	}{
		List: []any{},
	}
}

func emptyVotesList() any {
	return struct {
		List []any `json:"votes"`
	}{
		List: []any{},
	}
}

func emptyTransactionsList() any {
	return struct {
		List []any `json:"transactions"`
	}{
		List: []any{},
	}
}

func emptyFeesList() any {
	return struct {
		List []any `json:"fees"`
	}{
		List: []any{},
	}
}

func emptyTransfersList() any {
	return struct {
		List []any `json:"transfers"`
	}{
		List: []any{},
	}
}

func emptyAccountsList() any {
	return struct {
		List []any `json:"accounts"`
	}{
		List: []any{},
	}
}
