package main

import (
	conf "github.com/bytefly/dashcash-wallet/config"
	"github.com/bytefly/dashcash-wallet/util"
	"log"
	"math/big"
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
	Result       interface{} `json:"result"`
}

type ResponseMessage struct {
	JsonRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  Params `json:"params"`
}

const (
	TYPE_BLOCK_HASH = iota
	MIN_BTC_AMOUNT  = 100000

	TYPE_NONE = iota
	TYPE_USER_DEPOSIT
	TYPE_ADMIN_DEPOSIT
	TYPE_USER_WITHDRAW
	TYPE_ADMIN_WITHDRAW
	TYPE_FUND_COLLECTION
)

type ObjMessage struct {
	Type   int
	Hash   string
	Number *big.Int
}

var blkPool = make(map[uint64][]NotifyMessage)

func GetNewerBlock(config *conf.Config, ch chan<- ObjMessage) error {
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

func Listener(config *conf.Config, ch <-chan ObjMessage, notifyChannel chan<- NotifyMessage, last_id uint64) {
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
			for last.Cmp(message.Number) <= 0 {
				//log.Printf("Recovery: Doing block %s", last.Text(10))
				txns, err := ReadBlock(client, last, config.ChainName)
				if err != nil {
					log.Println("Listener:", err)
					break
				}

				if last.Cmp(stop) < 0 {
					for _, txn := range txns {
						notifyChannel <- txn
					}

					notifyChannel <- NotifyMessage{
						MessageType: NOTIFY_TYPE_ADMIN,
						Amount:      last,
					}
				} else {
					for _, txn := range txns {
						if txn.MessageType == NOTIFY_TYPE_TX {
							if txn.Amount.String() != "0" && txn.Coin == "BTC" {
								if txn.Amount.Uint64() < MIN_BTC_AMOUNT {
									continue
								}
							}
							//remove non wallet txn
							log.Println("new tx is confirmed:", txn.TxHash)
						}
					}

					//put it in map
					if len(txns) > 0 {
						blkPool[last.Uint64()] = txns
						log.Println("add txs to", last.Uint64(), "txs size:", len(txns))
					}
					//scan and broadcast 3 confirms
					txns, ok := blkPool[last.Uint64()-3]
					if ok {
						for _, txn := range txns {
							notifyChannel <- txn
						}

						notifyChannel <- NotifyMessage{
							MessageType: NOTIFY_TYPE_ADMIN,
							Amount:      new(big.Int).Sub(last, big.NewInt(3)),
						}

						// delete unconfirmed height txs map
						delete(blkPool, last.Uint64()-3)
						log.Println("delete txs in block", last.Uint64()-3)
					}
				}

				last.SetUint64(last.Uint64() + 1)
				config.LastBlock = last.Uint64()
			}

			// We set last_id a 0. We don't want this process to restart.
			//log.Printf("Recovery is over: Done up to block %s", last.Text(10))
			last_id = last.Uint64()
		}
	}
}

func Notifier(config *conf.Config, ch <-chan NotifyMessage) {
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
			continue
		}

		amountString := message.Amount.Text(10)
		addr = message.Address
		symbol = message.Coin
		amount = util.LeftShift(amountString, 8)
		fee = util.LeftShift(message.Fee.String(), 8)

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
		case TYPE_USER_DEPOSIT:
			//small btc deposit less then 0.001 is ignored
			if symbol == "BTC" && message.Amount.Uint64() < MIN_BTC_AMOUNT {
				break
			}
			log.Printf("%s %s tokens deposit to %s, tx: %s\n", symbol, amount, addr, message.TxHash)
			storeTokenDepositTx(config, symbol, message.TxHash, addr, amount)
		case TYPE_USER_WITHDRAW:
			log.Printf("%s %s tokens withdraw to %s, tx: %s fee: %s\n", symbol, amount, addr, message.TxHash, fee)
			storeTokenWithdrawTx(config, symbol, message.TxHash, addr, amount, fee)
		}
		EnterFundflowDB(config, message, symbol, fee)
	}
}
