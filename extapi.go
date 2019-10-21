package main

import (
	"github.com/TarsCloud/TarsGo/tars"
	"github.com/bytefly/dashcash-wallet/NeexTrx"
	"log"
)

const (
	CHAIN_ID = 1
)

func storeTokenDepositTx(token string, hash string, addr string, amount string) {
	comm := tars.NewCommunicator()
	obj := "NeexTrx.FreezingSysServer.FreezingSysObj"
	registry := cfg.Section("extapi").Key("registry").String()
	comm.SetProperty("locator", "tars.tarsregistry.QueryObj@tcp -h "+registry+" -p 17890")
	app := new(NeexTrx.FreezingSys)

	comm.StringToProxy(obj, app)

	ret, err := app.User_into_dc2(addr, token, hash, amount, CHAIN_ID)
	if err != nil {
		log.Println("call freezing deposit err:", err)
		return
	}
	log.Println("call freezing deposit result:", ret)
}

func storeTokenWithdrawTx(token string, hash string, addr string, amount string, fee string) {
	comm := tars.NewCommunicator()
	obj := "NeexTrx.FreezingSysServer.FreezingSysObj"
	registry := cfg.Section("extapi").Key("registry").String()
	comm.SetProperty("locator", "tars.tarsregistry.QueryObj@tcp -h "+registry+" -p 17890")
	app := new(NeexTrx.FreezingSys)

	comm.StringToProxy(obj, app)

	var rsp string
	ret, err := app.Commit_withdraw_dc(hash, token, amount, fee, &rsp)
	if err != nil {
		log.Println("call freezing withdraw err:", err)
		return
	}
	log.Println("call freezing withdraw result:", ret, ", rsp:", rsp, ", hash:", hash)
}
