package faucet

// FaucetResponse represents the message on the response of a faucet request
type FaucetResponse struct {
	// Amount transferred
	Amount string `json:"amount,omitempty"`
	// FaucetPackage represents the faucet package
	FaucetPackage []byte `json:"faucetPackage,omitempty"`
}

// FaucetPackage represents the data of a faucet package
type FaucetPackage struct {
	// FaucetPackagePayload is the Vocdoni faucet package payload
	FaucetPayload []byte `json:"faucetPayload"`
	// Signature is the signature for the vocdoni faucet payload
	Signature []byte `json:"signature"`
}
