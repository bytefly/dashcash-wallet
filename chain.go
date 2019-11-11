package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	conf "github.com/bytefly/dashcash-wallet/config"
	"github.com/bytefly/dashcash-wallet/util"
	"github.com/ibclabs/omnilayer-go"
	"log"
	"math/big"
	"strings"
)

func ConnectRPC(config *conf.Config) (*rpcclient.Client, error) {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:                 config.RPCURL,
		User:                 config.RPCUser,
		Pass:                 config.RPCPass,
		HTTPPostMode:         true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:           true, // Bitcoin core does not provide TLS by default
		DisableAutoReconnect: true,
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

func GetOmniTxStatus(c *conf.Config, hash string) (bool, error) {
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

func ParseTransaction(client *rpcclient.Client, msgtx *wire.MsgTx, chainName string) (messages []NotifyMessage, err error) {
	var (
		fee              uint64
		opReturnNum      int
		firstOpReturnPos int
		omniReceiver     string
		omniSender       string
		foundSender      bool
	)
	messages = make([]NotifyMessage, 0)
	param := util.GetParamByName(chainName)

	if msgtx == nil {
		return messages, fmt.Errorf("Transaction is nil: Can't parse.")
	}

	hash := msgtx.TxHash().String()
	inputAddrs := make([]string, 0)
	inputAddrs2 := make([]string, 0)
	outputAddrs := make([]string, 0)
	outputAddrs2 := make([]string, 0)
	outputValue := make(map[string]int64)

	for i := 0; i < len(msgtx.TxIn); i++ {
		prevHash := msgtx.TxIn[i].PreviousOutPoint.Hash
		prevIndex := msgtx.TxIn[i].PreviousOutPoint.Index
		//ignore coinbase input
		if prevIndex == 0xFFFFFFFF {
			continue
		}
		tx, err := client.GetRawTransaction(&prevHash)
		if err != nil {
			log.Println("get transaction err:", err, prevHash.String())
			continue
		}
		value := tx.MsgTx().TxOut[prevIndex].Value
		fee += uint64(value)

		script := tx.MsgTx().TxOut[prevIndex].PkScript
		_, addrSet, _, err := txscript.ExtractPkScriptAddrs(
			script, param)
		if err != nil {
			log.Println("parse input pkscript err:", err, hash, i)
			continue
		}

		addrStr := addrSet[0].EncodeAddress()
		_, ok := util.LoadAddrPath(addrStr)
		if ok {
			inputAddrs = append(inputAddrs, addrStr)
			removeUtxo(prevHash.String(), prevIndex, addrStr, value)
			log.Println("remove utxo:", prevHash.String(), prevIndex, addrStr)
		} else {
			inputAddrs2 = append(inputAddrs2, addrStr)
		}
		if i == 0 {
			omniSender = addrStr
		}
	}

	for i := len(msgtx.TxOut) - 1; i >= 0; i-- {
		fee -= uint64(msgtx.TxOut[i].Value)

		if msgtx.TxOut[i].Value == 0 && txscript.GetScriptClass(msgtx.TxOut[i].PkScript) == txscript.NullDataTy {
			opReturnNum++
			if opReturnNum == 1 {
				firstOpReturnPos = i
			}
			continue
		}

		_, addrSet, _, err := txscript.ExtractPkScriptAddrs(
			msgtx.TxOut[i].PkScript, param)
		if err != nil {
			log.Println("parse output pkscript err:", err, hash, i)
			continue
		}

		if len(addrSet) == 0 {
			log.Println("cannot get address", hash, i)
			continue
		}

		addrStr := addrSet[0].EncodeAddress()
		_, ok := util.LoadAddrPath(addrStr)
		if ok {
			outputAddrs = append(outputAddrs, addrStr)
			createUtxo(hash, uint32(i), addrStr, msgtx.TxOut[i].Value)
			log.Println("add utxo:", hash, i, addrStr)
		} else {
			outputAddrs2 = append(outputAddrs2, addrStr)
		}

		outputValue[addrStr] = msgtx.TxOut[i].Value

		if omniReceiver == "" {
			if addrStr == omniSender {
				if foundSender {
					omniReceiver = addrStr
				} else {
					foundSender = true
				}
			} else {
				//find the omni receiver
				omniReceiver = addrStr
			}
		}
	}

	message := NotifyMessage{
		MessageType: NOTIFY_TYPE_TX,
		TxHash:      hash,
		Fee:         big.NewInt(int64(fee)),
		Coin:        strings.ToUpper(chainName),
	}
	if strings.EqualFold(chainName, "btc") && opReturnNum == 1 && omniReceiver != "" {
		omniScript := msgtx.TxOut[firstOpReturnPos].PkScript
		_, senderExist := util.LoadAddrPath(omniSender)
		_, receiverExist := util.LoadAddrPath(omniReceiver)
		txType := -1
		if !senderExist && receiverExist {
			txType = 0
		} else if senderExist && !receiverExist {
			txType = 1
		} else if senderExist && receiverExist {
			log.Println("inner USDT tx")
		}

		if txType >= 0 {
			message.TxType = txType
			if len(omniScript) != 22 {
				log.Println("script len is invalid, ignore it")
			} else {
				//only support USDT@btc
				omniTemplate := []byte("omni\x00\x00\x00\x00\x00\x00\x00\x1f")
				if bytes.Compare(omniScript[2:14], omniTemplate) != 0 {
					log.Println("not a usdt send tx, ignore it")
				} else {
					omniValue := binary.BigEndian.Uint64(omniScript[14:])
					message.Coin = "USDT"
					message.Address = omniReceiver
					message.Amount = new(big.Int).SetUint64(omniValue)
					messages = append(messages, message)
					log.Println("USDT tx found, type:", message.TxType, hash, omniReceiver, omniValue)
				}
			}
		}
	}

	message.Coin = strings.ToUpper(chainName)
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

func ReadBlock(client *rpcclient.Client, block *big.Int, chainName string) ([]NotifyMessage, error) {
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
		message, err := ParseTransaction(client, tx, chainName)
		if err == nil {
			messages = append(messages, message...)
		}
	}

	return messages, nil
}

func SendTransaction(config *conf.Config, tx *wire.MsgTx) (string, error) {
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

func ParseMempoolTransaction(client *rpcclient.Client, msgtx *wire.MsgTx, chainName string) (err error) {
	param := util.GetParamByName(chainName)
	if msgtx == nil {
		return fmt.Errorf("Transaction is nil: Can't parse.")
	}

	hash := msgtx.TxHash().String()
	for i := 0; i < len(msgtx.TxIn); i++ {
		prevHash := msgtx.TxIn[i].PreviousOutPoint.Hash
		prevIndex := msgtx.TxIn[i].PreviousOutPoint.Index
		//ignore coinbase input
		if prevIndex == 0xFFFFFFFF {
			continue
		}
		tx, err := client.GetRawTransaction(&prevHash)
		if err != nil {
			log.Println("get transaction err:", err, hash, i, prevHash.String())
			continue
		}
		value := tx.MsgTx().TxOut[prevIndex].Value

		script := tx.MsgTx().TxOut[prevIndex].PkScript
		_, addrSet, _, err := txscript.ExtractPkScriptAddrs(
			script, param)
		if err != nil {
			log.Println("parse input pkscript err:", err, hash, i)
			continue
		}

		addrStr := addrSet[0].EncodeAddress()
		_, ok := util.LoadAddrPath(addrStr)
		if ok {
			removeUtxo(prevHash.String(), prevIndex, addrStr, value)
			log.Println("remove utxo:", prevHash.String(), prevIndex, addrStr)
		}
	}

	for i := 0; i < len(msgtx.TxOut); i++ {
		if txscript.GetScriptClass(msgtx.TxOut[i].PkScript) == txscript.NullDataTy {
			continue
		}

		_, addrSet, _, err := txscript.ExtractPkScriptAddrs(
			msgtx.TxOut[i].PkScript, param)
		if err != nil {
			log.Println("parse output pkscript err:", err, hash, i)
			continue
		}

		if len(addrSet) == 0 {
			log.Println("cannot get address", hash, i)
			continue
		}

		addrStr := addrSet[0].EncodeAddress()
		_, ok := util.LoadAddrPath(addrStr)
		if ok {
			createUtxo(hash, uint32(i), addrStr, msgtx.TxOut[i].Value)
			log.Println("add utxo:", hash, i, addrStr)
		}
	}

	return
}
