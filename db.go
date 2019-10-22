package main

import (
	"fmt"
	"log"
	"math/big"
	"strings"

	badger "github.com/dgraph-io/badger"
)

var db *badger.DB

func openDb() error {
	var err error
	db, err = badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		return err
	}
	return nil
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
			if err != nil {
				return err
			}
			return nil
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
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			err := item.Value(func(v []byte) error {
				pos := strings.IndexByte(string(k), '/')
				hash := k[:pos]
				index := k[pos+1:]

				pos = strings.IndexByte(string(v), ':')
				addr := v[:pos]
				val := v[pos+1:]

				if address == "" || address == string(addr) {
					valInt, _ := new(big.Int).SetString(string(val), 10)
					balance.Add(balance, valInt)
				}
				log.Printf("key=%s, value=%s hash: %s index: %s\n", k, v, hash, index)
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return balance, nil
}
