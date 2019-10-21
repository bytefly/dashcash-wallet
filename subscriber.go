package main

import (
	"log"
	"math/big"
	"time"
)

type WsEvent struct {
	Event string `json:"event"`
	Token string `json:"token"`
}

type SubscriptionMessage struct {
	JsonRPC string      `json:"jsonrpc"`
	Id      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type SubscriptionResponse struct {
	JsonRPC string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  string `json:"result"`
}

type BlockHeader struct {
	ParentHash string `json:"parentHash"`
	Difficulty string `json:"difficulty"`
	Number     string `json:"number"`
	GasLimit   string `json:"gasLimit"`
	GasUsed    string `json:"gasUsed"`
	Timestamp  string `json:"timestamp"`
	Hash       string `json:"hash"`
}

type Params struct {
	Subscription string      `json:"subscription"`
	Result       interface{} `json:"result`
}

type ResponseMessage struct {
	JsonRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  Params `json:"params"`
}

const (
	TYPE_BLOCK_HASH = iota
)

type ObjMessage struct {
	Type   int
	Hash   string
	Number *big.Int
}

func GetNewerBlock(config *Config, ch chan<- ObjMessage) error {
	client, err := ConnectRPC(config)
	if err != nil {
		return err
	}
	defer client.Shutdown()

	info, err := client.GetBlockChainInfo()
	if err != nil {
		log.Println("get best block hash err:", err)
		return err
	}

	bgInt := new(big.Int)
	bgInt.SetInt64(int64(info.Headers))
	ch <- ObjMessage{TYPE_BLOCK_HASH, info.BestBlockHash, bgInt}
	return nil
}

func Listener(config *Config, ch <-chan ObjMessage, notifyChannel chan<- NotifyMessage, last_id uint64) {
	client, err := ConnectRPC(config)
	if err != nil {
		panic(err)
	}
	defer client.Shutdown()

	for message := range ch {
		switch message.Type {
		case TYPE_BLOCK_HASH:
			last := new(big.Int)
			last.SetUint64(last_id)

			stop := new(big.Int)
			stop.Sub(message.Number, big.NewInt(3))
			log.Println("handling from ", last, "~", stop, message.Number)
			for 0 != last.Cmp(stop) {
				log.Printf("Recovery: Doing block %s", last.Text(10))
				txns, err := ReadBlock(client, message.Hash)
				if err != nil {
					log.Println("Listener:", err)
					continue
				}

				for _, txn := range txns {
					notifyChannel <- txn
				}

				notifyChannel <- NotifyMessage{
					MessageType: NOTIFY_TYPE_ADMIN,
					Amount:      last,
				}

				last.SetUint64(last.Uint64() + 1)
			}

			// We set last_id a 0. We don't want this process to restart.
			//log.Printf("Recovery is over: Done up to block %s", last.Text(10))
			last_id = last.Uint64()
		}
	}
}

func Notifier(config *Config, ch <-chan NotifyMessage) {
	var (
		symbol   string
		from     string
		to       string
		findFrom int
		findTo   int
		amount   string
		fee      string
	)

	for message := range ch {
		if message.MessageType == NOTIFY_TYPE_NONE {
			continue
		}

		if message.MessageType == NOTIFY_TYPE_ADMIN {
			config.LastBlock = message.Amount.Uint64()
			continue
		}

		amountString := message.Amount.Text(10)
		from = message.AddressFrom
		to = message.AddressTo
		symbol = message.Coin
		amount = LeftShift(amountString, 8)
		fee = LeftShift(message.Fee.String(), 8)

		//log.Printf("%s from: %s, to: %s, amount: %s, hash:%s\n", symbol, from, to, amount, message.TxHash)
		_, ok := addrs[from]
		if ok {
			findFrom = 1
		} else {
			findFrom = 0
		}
		_, ok = addrs[to]
		if ok {
			findTo = 1
		} else {
			findTo = 0
		}

		if findTo == 0 && findFrom == 0 {
			continue
		}
		if findTo == 1 && findFrom == 1 {
			log.Printf("token transfer within the same wallet (%s: %s -> %s)\n", symbol, from, to)
			continue
		}

		if symbol == "USDT" {
			status, err := GetOmniTxStatus(config, message.TxHash)
			if err != nil {
				log.Println("get tx status err:", err, ", tx:", message.TxHash)
				continue
			}
			if status == false {
				log.Println("usdt tx status is fail, tx:", message.TxHash)
				continue
			}
		}

		if findTo == 1 && findFrom == 0 {
			log.Printf("%s %s tokens deposit to the wallet, %s -> %s, tx: %s\n", symbol, amount, from, to, message.TxHash)
			storeTokenDepositTx(symbol, message.TxHash, to, amount)
		} else if findTo == 0 && findFrom == 1 {
			log.Printf("%s %s tokens withdraw from the wallet, %s -> %s, tx: %s fee: %s\n", symbol, amount, from, to, message.TxHash, fee)
			storeTokenWithdrawTx(symbol, message.TxHash, to, amount, fee)
		}
	}
}

func Subscriber(config *Config, notifyChannel chan<- NotifyMessage, last_id uint64) {
	ch := make(chan ObjMessage, 1024)

	go Listener(config, ch, notifyChannel, last_id)

	for {
		ts_startup := time.Now()
		err := GetNewerBlock(config, ch)
		if err != nil {
			log.Println(err)
		}

		elapsed := time.Now().Sub(ts_startup)

		if elapsed < time.Second*5 {
			// Wait a few seconds before retrying
			time.Sleep(time.Second * 5)
		}
	}
}
