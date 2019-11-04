package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"log"
	"strconv"
	"strings"
)

type TxInput struct {
	Hash    string
	Index   uint32
	Address string
}

type TxOut struct {
	Address string
	Amount  int64
}

type Utxo struct {
	Hash    string
	Index   uint32
	Address string
	Value   int64
}

type TrezorInput struct {
	AddressN  [5]uint32 `json:"address_n"`
	PrevIndex int       `json:"prev_index"`
	PrevHash  string    `json:"prev_hash"`
}

type TrezorOutput map[string]string

type TrezorRefInput struct {
	PrevHash  string `json:"prev_hash"`
	PrevIndex uint32 `json:"prev_index"`
	ScriptSig string `json:"script_sig"`
	Sequence  uint32 `json:"sequence"`
}

type TrezorRefOutput struct {
	Amount   int64  `json:"amount"`
	PkScript string `json:"script_pubkey"`
}

type TrezorRefTx struct {
	Hash     string            `json:"hash"`
	Inputs   []TrezorRefInput  `json:"inputs"`
	Outputs  []TrezorRefOutput `json:"bin_outputs"`
	Version  int32             `json:"version"`
	LockTime uint32            `json:"lock_time"`
}

type TrezorTx struct {
	Coin    string         `json:"coin"`
	Push    bool           `json:"push"`
	Inputs  []TrezorInput  `json:"inputs"`
	Outputs []TrezorOutput `json:"outputs"`
	RefTxs  []TrezorRefTx  `json:"refTxs"`
}

func getScriptFromAddress(address string) ([]byte, error) {
	var script []byte
	addr, _ := btcutil.DecodeAddress(address, &DSCMainNetParams)
	switch addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash, *btcutil.AddressWitnessScriptHash:
		script, _ = txscript.NewScriptBuilder().AddOp(txscript.OP_0).
			AddData(addr.ScriptAddress()).Script()
	case *btcutil.AddressPubKey:
		script, _ = txscript.NewScriptBuilder().AddData(addr.ScriptAddress()).AddOp(txscript.OP_CHECKSIG).Script()
	case *btcutil.AddressPubKeyHash:
		script, _ = txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).
			AddData(addr.ScriptAddress()).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).
			Script()
	case *btcutil.AddressScriptHash:
		script, _ = txscript.NewScriptBuilder().AddOp(txscript.OP_HASH160).
			AddData(addr.ScriptAddress()).AddOp(txscript.OP_EQUAL).Script()
	}

	return script, nil
}

func BuildRawMsgTx(inputs []TxInput, outputs []TxOut) (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(wire.TxVersion)

	for i := 0; i < len(inputs); i++ {
		utxoHash, _ := chainhash.NewHashFromStr(inputs[i].Hash)
		point := wire.OutPoint{Hash: *utxoHash, Index: inputs[i].Index}
		tx.AddTxIn(wire.NewTxIn(&point, nil, nil))
	}

	for i := 0; i < len(outputs); i++ {
		script, err := getScriptFromAddress(outputs[i].Address)
		if err != nil {
			return nil, err
		}

		amount := outputs[i].Amount
		tx.AddTxOut(wire.NewTxOut(amount, script))
	}

	return tx, nil
}

