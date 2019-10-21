package main

import (
	"fmt"
	"log"

	badger "github.com/dgraph-io/badger"
)

var db *badger.DB

func openDb() error {
	var err error
	db, err = badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		log.Println(err)
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
