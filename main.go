package main

import (
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	conf "github.com/bytefly/dashcash-wallet/config"
	"github.com/bytefly/dashcash-wallet/util"
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

	buildVer  = false
	commitID  string
	buildTime string
)

func init() {
	flag.BoolVar(&fDebug, "debug", false, "Debug")
	flag.StringVar(&fConfigFile, "cfg", "config.ini", "Configuration file")
	flag.BoolVar(&buildVer, "version", false, "print build version and then exit")
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var last_id uint64

	flag.Parse()

	fmt.Printf("Build: %s\tLastCommit: %s\n", buildTime, commitID[:7])
	if buildVer {
		os.Exit(0)
	}

	config, err := conf.LoadConfiguration(fConfigFile)
	if err != nil {
		log.Println("load cfg err:", err)
		return
	}

	param := util.GetParamByName(config.ChainName)
	if param == nil {
		log.Println("unsupport chain!!!")
		return
	}

	client, err := ConnectRPC(config)
	if err != nil {
		log.Println("connect to RPC server err:", err)
		return
	}

	last_id = config.LastBlock

	util.AddressInit(config.Xpub, 0, int(config.Index), param)
	util.AddressInit(config.Xpub, 1, int(config.InIndex), param)

	err = openDb(config.ChainName)
	if err != nil {
		log.Println("open db err:", err)
		return
	}

	r := mux.NewRouter()
	r.HandleFunc("/getAddress", GetAddrHandler(config))
	r.HandleFunc("/sendCoin", SendCoinHandler(config))
	r.HandleFunc("/getBalance", GetBalanceHandler(config))
	r.HandleFunc("/prepareTrezorSign", PrepareTrezorSignHandler(config))
	r.HandleFunc("/sendSignedTx", SendSignedTxHandler(config))
	r.HandleFunc("/getInnerBalance", GetInnerBalanceHandler(config))
	r.HandleFunc("/sendOmniCoin", SendOmniCoinHandler(config))

	r.HandleFunc("/dumpUtxo", DumpUtxoHandler(config))

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

	zmqInit(config.ZmqURL)

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

		err = zmqProcess(client, config.ChainName)
		if err != nil {
			zmqRestart(config.ZmqURL)
		}
	}

	zmqClose(config.ZmqURL)
	server.Close()
	closeDb()
	client.Shutdown()
	conf.SaveConfiguration(config, fConfigFile)
	log.Println("bye")
}
