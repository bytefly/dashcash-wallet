package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	conf "github.com/bytefly/dashcash-wallet/config"
	"github.com/bytefly/dashcash-wallet/usdt"
	"github.com/bytefly/dashcash-wallet/util"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
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

func SendCoinHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	param := util.GetParamByName(config.ChainName)
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

		if !util.VerifyAddress(config.ChainName, to) {
			log.Println("Invalid to address:", to)
			RespondWithError(w, 400, "Invalid to address")
			return
		}
		if strings.HasPrefix(strings.ToLower(config.ChainName), "bch") {
			to, _ = util.ConvertCashAddrToLegacy(to, param)
		}

		log.Println("send coin to", to, "amount:", amountStr)

		if amountStr == "" {
			log.Println("amount is missing")
			RespondWithError(w, 400, "Missing amount field")
			return
		}

		amount, err := strconv.ParseInt(util.RightShift(amountStr, 8), 10, 64)
		if err != nil {
			RespondWithError(w, 400, "invalid amount")
			return
		}

		outputs := make([]TxOut, 1)
		outputs[0] = TxOut{Address: to, Amount: amount}
		tx, _ := CreateTxForOutputs(config.FeeRate, "", outputs, "", param, true, false)
		if tx == nil {
			RespondWithError(w, 500, "utxo out of balance")
			return
		}

		signedTx, err := SignMsgTx(config.ChainName, config.Xpriv, tx)
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
		log.Println("new generated tx:", hash)
		Respond(w, 0, map[string]string{"txhash": hash})
	}
}

func PrepareTrezorSignHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	param := util.GetParamByName(config.ChainName)
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

		if to != "" && !util.VerifyAddress(config.ChainName, to) {
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

		amount, err := strconv.ParseInt(util.RightShift(amountStr, 8), 10, 64)
		if err != nil {
			RespondWithError(w, 400, "invalid amount")
			return
		}

		if to == "" {
			log.Println("to is missing, use inner first address instead")
			to, err = util.GetNewChangeAddr(config, 0)
			if err != nil {
				RespondWithError(w, 500, fmt.Sprintf("get change addr err:%v", err))
				return
			}
		}

		changeAddress, err := util.GetNewChangeAddr(config, config.InIndex)
		if err != nil {
			log.Println("get change address err:", err)
			return
		}

		if strings.HasPrefix(strings.ToLower(config.ChainName), "bch") {
			to, _ = util.ConvertCashAddrToLegacy(to, param)
			changeAddress, _ = util.ConvertCashAddrToLegacy(changeAddress, param)
		}
		outputs := make([]TxOut, 1)
		outputs[0] = TxOut{Address: to, Amount: amount}
		tx, hasChange := CreateTxForOutputs(config.FeeRate, "", outputs, changeAddress, param, false, false)
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
			util.StoreAddrPath(changeAddress, fmt.Sprintf("1/%d", config.InIndex))
			config.InIndex++
		}
		Respond(w, 0, map[string]string{"trezorTx": trezorTx})
	}
}

func GetAddrHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		defer m.Unlock()
		addr, err := util.GetNewExternalAddr(config, config.Index)
		if err != nil {
			log.Println("create address error: ", err)
			RespondWithError(w, 500, "Couldn't create eth address")
			return
		}
		param := util.GetParamByName(config.ChainName)
		if strings.HasPrefix(strings.ToLower(config.ChainName), "bch") {
			addr, _ = util.ConvertLegacyToCashAddr(addr, param)
			addr = addr[len(param.Bech32HRPSegwit)+1:]
		}
		log.Println("send addr:", addr, config.Index)
		util.StoreAddrPath(addr, fmt.Sprintf("0/%d", config.Index))
		config.Index++

		Respond(w, 0, addr)
	}
}

func GetInnerBalanceHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("get inner balance")

		balance, err := getInnerBalance(false)
		if err != nil {
			log.Println("get inner balance fail:", err)
			RespondWithError(w, 500, "get inner balance fail")
			return
		}

		Respond(w, 0, map[string]string{"balance": util.LeftShift(balance.String(), 8)})
	}
}

func GetBalanceHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		address := r.URL.Query().Get("address")

		log.Println("get balance of", address)
		if address != "" && !util.VerifyAddress(config.ChainName, address) {
			log.Println("Invalid address:", address)
			RespondWithError(w, 400, "Invalid address")
			return
		}

		balance, err := getBalance(address, false)
		if err != nil {
			log.Println("get balance fail:", err)
			RespondWithError(w, 500, "get balance fail")
			return
		}

		Respond(w, 0, map[string]string{"balance": util.LeftShift(balance.String(), 8)})
	}
}

func SendSignedTxHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
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

func DumpUtxoHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		utxos, err := GetAllUtxo("", true)
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

func SendOmniCoinHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	param := util.GetParamByName(config.ChainName)
	return func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		defer m.Unlock()

		if !strings.EqualFold(config.ChainName, "btc") {
			RespondWithError(w, 400, "omni coin not supported in the chain")
			return
		}

		err := r.ParseForm()
		if err != nil {
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		token := r.Form.Get("token")
		from := r.Form.Get("from")
		to := r.Form.Get("to")
		amountStr := r.Form.Get("amount")

		if token != "USDT" {
			log.Println("only USDT support now")
			RespondWithError(w, 400, "invalid token")
			return
		}
		// use the first inner address as sender in default
		if from == "" {
			from, _ = util.GetNewChangeAddr(config, 0)
		}
		if to == "" {
			log.Println("Got Send USDT order but to field is missing")
			RespondWithError(w, 400, "Missing to field")
			return
		}

		if !util.VerifyAddress(config.ChainName, to) {
			log.Println("Invalid to address:", to)
			RespondWithError(w, 400, "Invalid to address")
			return
		}
		if util.IsNativeSegWitAddress(config.ChainName, to) {
			log.Println("native segwit address not supported:", to)
			RespondWithError(w, 400, "cannot be segwit address")
			return
		}

		log.Println("send", token, "to", to, "amount:", amountStr)

		if amountStr == "" {
			log.Println("amount is missing")
			RespondWithError(w, 400, "Missing amount field")
			return
		}

		amount, err := strconv.ParseInt(util.RightShift(amountStr, 8), 10, 64)
		if err != nil {
			RespondWithError(w, 400, "invalid amount")
			return
		}

		//check balance
		pendingAmount, err := usdt.GetOMNIPendingAmount(config, from, usdt.OMNIToken)
		if err != nil {
			RespondWithError(w, 400, "get pending transactions error")
			return
		}
		balance, err := usdt.GetUSDTBalance(config, from)
		if err != nil {
			RespondWithError(w, 400, "get usdt balance error")
			return
		}
		if balance < amount {
			log.Printf("no enough balance, balance: %d, amount: %d\n", balance, amount)
			RespondWithError(w, 400, "No enough balance for transfer")
			return
		}
		if balance < (pendingAmount + amount) {
			log.Printf("no enough balance, balance: %d, amount: %d, pending_amount: %d\n", balance, amount, pendingAmount)
			RespondWithError(w, 400, "So many pending transactions that balance is less")
			return
		}

		outputs := make([]TxOut, 2)
		outputs[0] = TxOut{Script: usdt.GetOmniUsdtScript(uint64(amount))}
		outputs[1] = TxOut{Address: to, Amount: 546}
		tx, _ := CreateTxForOutputs(config.FeeRate, from, outputs, "", param, true, true)
		if tx == nil {
			RespondWithError(w, 500, "utxo out of balance")
			return
		}

		signedTx, err := SignMsgTx(config.ChainName, config.Xpriv, tx)
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

func PrepareOmniTrezorSignHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	param := util.GetParamByName(config.ChainName)
	return func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		defer m.Unlock()

		if !strings.EqualFold(config.ChainName, "btc") {
			RespondWithError(w, 400, "omni coin not supported in the chain")
			return
		}

		err := r.ParseForm()
		if err != nil {
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		token := r.Form.Get("token")
		from := r.Form.Get("from")
		to := r.Form.Get("to")
		amountStr := r.Form.Get("amount")

		if token != "USDT" {
			log.Println("only USDT support now")
			RespondWithError(w, 400, "invalid token")
			return
		}
		// use the first inner address as sender in default
		if from == "" {
			log.Println("missing from")
			RespondWithError(w, 400, "missing from")
			return
		} else if !util.VerifyAddress(config.ChainName, from) {
			log.Println("Invalid from address:", from)
			RespondWithError(w, 400, "Invalid from address")
			return
		}
		if to != "" {
			if !util.VerifyAddress(config.ChainName, to) {
				log.Println("Invalid to address:", to)
				RespondWithError(w, 400, "Invalid to address")
				return
			}
			if util.IsNativeSegWitAddress(config.ChainName, to) {
				log.Println("native segwit address not supported:", to)
				RespondWithError(w, 400, "cannot be segwit address")
				return
			}
		}

		log.Println("preparing send", token, "from", from, "to", to, "amount:", amountStr)

		if amountStr == "" {
			log.Println("amount is missing")
			RespondWithError(w, 400, "Missing amount field")
			return
		}

		amount, err := strconv.ParseInt(util.RightShift(amountStr, 8), 10, 64)
		if err != nil {
			RespondWithError(w, 400, "invalid amount")
			return
		}

		if to == "" {
			log.Println("to is missing, use inner first address instead")
			to, err = util.GetNewChangeAddr(config, 0)
			if err != nil {
				RespondWithError(w, 500, fmt.Sprintf("get change addr err:%v", err))
				return
			}
		}

		changeAddress, err := util.GetNewChangeAddr(config, config.InIndex)
		if err != nil {
			log.Println("get change address err:", err)
			return
		}

		outputs := make([]TxOut, 2)
		outputs[0] = TxOut{Script: usdt.GetOmniUsdtScript(uint64(amount))}
		outputs[1] = TxOut{Address: to, Amount: 546}
		tx, hasChange := CreateTxForOutputs(config.FeeRate, from, outputs, changeAddress, param, false, true)
		if tx == nil {
			RespondWithError(w, 500, "utxo out of balance")
			return
		}

		tmp := *tx.TxOut[1]
		tx.TxOut[1] = tx.TxOut[2]
		tx.TxOut[2] = &tmp

		trezorTx, err := PrepareTrezorSign(config, tx)
		if err != nil {
			RespondWithError(w, 500, fmt.Sprintf("prepare trezor sign err:%v", err))
			return
		}
		if hasChange {
			util.StoreAddrPath(changeAddress, fmt.Sprintf("1/%d", config.InIndex))
			config.InIndex++
		}
		Respond(w, 0, map[string]string{"trezorTx": trezorTx})
	}
}

func GetOmniBalanceHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(config.ChainName, "btc") {
			RespondWithError(w, 400, "omni coin not supported in the chain")
			return
		}

		address := r.URL.Query().Get("address")
		token := r.URL.Query().Get("token")

		log.Println("get omni balance of", address, token)
		if address == "" {
			RespondWithError(w, 400, "missing address")
			return
		}
		if !util.VerifyAddress(config.ChainName, address) {
			log.Println("Invalid address:", address)
			RespondWithError(w, 400, "Invalid address")
			return
		}

		if token == "" {
			token = "USDT"
		}
		pendingAmount, err := usdt.GetOMNIPendingAmount(config, address, usdt.OMNIToken)
		if err != nil {
			RespondWithError(w, 400, "get pending transactions error")
			return
		}
		balance, err := usdt.GetUSDTBalance(config, address)
		if err != nil {
			RespondWithError(w, 400, "get usdt balance error")
			return
		}

		Respond(w, 0, map[string]string{"balance": util.LeftShift(big.NewInt(balance-pendingAmount).String(), 8)})
	}
}

func CheckAddrHandler(config *conf.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := r.URL.Query().Get("address")
		if addr == "" {
			RespondWithError(w, 400, "missing address")
			return
		}

		ok := util.VerifyAddress(config.ChainName, addr)
		if ok {
			Respond(w, 0, map[string]string{"result": "valid"})
		} else {
			Respond(w, 0, map[string]string{"result": "invalid"})
		}
	}
}
