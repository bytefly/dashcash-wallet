package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/wire"
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

		outputs := make([]TxOut, 1)
		outputs[0] = TxOut{Address: to, Amount: amount}
		tx, _ := CreateTxForOutputs(config.FeeRate, outputs, "", true)
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

		hash, err = SendTransaction(config, signedTx)
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("send tx err:%v", err))
			return
		}
		Respond(w, 0, map[string]string{"txhash": hash})
	}
}

func PrepareTrezorSignHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		defer m.Unlock()

		err := r.ParseForm()
		if err != nil {
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		to := r.Form.Get("to")
		amountStr := r.Form.Get("amount")

		if to != "" && !VerifyAddress(to) {
			log.Println("Invalid to address:", to)
			RespondWithError(w, 400, "Invalid to address")
			return
		}

		log.Println("preparing send coin to", to, "amount:", amountStr)

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

		if to == "" {
			log.Println("to is missing, use inner first address instead")
			to, err = GetNewChangeAddr(config, 0)
			if err != nil {
				RespondWithError(w, 500, fmt.Sprintf("get change addr err:%v", err))
				return
			}
		}

		changeAddress, err := GetNewChangeAddr(config, config.InIndex)
		if err != nil {
			log.Println("get change address err:", err)
			return
		}

		outputs := make([]TxOut, 1)
		outputs[0] = TxOut{Address: to, Amount: amount}
		tx, hasChange := CreateTxForOutputs(config.FeeRate, outputs, changeAddress, false)
		if tx == nil {
			RespondWithError(w, 500, "utxo out of balance")
			return
		}

		trezorTx, err := PrepareTrezorSign(config, tx)
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("prepare trezor sign err:%v", err))
			return
		}
		if hasChange {
			addrs.Store(changeAddress, fmt.Sprintf("1/%d", config.InIndex))
			config.InIndex++
		}
		Respond(w, 0, map[string]string{"trezorTx": trezorTx})
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

func GetInnerBalanceHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("get inner balance")

		balance, err := getInnerBalance()
		if err != nil {
			log.Println("get inner balance fail:", err)
			RespondWithError(w, 500, "get inner balance fail")
			return
		}

		Respond(w, 0, map[string]string{"balance": LeftShift(balance.String(), 8)})
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

func SendSignedTxHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	var tx wire.MsgTx
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		str := r.Form.Get("hex")
		if str == "" {
			log.Println("hex is empty")
			RespondWithError(w, 400, "Missing hex")
			return
		}

		buf, err := hex.DecodeString(str)
		if err != nil {
			log.Println("invalid hex string")
			RespondWithError(w, 400, "invalid hex string")
			return
		}
		err = tx.Deserialize(bytes.NewReader(buf))
		if err != nil {
			log.Println("invalid serialized transaction")
			RespondWithError(w, 400, "invalid serialized transaction")
			return
		}

		hash, err := SendTransaction(config, &tx)
		if err != nil {
			log.Println("send signed tx error: ", err)
			RespondWithError(w, 500, fmt.Sprintf("send signed tx err: %v", err))
			return
		}
		log.Println("send signed tx ok:", hash)

		Respond(w, 0, map[string]string{"hash": hash})
	}
}

func DumpUtxoHandler(config *Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		utxos, err := GetAllUtxo("")
		if err != nil {
			log.Println("get all utxo err:", err)
			RespondWithError(w, 500, "Error")
			return
		}

		for _, u := range utxos {
			log.Println(u.Hash, u.Index, " => ", u.Address, u.Value)
		}
		Respond(w, 0, "Done")
	}
}
