package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
)

var m sync.Mutex

func Respond(w http.ResponseWriter, code int, payload interface{}) {
	ret := make(map[string]interface{})
	ret["Code"] = code
	if code >= 200 && code < 300 {
		ret["Msg"] = "success"
	} else {
		ret["Msg"] = "failure"
	}

	if payload != nil {
		ret["Data"] = payload
	}

	response, _ := json.Marshal(ret)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
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
	isWithdraw := false
	branch := 0
	var index int
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			log.Println("SendEthHandler: Could not parse body parameters")
			RespondWithError(w, 400, "Could not parse parameters")
			return
		}

		from := r.Form.Get("from")
		to := r.Form.Get("to")
		amount := r.Form.Get("amount")

		if from == "" {
			from, err = getNewAddrByBranch(config, 1, 0)
			if err != nil {
				log.Println("create from address error: ", err)
				RespondWithError(w, 500, "Couldn't get from address")
				return
			}
			isWithdraw = true
			branch = 1
			index = 0
		}

		if to == "" {
			log.Println("Got Send btc order but to field is missing")
			RespondWithError(w, 400, "Missing to field")
			return
		} else {
			_, ok := addrs.Load(to)
			if ok {
				log.Println("to address is in our wallet")
				RespondWithError(w, 500, "Couldn't launch transfering within the same wallet")
				return
			}
		}

		if !VerifyAddress(from) || !VerifyAddress(to) {
			log.Println("Invalid from/to address:", from, to)
			RespondWithError(w, 400, "Invalid from/to address")
			return
		}

		if !isWithdraw {
			v, ok := addrs.Load(from)
			if !ok {
				log.Println("from address is not in our wallet")
				RespondWithError(w, 400, "Invalid from address")
				return
			}
			index = int(v.(uint32))
		}
		log.Println("send eth from", from, "to", to)

		private, err := GetPrivateKey(config.Xpriv, branch, index)
		if err != nil {
			log.Println("get private key fail")
			RespondWithError(w, 500, "get pirvate key fail")
			return
		}

		if amount == "" {
			log.Println("Got Send Ethereum order but 'amount' field is missing")
			RespondWithError(w, 400, "Missing 'amount' field")
			return
		}

		_, err = strconv.ParseFloat(amount, 64)
		if err != nil {
			RespondWithError(w, 400, "Could not convert amount")
			return
		}
		/*//FIXME
		bgAmountInt := new(big.Int)
		bgAmountInt.SetString(RightShift(amount, 18), 10)
		tx, err := SendEthCoin(config, bgAmountInt, private, to, nil, nil)
		if err != nil {
			log.Println("send eth err:", err)
			RespondWithError(w, 500, fmt.Sprintf("Could not send Ethereum coin: %v", err))
			return
		}
		*/
		log.Println(private)
		tx := "demo"
		Respond(w, 200, map[string]string{"txhash": tx})
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
		//Respond(w, 200, map[string]string{"address": addr})
		Respond(w, 200, addr)
	}
}
