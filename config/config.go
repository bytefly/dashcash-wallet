package config

import (
	"gopkg.in/ini.v1"
	"strconv"
)

type Config struct {
	TestNet   int
	ChainName string
	ChainId   int32
	RPCURL    string
	RPCUser   string
	RPCPass   string
	Port      int

	Xpub      string
	Xpriv     string
	AccountId int
	Index     uint32
	InIndex   uint32

	LastBlock    uint64
	FeeRate      uint32
	RegistryAddr string
	ZmqURL       string
	DBDir        string

	DBHost string
	DBName string
	DBUser string
	DBPass string
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
	config.ChainName = cfg.Section("network").Key("chain").String()
	config.ChainId = int32(cfg.Section("network").Key("chain_id").MustInt(1))
	config.RPCURL = cfg.Section("network").Key("rpc_host").String()
	config.RPCUser = cfg.Section("network").Key("rpc_user").String()
	config.RPCPass = cfg.Section("network").Key("rpc_pass").String()
	config.Port = cfg.Section("network").Key("port").MustInt(8081)

	config.Xpub = cfg.Section("account").Key("xpub").String()
	config.Xpriv = cfg.Section("account").Key("xpriv").String()
	config.AccountId = cfg.Section("account").Key("id").MustInt(0)
	config.Index = uint32(cfg.Section("account").Key("index").MustInt(0))
	config.InIndex = uint32(cfg.Section("account").Key("change_index").MustInt(0))

	config.LastBlock = uint64(cfg.Section("extapi").Key("lastBlock").MustInt(0))
	config.FeeRate = uint32(cfg.Section("extapi").Key("feerate").MustInt(0))
	config.RegistryAddr = cfg.Section("extapi").Key("registry").String()
	config.ZmqURL = cfg.Section("extapi").Key("zmq").String()
	config.DBDir = cfg.Section("extapi").Key("dbDir").String()

	config.DBHost = cfg.Section("db").Key("host").String()
	config.DBName = cfg.Section("db").Key("name").String()
	config.DBUser = cfg.Section("db").Key("user").String()
	config.DBPass = cfg.Section("db").Key("pass").String()
	return config, nil
}

func SaveConfiguration(config *Config, filepath string) {
	cfg.Section("account").Key("index").SetValue(strconv.FormatInt(int64(config.Index), 10))
	cfg.Section("account").Key("change_index").SetValue(strconv.FormatInt(int64(config.InIndex), 10))
	cfg.Section("extapi").Key("lastBlock").SetValue(strconv.FormatUint(config.LastBlock-3, 10))
	cfg.SaveTo(filepath)
}
