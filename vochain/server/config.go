package server

import (
	"github.com/spf13/viper"
	tcfg "github.com/tendermint/tendermint/config"
)

type Config tcfg.Config

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) parseDefaultConfig() (*tcfg.Config, error) {
	conf := createDefaultConfig()
	err := viper.Unmarshal(conf)
	return conf, err
}

func (c *Config) createDefaultConfig() *tcfg.Config {
	return tcfg.DefaultConfig()
}
