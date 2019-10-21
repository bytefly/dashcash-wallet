package main

import (
	"gopkg.in/ini.v1"
	"strconv"
)

type Config struct {
	TestNet int
	RPCURL  string
	RPCUser string
	RPCPass string
	Port    int

	Xpub      string
	Xpriv     string
	AccountId int
	Index     uint32

	LastBlock    uint64
	FeeRate int64
	RegistryAddr string
}

var cfg *ini.File

func LoadConfiguration(filepath string) (*Config, error) {
	var err error
	cfg, err = ini.Load(filepath)
	if err != nil {
		return nil, err
	}

	config := new(Config)

	config.TestNet = cfg.Section("network").Key("testnet").MustInt(0)
	config.RPCURL = cfg.Section("network").Key("rpc_host").String()
	config.RPCUser = cfg.Section("network").Key("rpc_user").String()
	config.RPCPass = cfg.Section("network").Key("rpc_pass").String()
	config.Port = cfg.Section("network").Key("port").MustInt(8081)

	config.Xpub = cfg.Section("account").Key("xpub").String()
	config.Xpriv = cfg.Section("account").Key("xpriv").String()
	config.AccountId = cfg.Section("account").Key("id").MustInt(0)
	config.Index = uint32(cfg.Section("account").Key("index").MustInt(0))

	config.LastBlock = uint64(cfg.Section("extapi").Key("lastBlock").MustInt(0))
	config.FeeRate = int64(cfg.Section("extapi").Key("feerate").MustInt(0))
	config.RegistryAddr = cfg.Section("extapi").Key("registry").String()
	return config, nil
}

func SaveConfiguration(config *Config, filepath string) {
	cfg.Section("account").Key("index").SetValue(strconv.FormatInt(int64(config.Index), 10))
	cfg.Section("extapi").Key("lastBlock").SetValue(strconv.FormatUint(config.LastBlock, 10))
	cfg.SaveTo(filepath)
}
