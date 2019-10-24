package main

import (
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

func getScriptFromAddress(address string) ([]byte, error) {
	var script []byte
	addr, _ := btcutil.DecodeAddress(address, &DSCMainNetParams)
	switch addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash, *btcutil.AddressWitnessScriptHash:
		script, _ = txscript.NewScriptBuilder().AddOp(txscript.OP_0).
			AddData(addr.ScriptAddress()).Script()
	case *btcutil.AddressPubKey: //TODO
		log.Println("pub key address is not supported")
		return nil, errors.New("Pub key addr not supported")
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
