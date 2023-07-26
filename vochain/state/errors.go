package state

import "fmt"

var (
	ErrVoteNotFound         = fmt.Errorf("vote not found")
	ErrProcessNotFound      = fmt.Errorf("process not found")
	ErrAccountNonceInvalid  = fmt.Errorf("invalid account nonce")
	ErrAccountNotExist      = fmt.Errorf("account does not exist")
	ErrBalanceOverflow      = fmt.Errorf("balance overflow")
	ErrNotEnoughBalance     = fmt.Errorf("not enough balance to transfer")
	ErrAccountBalanceZero   = fmt.Errorf("zero balance account not valid")
	ErrAccountAlreadyExists = fmt.Errorf("account already exists")
	ErrInvalidURILength     = fmt.Errorf("invalid URI length")
	ErrRegisteredValidSIK   = fmt.Errorf("address already has a valid sik")
	ErrSIKAlreadyInvalid    = fmt.Errorf("sik is already invalidated")
	ErrSIKSubTree           = fmt.Errorf("error getting SIK deep sub tree")
	ErrSIKGet               = fmt.Errorf("error getting SIK")
	ErrSIKSet               = fmt.Errorf("error setting new SIK")
	ErrSIKDelete            = fmt.Errorf("error deleting new SIK")
	ErrSIKRootsGet          = fmt.Errorf("error getting current valid SIK root")
	ErrSIKRootsSet          = fmt.Errorf("error setting new SIK roots")
	ErrSIKRootsDelete       = fmt.Errorf("error deleting old SIK roots")
)
