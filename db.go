package main

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"log"
	"math/big"
	"strconv"
	"strings"

	badger "github.com/dgraph-io/badger"
)

var db *badger.DB

const (
	// standard tx fee per kb of tx size (bitcoind 0.12 default min-relay fee-rate)
	TX_FEE_PER_KB  = 1000
	MIN_FEE_PER_KB = TX_FEE_PER_KB
	// estimated size for a typical transaction output
	TX_OUTPUT_SIZE = 34
	// estimated size for a typical compact pubkey transaction input
	TX_INPUT_SIZE = 148
	//no txout can be below this amount
	TX_MIN_OUTPUT_AMOUNT = (TX_FEE_PER_KB * 3 * (TX_OUTPUT_SIZE + TX_INPUT_SIZE) / 1000)
	// no tx can be larger than this size in bytes
	TX_MAX_SIZE = 100000
)

func openDb() error {
	var err error
	db, err = badger.Open(badger.DefaultOptions("/tmp/badger"))
	return err
}

func closeDb() {
	if db != nil {
		db.Close()
	}
}

func createUtxo(hash string, index uint32, address string, value int64) error {
	err := db.Update(func(txn *badger.Txn) error {
		key := fmt.Sprintf("%s/%d", hash, index)
		val := fmt.Sprintf("%s:%d", address, value)

		_, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			// Use the transaction...
			err = txn.Set([]byte(key), []byte(val))
			return err
		}

		return err
	})

	return err
}

func removeUtxo(hash string, index uint32, address string, value int64) error {
	err := db.Update(func(txn *badger.Txn) error {
		key := fmt.Sprintf("%s/%d", hash, index)
		err := txn.Delete([]byte(key))
		return err
	})

	return err
}

func getBalance(address string) (*big.Int, error) {
	balance := new(big.Int)
	db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			item.Value(func(v []byte) error {
				pos := strings.IndexByte(string(v), ':')
				addr := v[:pos]
				val := v[pos+1:]

				if address == "" || address == string(addr) {
					valInt, _ := new(big.Int).SetString(string(val), 10)
					balance.Add(balance, valInt)
				}
				return nil
			})
		}

		return nil
	})

	return balance, nil
}

func GetUtxoByKey(hash string, index uint32) (*TxOut, error) {
	out := new(TxOut)
	err := db.View(func(txn *badger.Txn) error {
		key := hash + "/" + strconv.FormatUint(uint64(index), 10)
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		err = item.Value(func(v []byte) error {
			pos := strings.IndexByte(string(v), ':')
			out.Address = string(v[:pos])
			out.Amount, _ = strconv.ParseInt(string(v[pos+1:]), 10, 64)
			return nil
		})
		return err
	})

	if err != nil {
		return nil, err
	}
	return out, nil
}

func GetAllUtxo(address string) ([]Utxo, error) {
	utxos := make([]Utxo, 0)
	db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			item.Value(func(v []byte) error {
				pos := strings.IndexByte(string(k), '/')
				hash := string(k[:pos])
				index, _ := strconv.ParseInt(string(k[pos+1:]), 10, 32)

				pos = strings.IndexByte(string(v), ':')
				addr := string(v[:pos])
				val, _ := strconv.ParseInt(string(v[pos+1:]), 10, 64)

				if address == "" || address == string(addr) {
					utxo := Utxo{Hash: hash, Index: uint32(index), Address: addr, Value: val}
					utxos = append(utxos, utxo)
				}
				return nil
			})
		}

		return nil
	})

	return utxos, nil
}

func txFee(feePerKb uint32, size uint32) int64 {
	// standard fee based on tx size
	standardFee := int64(size*TX_FEE_PER_KB) / 1000
	// fee using feePerKb, rounded up to nearest 100 satoshi
	fee := (((int64(size*feePerKb) / 1000) + 99) / 100) * 100

	if fee > standardFee {
		return fee
	}
	return standardFee
}

// outputs below this amount are uneconomical due to fees (TX_MIN_OUTPUT_AMOUNT is the absolute minimum output amount)
func minOutputAmount(feePerKb uint32) int64 {
	amount := (int64(TX_MIN_OUTPUT_AMOUNT*feePerKb) + MIN_FEE_PER_KB - 1) / MIN_FEE_PER_KB
	if amount > TX_MIN_OUTPUT_AMOUNT {
		return amount
	}
	return TX_MIN_OUTPUT_AMOUNT
}

