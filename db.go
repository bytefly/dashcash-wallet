package main

import (
	"log"

	badger "github.com/dgraph-io/badger"
)

func test() error {
	// Open the Badger database located in the /tmp/badger directory.
	// It will be created if it doesn't exist.
	db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	//read-only
	err = db.View(func(txn *badger.Txn) error {
		return nil
	})

	//read-write
	err = db.Update(func(txn *badger.Txn) error {
		return nil
	})

	// Start a writable transaction.
	txn := db.NewTransaction(true)
	defer txn.Discard()

	// Use the transaction...
	err = txn.Set([]byte("answer"), []byte("42"))
	if err != nil {
		return err
	}

	// Commit the transaction and check for error.
	if err = txn.Commit(); err != nil {
		return err
	}
	return nil
}
