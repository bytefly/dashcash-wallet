package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/bitcoinsv/bsvd/bsvec"
	bsvtxscript "github.com/bitcoinsv/bsvd/txscript"
	bsvwire "github.com/bitcoinsv/bsvd/wire"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	conf "github.com/bytefly/dashcash-wallet/config"
	"github.com/bytefly/dashcash-wallet/util"
	"github.com/gcash/bchd/bchec"
	bchtxscript "github.com/gcash/bchd/txscript"
	bchwire "github.com/gcash/bchd/wire"
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
	Script  []byte
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
	Amount    string    `json:"amount"`
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

func getScriptFromAddress(address string, param *chaincfg.Params) ([]byte, error) {
	var script []byte
	addr, _ := btcutil.DecodeAddress(address, param)
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

func BuildRawMsgTx(param *chaincfg.Params, inputs []TxInput, outputs []TxOut) (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(wire.TxVersion)

	if inputs != nil {
		for i := 0; i < len(inputs); i++ {
			utxoHash, _ := chainhash.NewHashFromStr(inputs[i].Hash)
			point := wire.OutPoint{Hash: *utxoHash, Index: inputs[i].Index}
			tx.AddTxIn(wire.NewTxIn(&point, nil, nil))
		}
	}

	for i := 0; i < len(outputs); i++ {
		if outputs[i].Address != "" {
			script, err := getScriptFromAddress(outputs[i].Address, param)
			if err != nil {
				return nil, err
			}

			tx.AddTxOut(wire.NewTxOut(outputs[i].Amount, script))
		} else {
			tx.AddTxOut(wire.NewTxOut(outputs[i].Amount, outputs[i].Script))
		}
	}

	return tx, nil
}

func BuildSignedMsgTx(chain, xpriv string, inputs []TxInput, outputs []TxOut) (*wire.MsgTx, error) {
	param := util.GetParamByName(chain)
	tx, err := BuildRawMsgTx(param, inputs, outputs)
	if err != nil {
		return nil, err
	}

	return SignMsgTx(chain, xpriv, tx)
}

func SignMsgTx(chain, xpriv string, tx *wire.MsgTx) (*wire.MsgTx, error) {
	signedTx := tx.Copy()
	param := util.GetParamByName(chain)
	onBCH := false
	onBSV := false
	var bchTx bchwire.MsgTx
	var bsvTx bsvwire.MsgTx
	if strings.HasPrefix(strings.ToLower(chain), "bch") {
		onBCH = true
	}
	if strings.HasPrefix(strings.ToLower(chain), "bsv") {
		onBSV = true
	}
	if onBCH || onBSV {
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSizeStripped()))
		_ = tx.SerializeNoWitness(buf)
		tx.SerializeNoWitness(buf)
		if onBCH {
			bchTx.Deserialize(buf)
		}
		if onBSV {
			bsvTx.Deserialize(buf)
		}
	}

	for i := 0; i < len(signedTx.TxIn); i++ {
		outPoint := tx.TxIn[i].PreviousOutPoint
		out, err := GetUtxoByKey(outPoint.Hash.String(), outPoint.Index)
		if err != nil {
			log.Println("get utxo err:", err)
			return nil, err
		}

		address := out.Address
		val, ok := util.LoadAddrPath(address)
		if !ok {
			log.Println("utxo not fround in wallet")
			return nil, errors.New("Unspendable utxo found")
		}

		if onBCH {
			address, _ = util.ConvertCashAddrToLegacy(address, param)
		}
		script, err := getScriptFromAddress(address, param)
		if err != nil {
			return nil, err
		}

		pos := strings.IndexByte(val, '/')
		branch, _ := strconv.ParseInt(val[0:pos], 10, 32)
		addrId, _ := strconv.ParseInt(val[pos+1:], 10, 32)
		if branch != 1 {
			log.Println("input must only come from inner address")
			return nil, errors.New("invalid input")
		}
		privKey, _ := util.GetPrivateKey(xpriv, int(branch), int(addrId))
		//log.Println("privkey:", hex.EncodeToString(privKey.ToECDSA().D.Bytes()))
		if onBCH {
			signedTx.TxIn[i].SignatureScript, err = bchtxscript.SignatureScript(
				&bchTx,     // The tx to be signed.
				i,          // The index of the txin the signature is for.
				out.Amount, //amount of the input
				script,     // The other half of the script from the PubKeyHash.
				bchtxscript.SigHashForkID|bchtxscript.SigHashAll, // The signature flags that indicate what the sig covers.
				(*bchec.PrivateKey)(privKey),                     // The key to generate the signature with.
				true)                                             // The compress sig flag. This saves space on the blockchain.
		} else if onBSV {
			signedTx.TxIn[i].SignatureScript, err = bsvtxscript.SignatureScript(
				&bsvTx,     // The tx to be signed.
				i,          // The index of the txin the signature is for.
				out.Amount, //amount of the input
				script,     // The other half of the script from the PubKeyHash.
				bsvtxscript.SigHashForkID|bsvtxscript.SigHashAll, // The signature flags that indicate what the sig covers.
				(*bsvec.PrivateKey)(privKey),                     // The key to generate the signature with.
				true)                                             // The compress sig flag. This saves space on the blockchain.
		} else {
			signedTx.TxIn[i].SignatureScript, err = txscript.SignatureScript(
				signedTx,            // The tx to be signed.
				i,                   // The index of the txin the signature is for.
				script,              // The other half of the script from the PubKeyHash.
				txscript.SigHashAll, // The signature flags that indicate what the sig covers.
				privKey,             // The key to generate the signature with.
				true)                // The compress sig flag. This saves space on the blockchain.
		}
		if err != nil {
			log.Println("create signature error:", err)
			return nil, err
		}
		if onBCH {
			bchTx.TxIn[i].SignatureScript = make([]byte, len(signedTx.TxIn[i].SignatureScript))
			copy(bchTx.TxIn[i].SignatureScript, signedTx.TxIn[i].SignatureScript)
		}
		if onBSV {
			bsvTx.TxIn[i].SignatureScript = make([]byte, len(signedTx.TxIn[i].SignatureScript))
			copy(bsvTx.TxIn[i].SignatureScript, signedTx.TxIn[i].SignatureScript)
		}
	}

	return signedTx, nil
}