func CreateTxForOutputs(feePerKb uint32, outputs []TxOut, changeAddress string) (*wire.MsgTx, bool) {
	var (
		totalBalance int64
		balance      int64
		cpfpSize     int
		amount       int64
	)

	inputs := make([]TxInput, 0)
	// caching all utxos may be better
	utxos, err := GetAllUtxo("")

	if err != nil || len(utxos) == 0 {
		return nil, false
	}

	for _, o := range utxos {
		totalBalance += o.Value
	}

	for i := 0; i < len(outputs); i++ {
		amount += outputs[i].Amount
	}

	//build a inital skeleton transaction
	tx, err := BuildRawMsgTx(inputs, outputs)
	if err != nil {
		return nil, false
	}

	minAmount := minOutputAmount(feePerKb)
	feeAmount := txFee(feePerKb, uint32(tx.SerializeSize())+TX_OUTPUT_SIZE)

	// TODO: use up all UTXOs for all used addresses to avoid leaving funds in addresses whose public key is revealed
	// TODO: avoid combining addresses in a single transaction when possible to reduce information leakage
	// TODO: use up UTXOs received from any of the output scripts that this transaction sends funds to, to mitigate an
	//       attacker double spending and requesting a refund
	for i := 0; i < len(utxos); i++ {
		utxoHash, _ := chainhash.NewHashFromStr(utxos[i].Hash)
		point := wire.OutPoint{Hash: *utxoHash, Index: utxos[i].Index}
		tx.AddTxIn(wire.NewTxIn(&point, nil, nil))

		if tx.SerializeSize()+TX_OUTPUT_SIZE > TX_MAX_SIZE { // transaction size-in-bytes too large
			tx = nil

			// check for sufficient total funds before building a smaller transaction
			if totalBalance < amount+txFee(feePerKb, uint32(10+len(utxos)*TX_INPUT_SIZE+
				(len(outputs)+1)*TX_OUTPUT_SIZE+cpfpSize)) {
				break
			}

			if outputs[len(outputs)-1].Amount > amount+feeAmount+minAmount-balance {
				newOutputs := make([]TxOut, 0)

				for j := 0; j < len(outputs); j++ {
					newOutputs = append(newOutputs, outputs[j])
				}

				newOutputs[len(newOutputs)-1].Amount -= amount + feeAmount - balance // reduce last output amount
				tx, _ = CreateTxForOutputs(feePerKb, newOutputs, changeAddress)
			} else {
				tx, _ = CreateTxForOutputs(feePerKb, outputs[0:len(outputs)-1], changeAddress) // remove last output
			}

			balance = 0
			amount = 0
			feeAmount = 0
			break
		}

		balance += utxos[i].Value

		//  size of unconfirmed, non-change inputs for child-pays-for-parent fee
		//  don't include parent tx with more than 10 inputs or 10 outputs
		//  if (tx->blockHeight == TX_UNCONFIRMED && tx->inCount <= 10 && tx->outCount <= 10 &&
		//  ! _BRWalletTxIsSend(wallet, tx)) cpfpSize += transactionVSize(tx);

		// fee amount after adding a change output
		feeAmount = txFee(feePerKb, uint32(tx.SerializeSize()+TX_OUTPUT_SIZE+cpfpSize))

		// increase fee to round off remaining wallet balance to nearest 100 satoshi
		if totalBalance > amount+feeAmount {
			feeAmount += (totalBalance - (amount + feeAmount)) % 100
		}

		if balance == amount+feeAmount || balance >= amount+feeAmount+minAmount {
			break
		}
	}

	if (tx != nil) && (len(outputs) < 1 || balance < amount+feeAmount) { // no outputs/insufficient funds
		log.Println("no outputs/insufficient funds")
		return nil, false
	} else if (tx != nil) && balance-(amount+feeAmount) > minAmount { // add change output
		if changeAddress == "" {
			// if no change address pass in, use the first input address
			out, err := GetUtxoByKey(tx.TxIn[0].PreviousOutPoint.Hash.String(), tx.TxIn[0].PreviousOutPoint.Index)
			if err != nil {
				log.Println("utxo may be spent, you can try again")
				return nil, false
			}
			changeAddress = out.Address
		}
		script, _ := getScriptFromAddress(changeAddress)
		tx.AddTxOut(wire.NewTxOut(balance-(amount+feeAmount), script))
		return tx, true
	}

	return tx, false
}