func BuildSignedMsgTx(xpriv string, inputs []TxInput, outputs []TxOut) (*wire.MsgTx, error) {
	tx, err := BuildRawMsgTx(inputs, outputs)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(inputs); i++ {
		val, ok := addrs.Load(inputs[i].Address)
		if !ok {
			log.Println("utxo not fround in wallet")
			return nil, errors.New("Unspendable utxo found")
		}

		script, err := getScriptFromAddress(inputs[i].Address)
		if err != nil {
			return nil, err
		}

		pos := strings.IndexByte(val.(string), '/')
		branch, _ := strconv.ParseInt(val.(string)[0:pos], 10, 32)
		addrId, _ := strconv.ParseInt(val.(string)[pos+1:], 10, 32)
		if branch != 1 {
			log.Println("input must only come from inner address")
			return nil, errors.New("invalid input")
		}
		privKey, _ := GetPrivateKey(xpriv, int(branch), int(addrId))
		//log.Println("privkey:", hex.EncodeToString(privKey.ToECDSA().D.Bytes()))
		sig, err := txscript.SignatureScript(
			tx,                  // The tx to be signed.
			i,                   // The index of the txin the signature is for.
			script,              // The other half of the script from the PubKeyHash.
			txscript.SigHashAll, // The signature flags that indicate what the sig covers.
			privKey,             // The key to generate the signature with.
			true)                // The compress sig flag. This saves space on the blockchain.
		if err != nil {
			log.Println("create signature error:", err)
			return nil, err
		}
		tx.TxIn[i].SignatureScript = sig
	}

	return tx, nil
}

func SignMsgTx(xpriv string, tx *wire.MsgTx) (*wire.MsgTx, error) {
	signedTx := tx.Copy()
	for i := 0; i < len(signedTx.TxIn); i++ {
		outPoint := tx.TxIn[i].PreviousOutPoint
		out, err := GetUtxoByKey(outPoint.Hash.String(), outPoint.Index)
		if err != nil {
			log.Println("get utxo err:", err)
			return nil, err
		}

		address := out.Address
		val, ok := addrs.Load(address)
		if !ok {
			log.Println("utxo not fround in wallet")
			return nil, errors.New("Unspendable utxo found")
		}

		script, err := getScriptFromAddress(address)
		if err != nil {
			return nil, err
		}

		pos := strings.IndexByte(val.(string), '/')
		branch, _ := strconv.ParseInt(val.(string)[0:pos], 10, 32)
		addrId, _ := strconv.ParseInt(val.(string)[pos+1:], 10, 32)
		if branch != 1 {
			log.Println("input must only come from inner address")
			return nil, errors.New("invalid input")
		}
		privKey, _ := GetPrivateKey(xpriv, int(branch), int(addrId))
		//log.Println("privkey:", hex.EncodeToString(privKey.ToECDSA().D.Bytes()))
		sig, err := txscript.SignatureScript(
			signedTx,            // The tx to be signed.
			i,                   // The index of the txin the signature is for.
			script,              // The other half of the script from the PubKeyHash.
			txscript.SigHashAll, // The signature flags that indicate what the sig covers.
			privKey,             // The key to generate the signature with.
			true)                // The compress sig flag. This saves space on the blockchain.
		if err != nil {
			log.Println("create signature error:", err)
			return nil, err
		}
		signedTx.TxIn[i].SignatureScript = sig
	}

	return signedTx, nil
}

func DumpMsgTxInput(tx *wire.MsgTx) {
	for i := 0; i < len(tx.TxIn); i++ {
		log.Println(tx.TxIn[i].PreviousOutPoint.String())
	}
}

func DumpMsgTxOutput(tx *wire.MsgTx) {
	for i := 0; i < len(tx.TxOut); i++ {
		_, addrSet, _, err := txscript.ExtractPkScriptAddrs(
			tx.TxOut[i].PkScript, &DSCMainNetParams)
		if err == nil {
			log.Println(addrSet[0].EncodeAddress(), tx.TxOut[i].Value)
		} else {
			log.Println("parse pkscript err:", err, i)
		}
	}
}

