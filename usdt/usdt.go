package usdt

import (
	"encoding/binary"
	"github.com/btcsuite/btcd/txscript"
	conf "github.com/bytefly/dashcash-wallet/config"
	"github.com/bytefly/dashcash-wallet/util"
	"github.com/ibclabs/omnilayer-go"
	"github.com/ibclabs/omnilayer-go/omnijson"
	"log"
	"strconv"
)

const (
	OMNIToken = 31
)

func GetUSDTBalance(config *conf.Config, addr string) (balance int64, err error) {
	var c = &omnilayer.ConnConfig{
		Host:                 config.RPCURL,
		User:                 config.RPCUser,
		Pass:                 config.RPCPass,
		DisableAutoReconnect: false,
		DisableConnectOnNew:  false,
		EnableBCInfoHacks:    true,
	}
	client := omnilayer.New(c)
	cmd := &omnijson.OmniGetBalanceCommand{addr, OMNIToken}
	result, err := client.OmniGetBalance(*cmd)
	if err != nil {
		log.Println("get balance for", addr, "err:", err)
		return
	}

	balance, err = strconv.ParseInt(util.RightShift(result.Balance, 8), 10, 64)
	return
}

func GetOMNIPendingAmount(config *conf.Config, addr string, tokenId uint32) (pendingAmount int64, err error) {
	var c = &omnilayer.ConnConfig{
		Host:                 config.RPCURL,
		User:                 config.RPCUser,
		Pass:                 config.RPCPass,
		DisableAutoReconnect: false,
		DisableConnectOnNew:  false,
		EnableBCInfoHacks:    true,
	}
	client := omnilayer.New(c)

	cmd := &omnijson.OmniListPendingTransactionsCommand{addr}
	result, err := client.OmniListPendingTransactions(*cmd)
	if err != nil {
		log.Println("list pending transactions for", addr, "err:", err)
		return
	}

	for _, v := range result {
		if v.From != addr {
			continue
		} else if v.Confirmations > 0 {
			continue
		} else if v.PropertyID != tokenId {
			continue
		} else if v.Type != "Simple Send" {
			continue
		}
		log.Println(v.ID, v.Amount)
		amount, _ := strconv.ParseInt(util.RightShift(v.Amount, 8), 10, 64)
		pendingAmount += amount
	}

	return
}

func GetOmniUsdtScript(amount uint64) []byte {
	payload := []byte("omni")

	bs := make([]byte, 8)
	binary.BigEndian.PutUint16(bs, 0) //version = 0
	payload = append(payload, bs[0:2]...)

	binary.BigEndian.PutUint16(bs, 0) //type = simple_send
	payload = append(payload, bs[0:2]...)

	binary.BigEndian.PutUint32(bs, OMNIToken) //coin type = USDT
	payload = append(payload, bs[0:4]...)

	binary.BigEndian.PutUint64(bs, amount) //coin value
	payload = append(payload, bs[0:8]...)

	script, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData(payload).Script()
	return script
}
