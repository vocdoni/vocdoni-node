package main

import (
	"os"
	"os/user"
	"strings"

	flag "github.com/spf13/pflag"
	viper "github.com/spf13/viper"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	censusmanager "gitlab.com/vocdoni/go-dvote/service/census"
)

func newConfig() (config.CensusCfg, error) {
	var globalCfg config.CensusCfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	defaultDirPath := usr.HomeDir + "/.dvote/censushttp"
	//setup flags
	path := flag.String("cfgpath", defaultDirPath+"/config.yaml", "cfgpath. Specify filepath for censushttp config")
	flag.String("logLevel", "info", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.Int("port", 8080, "HTTP port to listen")
	flag.String("namespaces", "", "Namespace and/or allowed public keys, syntax is <namespace>[:pubKey],[<namespace>[:pubKey]],...")
	flag.String("signKey", "", "Private key for signing API response messages (ECDSA)")

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
	viper.BindPFlag("namespaces", flag.Lookup("namespaces"))
	viper.BindPFlag("signingKey", flag.Lookup("signKey"))

	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	// Signing key
	var signer *signature.SignKeys
	signer = new(signature.SignKeys)

	// Add signing private key if exist in configuration or flags
	if len(globalCfg.SigningKey) > 1 {
		log.Infof("adding signing key")
		err := signer.AddHexKey(globalCfg.SigningKey)
		if err != nil {
			log.Fatalf("Fatal error adding hex key: %v", err.Error())
		}
		pub, _ := signer.HexString()
		log.Infof("using custom pubKey %s", pub)
	} else {
		log.Warn("no signing key provided, generating one...")
		signer.Generate()
		pub, priv := signer.HexString()
		log.Infof("Public: %s | Private: %s", pub, priv)
	}

	port := globalCfg.Port
	for i := 0; i < len(globalCfg.Namespaces); i++ {
		s := strings.Split(globalCfg.Namespaces[i], ":")
		ns := s[0]
		pubK := ""
		if len(s) > 1 {
			pubK = s[1]
			log.Infof("public Key authentication enabled on namespace %s", ns)
		}
		censusmanager.AddNamespace(ns, pubK)
		log.Infof("starting process HTTP service on port %d for namespace %s", port, ns)
	}
	censusmanager.Listen(port, "http", signer)

}
