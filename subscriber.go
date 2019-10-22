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
			for 0 != last.Cmp(stop) {
				log.Printf("Recovery: Doing block %s", last.Text(10))
				txns, err := ReadBlock(client, last)
				if err != nil {
					log.Println("Listener:", err)
					break
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
		symbol string
		addr   string
		amount string
		fee    string
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
		addr = message.Address
		symbol = message.Coin
		amount = LeftShift(amountString, 8)
		fee = LeftShift(message.Fee.String(), 8)

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

		switch message.TxType {
		case 0:
			log.Printf("%s %s tokens deposit to %s, tx: %s\n", symbol, amount, addr, message.TxHash)
			storeTokenDepositTx(symbol, message.TxHash, addr, amount)
		case 1:
			log.Printf("%s %s tokens withdraw to %s, tx: %s fee: %s\n", symbol, amount, addr, message.TxHash, fee)
			storeTokenWithdrawTx(symbol, message.TxHash, addr, amount, fee)
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
