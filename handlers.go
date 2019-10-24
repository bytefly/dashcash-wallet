package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

var m sync.Mutex

func Respond(w http.ResponseWriter, code int, payload interface{}) {
	ret := make(map[string]interface{})
	ret["Code"] = code
	if code >= 0 && code < 300 {
		ret["Msg"] = "success"
	} else {
		ret["Msg"] = "failure"
	}

	if payload != nil {
		ret["Data"] = payload
	}

	response, _ := json.Marshal(ret)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(response)
}

func RespondWithError(w http.ResponseWriter, code int, msg string) {
	Respond(w, code, map[string]string{"error": msg})
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("404: %s %s\n", r.Method, r.URL)
	RespondWithError(w, 404, "Not found")
}

func SendCoinHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		defer m.Unlock()

		err := r.ParseForm()
		if err != nil {
			log.Println("SendEthHandler: Could not parse body parameters")
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		to := r.Form.Get("to")
		amountStr := r.Form.Get("amount")

		if to == "" {
			log.Println("Got Send btc order but to field is missing")
			RespondWithError(w, 400, "Missing to field")
			return
		}

		if !VerifyAddress(to) {
			log.Println("Invalid to address:", to)
			RespondWithError(w, 400, "Invalid to address")
			return
		}

		log.Println("send coin to", to, "amount:", amountStr)

		if amountStr == "" {
			log.Println("amount is missing")
			RespondWithError(w, 400, "Missing amount field")
			return
		}

		amount, err := strconv.ParseInt(RightShift(amountStr, 8), 10, 64)
		if err != nil {
			RespondWithError(w, 400, "invalid amount")
			return
		}

		changeAddress, err := GetNewChangeAddr(config, config.InIndex)
		if err != nil {
			log.Println("get change address err:", err)
			return
		}

		outputs := make([]TxOut, 1)
		outputs[0] = TxOut{Address: to, Amount: amount}
		tx := CreateTxForOutputs(config.FeeRate, outputs, changeAddress)
		if tx == nil {
			RespondWithError(w, 500, "utxo out of balance")
			return
		}
		signedTx, err := SignMsgTx(config.Xpriv, tx)
		if err != nil {
			RespondWithError(w, 500, "tx cannot be signed")
			return
		}
		hash := signedTx.TxHash().String()

		DumpMsgTxInput(signedTx)

		hash, err = SendTransaction(config, signedTx)
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("send tx err:%v", err))
			return
		}
		addrs.Store(changeAddress, fmt.Sprintf("1/%d", config.InIndex))
		config.InIndex++
		Respond(w, 0, map[string]string{"txhash": hash})
	}
}

func GetAddrHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		defer m.Unlock()
		addr, err := GetNewExternalAddr(config, config.Index)
		if err != nil {
			log.Println("create address error: ", err)
			RespondWithError(w, 500, "Couldn't create eth address")
			return
		}
		log.Println("send addr:", addr)
		addrs.Store(addr, uint32(config.Index))
		config.Index++

		Respond(w, 0, addr)
	}
}

func GetBalanceHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		address := r.URL.Query().Get("address")

		log.Println("get balance of", address)
		if address != "" && !VerifyAddress(address) {
			log.Println("Invalid address:", address)
			RespondWithError(w, 400, "Invalid address")
			return
		}

		balance, err := getBalance(address)
		if err != nil {
			log.Println("get balance fail:", err)
			RespondWithError(w, 500, "get balance fail")
			return
		}

		Respond(w, 0, map[string]string{"balance": LeftShift(balance.String(), 8)})
	}
}
