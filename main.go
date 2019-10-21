package main

import (
	"flag"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

const (
	NOTIFY_TYPE_NONE = iota
	NOTIFY_TYPE_TX
	NOTIFY_TYPE_ADMIN
)

type NotifyMessage struct {
	MessageType int
	Address     string
	Amount      *big.Int
	TxHash      string
	Fee         *big.Int
	Coin        string
	TxType      int //0: deposit, 1: withdraw
}

var (
	fDebug      bool
	fConfigFile string
)

func init() {
	flag.BoolVar(&fDebug, "debug", false, "Debug")
	flag.StringVar(&fConfigFile, "cfg", "config.ini", "Configuration file")
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var last_id uint64

	flag.Parse()

	config, err := LoadConfiguration(fConfigFile)
	if err != nil {
		panic(err)
	}

	last_id = config.LastBlock

	AddressInit(config.Xpub, config.AccountId, int(config.Index), config.TestNet)
	log.Println(config.Index, " address init ok")

	r := mux.NewRouter()
	r.HandleFunc("/getAddress", GetAddrHandler(config))
	r.HandleFunc("/sendCoin", SendCoinHandler(config))

	r.NotFoundHandler = http.HandlerFunc(NotFoundHandler)
	log.Println("last block: ", last_id)

	ch := make(chan NotifyMessage, 1024)
	go Notifier(config, ch)
	go Subscriber(config, ch, last_id)

	host := ":" + strconv.FormatInt(int64(config.Port), 10)
	log.Printf("Starting web server at %s ...\n", host)

	server := &http.Server{
		ReadTimeout:  time.Duration(2000) * time.Millisecond,
		WriteTimeout: time.Duration(2000) * time.Millisecond,
		Handler:      r,
	}

	var listener net.Listener
	if listener, err = net.Listen("tcp", host); err != nil {
		return
	}
	go server.Serve(listener)

	stop := 0
	for {
		select {
		case <-ticker.C:
		case <-interrupt:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			// stop all modules
			//http.StopHttpService(serviceObj)
			stop = 1
			break
		}

		if stop == 1 {
			break
		}
	}
	server.Close()
	SaveConfiguration(config, fConfigFile)
	log.Println("bye")
}
