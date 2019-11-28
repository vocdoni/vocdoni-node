package main

import (
	"net/http"
	"os"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/net"

	censusmanager "gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
)

func newConfig() (config.CensusCfg, error) {
	var globalCfg config.CensusCfg
	//setup flags
	home, err := os.UserHomeDir()
	if err != nil {
		return globalCfg, err
	}
	defaultDirPath := home + "/.dvote/censushttp"
	//setup flags
	path := flag.String("cfgpath", defaultDirPath+"/config.yaml", "cfgpath. Specify filepath for censushttp config")
	flag.String("logLevel", "info", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.Int("port", 8080, "HTTP port to listen")
	flag.String("signKey", "", "Private key for signing API response messages (ECDSA)")
	flag.String("sslDomain", "", "Enables HTTPs using a LetsEncrypt certificate")
	flag.String("dataDir", defaultDirPath, "Use a custom dir for storing the run time data")
	flag.String("rootKey", "", "Public ECDSA key allowed to create new Census")

	viper := viper.New()
	viper.SetDefault("logLevel", "info")
	flag.Parse()
	viper.SetConfigType("yaml")
	if *path == defaultDirPath+"/config.yaml" {
		//if path left default, write new cfg file if empty or if file doesn't exist.
		if err = viper.SafeWriteConfigAs(*path); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(defaultDirPath, os.ModePerm)
				if err != nil {
					return globalCfg, err
				}
				err = viper.WriteConfigAs(*path)
				if err != nil {
					return globalCfg, err
				}
			}
		}
	}

	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("port", flag.Lookup("port"))
	viper.BindPFlag("signKey", flag.Lookup("signKey"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("sslDomain", flag.Lookup("sslDomain"))
	viper.BindPFlag("rootKey", flag.Lookup("rootKey"))

	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

func addCorsHeaders(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLogger(globalCfg.LogLevel, "stdout")
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	// Signing key
	signer := new(signature.SignKeys)

	// Add signing private key if exist in configuration or flags
	if len(globalCfg.SignKey) > 1 {
		log.Infof("adding signing key")
		err := signer.AddHexKey(globalCfg.SignKey)
		if err != nil {
			log.Fatalf("Fatal error adding hex key: %v", err)
		}
		pub, _ := signer.HexString()
		log.Infof("using custom pubKey %s", pub)
	} else {
		log.Warn("no signing key provided, generating one...")
		signer.Generate()
		pub, priv := signer.HexString()
		log.Infof("Public: %s Private: %s", pub, priv)
	}
	var cm censusmanager.CensusManager
	err = cm.Init(globalCfg.DataDir, globalCfg.RootKey)
	if err != nil {
		log.Fatalf("cannot initialize census manager: %s", err)
	}
	for i := 0; i < len(cm.Census.Namespaces); i++ {
		log.Infof("loaded namespace %s", cm.Census.Namespaces[i].Name)
	}

	pxy := net.NewProxy()
	pxy.C.SSLDomain = globalCfg.SslDomain
	pxy.C.SSLCertDir = globalCfg.DataDir
	log.Infof("storing SSL certificate in %s", pxy.C.SSLCertDir)
	pxy.C.Address = "0.0.0.0"
	pxy.C.Port = globalCfg.Port
	err = pxy.Init()
	if err != nil {
		log.Warn("letsEncrypt SSL certificate cannot be obtained, probably port 443 is not accessible or domain provided is not correct")
		log.Info("disabling SSL!")
		// Probably SSL has failed
		pxy.C.SSLDomain = ""
		globalCfg.SslDomain = ""
		err = pxy.Init()
		if err != nil {
			log.Fatal(err)
		}
	}

	pxy.AddHandler("/", func(w http.ResponseWriter, r *http.Request) {
		addCorsHeaders(&w, r)
		if r.Method == http.MethodPost {
			cm.HTTPhandler(w, r, signer)
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})

	// Block forever.
	select {}
}