func PrepareTrezorSign(config *Config, tx *wire.MsgTx) (string, error) {
	var trezorTx TrezorTx
	client, err := ConnectRPC(config)

	if err != nil {
		return "", err
	}
	defer client.Shutdown()

	trezorTx.Coin = "dsc"
	// we have no blockbook instance installed
	trezorTx.Push = false

	trezorTx.Inputs = make([]TrezorInput, len(tx.TxIn))
	trezorTx.Outputs = make([]TrezorOutput, len(tx.TxOut))
	trezorTx.RefTxs = make([]TrezorRefTx, len(tx.TxIn))
	for i := 0; i < len(tx.TxIn); i++ {
		prevHash := tx.TxIn[i].PreviousOutPoint.Hash.String()
		prevIndex := tx.TxIn[i].PreviousOutPoint.Index
		out, err := GetUtxoByKey(prevHash, prevIndex)
		if err != nil {
			log.Println("the utxo may be spent:", prevHash, prevIndex)
			return "", err
		}

		val, _ := addrs.Load(out.Address)
		pos := strings.IndexByte(val.(string), '/')
		branch, _ := strconv.ParseInt(val.(string)[0:pos], 10, 32)
		addrId, _ := strconv.ParseInt(val.(string)[pos+1:], 10, 32)

		trezorTx.Inputs[i].AddressN[0] = 44 | 0x80000000
		trezorTx.Inputs[i].AddressN[1] = 0 | 0x80000000 // FIXME: must be 1208
		trezorTx.Inputs[i].AddressN[2] = 0 | 0x80000000 //account 0
		trezorTx.Inputs[i].AddressN[3] = uint32(branch)
		trezorTx.Inputs[i].AddressN[4] = uint32(addrId)
		trezorTx.Inputs[i].PrevIndex = int(prevIndex)
		trezorTx.Inputs[i].PrevHash = prevHash

		trezorTx.RefTxs[i].Hash = prevHash
		prevTx, err := client.GetRawTransaction(&tx.TxIn[i].PreviousOutPoint.Hash)
		if err != nil {
			log.Println("read tx info err:", err, prevHash)
			return "", err
		}

		trezorTx.RefTxs[i].Version = prevTx.MsgTx().Version
		trezorTx.RefTxs[i].LockTime = prevTx.MsgTx().LockTime
		trezorTx.RefTxs[i].Inputs = make([]TrezorRefInput, len(prevTx.MsgTx().TxIn))
		trezorTx.RefTxs[i].Outputs = make([]TrezorRefOutput, len(prevTx.MsgTx().TxOut))
		for j := 0; j < len(prevTx.MsgTx().TxIn); j++ {
			trezorTx.RefTxs[i].Inputs[j].PrevHash = prevTx.MsgTx().TxIn[j].PreviousOutPoint.Hash.String()
			trezorTx.RefTxs[i].Inputs[j].PrevIndex = prevTx.MsgTx().TxIn[j].PreviousOutPoint.Index
			trezorTx.RefTxs[i].Inputs[j].ScriptSig = hex.EncodeToString(prevTx.MsgTx().TxIn[j].SignatureScript)
			trezorTx.RefTxs[i].Inputs[j].Sequence = prevTx.MsgTx().TxIn[j].Sequence
		}
		for j := 0; j < len(prevTx.MsgTx().TxOut); j++ {
			trezorTx.RefTxs[i].Outputs[j].Amount = prevTx.MsgTx().TxOut[j].Value
			trezorTx.RefTxs[i].Outputs[j].PkScript = hex.EncodeToString(prevTx.MsgTx().TxOut[j].PkScript)
		}
	}

	for i := 0; i < len(tx.TxOut); i++ {
		_, addrSet, _, err := txscript.ExtractPkScriptAddrs(
			tx.TxOut[i].PkScript, &DSCMainNetParams)
		if err != nil {
			log.Println("parse pkscript err:", err, i)
			return "", err
		}
		trezorTx.Outputs[i] = make(TrezorOutput)
		trezorTx.Outputs[i]["address"] = addrSet[0].EncodeAddress()
		trezorTx.Outputs[i]["amount"] = strconv.FormatInt(tx.TxOut[i].Value, 10)
		trezorTx.Outputs[i]["script_type"] = "PAYTOADDRESS"
	}

	str, err := json.Marshal(&trezorTx)
	if err != nil {
		return "", err
	}
	return string(str), nil
}
