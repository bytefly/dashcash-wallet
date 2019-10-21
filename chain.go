package main

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/ibclabs/omnilayer-go"
	"log"
	"math/big"
)

func ConnectRPC(config *Config) (*rpcclient.Client, error) {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         config.RPCURL,
		User:         config.RPCUser,
		Pass:         config.RPCPass,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("Couldn't connect to RPC server: %v", err)
	}

	return client, nil
}

func GetOmniTxStatus(c *Config, hash string) (bool, error) {
	var config = &omnilayer.ConnConfig{
		Host:                 c.RPCURL,
		User:                 c.RPCUser,
		Pass:                 c.RPCPass,
		DisableAutoReconnect: false,
		DisableConnectOnNew:  false,
		EnableBCInfoHacks:    true,
	}
	client := omnilayer.New(config)

	txResult, err := client.OmniGetTransaction(hash)
	if err != nil {
		log.Println("get tx info for ", hash, " err: ", err)
		return false, err
	}

	return txResult.Valid, nil
}

func ParseTransaction(client *rpcclient.Client, msgtx *wire.MsgTx, isPending bool) (messages []NotifyMessage, err error) {
	messages = make([]NotifyMessage, 0)

	if msgtx == nil {
		return messages, fmt.Errorf("Transaction is nil: Can't parse.")
	}

	hash := msgtx.TxHash().String()
	var fee uint64
	for i := 0; i < len(msgtx.TxIn); i++ {
		tx, err := client.GetRawTransaction(&(msgtx.TxIn[i].PreviousOutPoint.Hash))
		if err != nil {
			log.Println("get transaction err:", err)
			return messages, err
		}
		fee += uint64(tx.MsgTx().TxOut[msgtx.TxIn[i].PreviousOutPoint.Index].Value)
	}
	for i := 0; i < len(msgtx.TxOut); i++ {
		fee -= uint64(msgtx.TxOut[i].Value)
	}
	//TODO: analyze every transaction
	message := NotifyMessage{
		MessageType: NOTIFY_TYPE_TX,
		AddressFrom: "1fffff",
		AddressTo:   "1abcde32938",
		Amount:      big.NewInt(1000000),
		TxHash:      hash,
		Fee:         big.NewInt(int64(fee)),
		Coin:        "BTC",
		TxType:      0,
	}

	messages = append(messages, message)
	return
}

func ReadBlock(client *rpcclient.Client, hashStr string) ([]NotifyMessage, error) {
	var err error
	messages := make([]NotifyMessage, 0)

	hash, _ := chainhash.NewHashFromStr(hashStr)

	blockInfo, err := client.GetBlock(hash)
	if err != nil {
		return messages, fmt.Errorf("ReadBlock failed: %v", err)
	}

	for _, tx := range blockInfo.Transactions {
		message, err := ParseTransaction(client, tx, false)
		if err != nil {
			return messages, err
		}

		messages = append(messages, message...)
	}

	return messages, nil
}
