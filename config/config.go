package config

// Config stores global configurations for vocdoni-node
type Config struct {
	// Vochain config options
	Vochain VochainCfg
	// LogLevel logging level
	LogLevel string
	// LogOutput logging output
	LogOutput string
	// LogErrorFile for logging warning, error and fatal messages
	LogErrorFile string
	// DataDir path where the gateway files will be stored
	DataDir string
	// SigningKey key used to sign transactions
	SigningKey string
	// Mode describes the operation mode of program
	Mode string
	// Dev enables the development mode (less security)
	Dev bool
	// PprofPort is the port where pprof http endpoint listen if dev enabled
	PprofPort int
	// ListenPort port where the API server will listen on
	ListenPort int
	// ListenHost host where the API server will listen on
	ListenHost string
	// TLS related config options
	TLS struct {
		Domain  string
		DirCert string
	}
	// EnableAPI enables the HTTP API REST service
	EnableAPI bool
	// AdminAPIToken is the token used to authenticate admin API requests
	AdminToken string
	// EnableFaucet enables the faucet API service for the given amounts
	EnableFaucetWithAmount uint64
}

// VochainCfg includes all possible config params needed by the Vochain
type VochainCfg struct {
	// DataDir directory where the Vochain related data is stored
	DataDir string
	// DBType is the type of key-value db to be used
	DBType string

}