func DumpMsgTxInput(tx *wire.MsgTx) {
	for i := 0; i < len(tx.TxIn); i++ {
		log.Println(tx.TxIn[i].PreviousOutPoint.String())
	}
}

func DumpMsgTxOutput(tx *wire.MsgTx, param *chaincfg.Params) {
	for i := 0; i < len(tx.TxOut); i++ {
		_, addrSet, _, err := txscript.ExtractPkScriptAddrs(
			tx.TxOut[i].PkScript, param)
		if err == nil {
			log.Println(addrSet[0].EncodeAddress(), tx.TxOut[i].Value)
		} else {
			log.Println("parse pkscript err:", err, i)
		}
	}
}

func PrepareTrezorSign(config *conf.Config, tx *wire.MsgTx) (string, error) {
	var trezorTx TrezorTx
	client, err := ConnectRPC(config)
	param := util.GetParamByName(config.ChainName)
	onBCH := false
	if strings.HasPrefix(strings.ToLower(config.ChainName), "bch") {
		onBCH = true
	}

	if err != nil {
		return "", err
	}
	defer client.Shutdown()

	trezorTx.Coin = config.ChainName
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

		val, _ := util.LoadAddrPath(out.Address)
		pos := strings.IndexByte(val, '/')
		branch, _ := strconv.ParseInt(val[0:pos], 10, 32)
		addrId, _ := strconv.ParseInt(val[pos+1:], 10, 32)

		trezorTx.Inputs[i].AddressN[0] = 44 | 0x80000000
		trezorTx.Inputs[i].AddressN[1] = param.HDCoinType | 0x80000000
		trezorTx.Inputs[i].AddressN[2] = 0 | 0x80000000 //account 0
		trezorTx.Inputs[i].AddressN[3] = uint32(branch)
		trezorTx.Inputs[i].AddressN[4] = uint32(addrId)
		trezorTx.Inputs[i].PrevIndex = int(prevIndex)
		trezorTx.Inputs[i].PrevHash = prevHash
		trezorTx.Inputs[i].Amount = strconv.FormatInt(out.Amount, 10)

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
			tx.TxOut[i].PkScript, param)
		if err != nil {
			log.Println("parse pkscript err:", err, i)
			return "", err
		}
		trezorTx.Outputs[i] = make(TrezorOutput)
		if len(addrSet) == 0 {
			trezorTx.Outputs[i]["amount"] = "0"
			trezorTx.Outputs[i]["script_type"] = "PAYTOOPRETURN"
			trezorTx.Outputs[i]["op_return_data"] = hex.EncodeToString(tx.TxOut[i].PkScript[2:])
		} else {
			if onBCH {
				trezorTx.Outputs[i]["address"], _ = util.ConvertLegacyToCashAddr(addrSet[0].EncodeAddress(), param)
			} else {
				trezorTx.Outputs[i]["address"] = addrSet[0].EncodeAddress()
			}
			trezorTx.Outputs[i]["amount"] = strconv.FormatInt(tx.TxOut[i].Value, 10)
			trezorTx.Outputs[i]["script_type"] = "PAYTOADDRESS"
		}
	}

	str, err := json.Marshal(&trezorTx)
	if err != nil {
		return "", err
	}
	return string(str), nil
}
