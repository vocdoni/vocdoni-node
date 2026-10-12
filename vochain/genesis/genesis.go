package genesis

// DefaultTransactionCosts returns a default set of transaction costs to use as template.
func DefaultTransactionCosts() TransactionCosts {
	return TransactionCosts{
		SetProcessStatus:        2,
		SetProcessCensus:        2,
		SetProcessDuration:      2,
		SetProcessQuestionIndex: 1,
		RegisterKey:             1,
		NewProcess:              5,
		SendTokens:              1,
		SetAccountInfoURI:       1,
		CreateAccount:           1,
		AddDelegateForAccount:   1,
		DelDelegateForAccount:   1,
		CollectFaucet:           1,
		SetAccountValidator:     10000,
	}
}
