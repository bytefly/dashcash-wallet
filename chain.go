package main

import (
	"fmt"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
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
	inputAddrs := make([]string, 0)
	inputAddrs2 := make([]string, 0)
	outputAddrs := make([]string, 0)
	outputAddrs2 := make([]string, 0)
	outputValue := make(map[string]int64)

	//TODO: omni layer not contained
	for i := 0; i < len(msgtx.TxIn); i++ {
		prevHash := msgtx.TxIn[i].PreviousOutPoint.Hash
		prevIndex := msgtx.TxIn[i].PreviousOutPoint.Index
		tx, err := client.GetRawTransaction(&prevHash)
		if err != nil {
			log.Println("get transaction err:", err)
			return messages, err
		}
		value := tx.MsgTx().TxOut[prevIndex].Value
		fee += uint64(value)

		script := tx.MsgTx().TxOut[prevIndex].PkScript
		pkScript, err := txscript.ParsePkScript(script)
		if err != nil {
			log.Println("parse pkscript err:", err)
			continue
		}

		addr, err := pkScript.Address(&DSCMainNetParams)
		if err != nil {
			log.Println("get addr err:", err)
			continue
		}
		addrStr := addr.EncodeAddress()
		_, ok := addrs.Load(addrStr)
		if ok {
			removeUtxo(prevHash.String(), prevIndex, addrStr, value)
			inputAddrs = append(inputAddrs, addrStr)
			log.Println("input:", addrStr)
		} else {
			inputAddrs2 = append(inputAddrs2, addrStr)
		}
	}

	for i := 0; i < len(msgtx.TxOut); i++ {
		fee -= uint64(msgtx.TxOut[i].Value)

		pkScript, err := txscript.ParsePkScript(msgtx.TxOut[i].PkScript)
		if err != nil {
			log.Println("parse pkscript err:", err)
			continue
		}

		addr, err := pkScript.Address(&DSCMainNetParams)
		if err != nil {
			log.Println("get addr err:", err)
			continue
		}
		addrStr := addr.EncodeAddress()
		_, ok := addrs.Load(addrStr)
		if ok {
			createUtxo(hash, uint32(i), addrStr, msgtx.TxOut[i].Value)
			outputAddrs = append(outputAddrs, addrStr)
			log.Println("output:", addrStr)
		} else {
			outputAddrs2 = append(outputAddrs2, addrStr)
		}

		outputValue[addrStr] = msgtx.TxOut[i].Value
	}

	message := NotifyMessage{
		MessageType: NOTIFY_TYPE_TX,
		TxHash:      hash,
		Fee:         big.NewInt(int64(fee)),
		Coin:        "DSC",
	}
	if len(inputAddrs) == 0 && len(outputAddrs) > 0 {
		log.Println("deposit tx found")
		message.TxType = 0
		for i := 0; i < len(outputAddrs); i++ {
			message.Address = outputAddrs[i]
			message.Amount = big.NewInt(outputValue[message.Address])
			messages = append(messages, message)
		}
	} else if len(inputAddrs2) == 0 && len(outputAddrs2) > 0 {
		log.Println("withdraw tx found")
		message.TxType = 1
		for i := 0; i < len(outputAddrs2); i++ {
			message.Address = outputAddrs2[i]
			message.Amount = big.NewInt(outputValue[message.Address])
			messages = append(messages, message)
		}
	} else if len(inputAddrs2) == 0 && len(outputAddrs2) == 0 {
		log.Println("inner tx found:", hash)
	}

	return
}

func ReadBlock(client *rpcclient.Client, block *big.Int) ([]NotifyMessage, error) {
	var err error
	messages := make([]NotifyMessage, 0)

	hash, err := client.GetBlockHash(block.Int64())
	if err != nil {
		return messages, fmt.Errorf("read block hash err: %v", err)
	}

	blockInfo, err := client.GetBlock(hash)
	if err != nil {
		return messages, fmt.Errorf("get block err: %v", err)
	}

	for i, tx := range blockInfo.Transactions {
		//ignore coin base
		if i == 0 {
			continue
		}
		message, err := ParseTransaction(client, tx, false)
		if err != nil {
			return messages, err
		}

		messages = append(messages, message...)
	}

	return messages, nil
}

func SendTransaction(config *Config, tx *wire.MsgTx) (string, error) {
	client, err := ConnectRPC(config)
	if err != nil {
		panic(err)
	}
	defer client.Shutdown()

	hexHash, err := client.SendRawTransaction(tx, false)
	if err != nil {
		return "Send raw transaction error", err
	}

	hash := hexHash.String()
	return hash, nil
}
