package main

import (
	"encoding/hex"
	"errors"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"log"
	"math/big"
	"strconv"
	"strings"
)

type TxInput struct {
	Hash    string
	Index   uint32
	Script  string
	Address string
}

type TxOut struct {
	Address string
	Amount  *big.Int
}

func buildMsgTx(xpriv string, inputs []TxInput, outputs []TxOut) (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(wire.TxVersion)

	for i := 0; i < len(inputs); i++ {
		utxoHash, _ := chainhash.NewHashFromStr(inputs[i].Hash)
		point := wire.OutPoint{Hash: *utxoHash, Index: inputs[i].Index}
		tx.AddTxIn(wire.NewTxIn(&point, nil, nil))
	}

	for i := 0; i < len(outputs); i++ {
		var script []byte
		addr, _ := btcutil.DecodeAddress(outputs[i].Address, &DSCMainNetParams)
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

		amount := outputs[i].Amount.Int64()
		tx.AddTxOut(wire.NewTxOut(amount, script))
	}

	for i := 0; i < len(inputs); i++ {
		script, _ := hex.DecodeString(inputs[i].Script)
		val, ok := addrs.Load(inputs[i].Address)
		if !ok {
			log.Println("utxo not fround in wallet")
			return nil, errors.New("Unspendable utxo found")
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
