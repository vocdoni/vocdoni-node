package state

import "fmt"

var (
	ErrVoteDoesNotExist     = fmt.Errorf("vote does not exist")
	ErrProcessNotFound      = fmt.Errorf("process not found")
	ErrAccountNonceInvalid  = fmt.Errorf("invalid account nonce")
	ErrAccountNotExist      = fmt.Errorf("account does not exist")
	ErrBalanceOverflow      = fmt.Errorf("balance overflow")
	ErrNotEnoughBalance     = fmt.Errorf("not enough balance to transfer")
	ErrAccountBalanceZero   = fmt.Errorf("zero balance account not valid")
	ErrAccountAlreadyExists = fmt.Errorf("account already exists")
	ErrInvalidURILength     = fmt.Errorf("invalid URI length")

/*
ErrInvalidAddress       = fmt.Errorf("invalid address")
ErrNilTx                = fmt.Errorf("nil transaction")
*/
)
