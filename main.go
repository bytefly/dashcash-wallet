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
	TxType      int
	BlockTime   uint64
}

var (
	fDebug      bool
	fConfigFile string

	buildVer  = false
	commitID  string
	buildTime string

	addUtxo bool
	rmUtxo  bool
	hash    string
	index   int
	addr    string
	value   int64
)

func init() {
	flag.BoolVar(&fDebug, "debug", false, "Debug")
	flag.StringVar(&fConfigFile, "cfg", "config.ini", "Configuration file")
	flag.BoolVar(&buildVer, "version", false, "print build version and then exit")

	flag.BoolVar(&addUtxo, "addUtxo", false, "add a utxo to db")
	flag.BoolVar(&rmUtxo, "rmUtxo", false, "remove a utxo from db")
	flag.StringVar(&hash, "hash", "demo", "tx hash")
	flag.IntVar(&index, "index", 0, "tx output index")
	flag.StringVar(&addr, "addr", "", "tx output address")
	flag.Int64Var(&value, "value", 0, "tx output amount")
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	ticker := time.NewTicker(time.Second)
	newBlockTicker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	defer newBlockTicker.Stop()

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

	last_id = config.LastBlock

	util.AddressInit(config.Xpub, 0, int(config.Index), param)
	util.AddressInit(config.Xpub, 1, int(config.InIndex), param)

	err = openDb(config.DBDir)
	if err != nil {
		log.Println("open db err:", err)
		return
	}

	if addUtxo {
		createUtxo(hash, uint32(index), addr, value)
		return
	}
	if rmUtxo {
		removeUtxo(hash, uint32(index), "", 0)
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
	r.HandleFunc("/prepareOmniTrezorSign", PrepareOmniTrezorSignHandler(config))
	r.HandleFunc("/getOmniBalance", GetOmniBalanceHandler(config))

	r.HandleFunc("/dumpUtxo", DumpUtxoHandler(config))

	r.NotFoundHandler = http.HandlerFunc(NotFoundHandler)
	log.Println("last block: ", last_id)

	ch1 := make(chan NotifyMessage, 1024)
	ch2 := make(chan ObjMessage, 1024)
	go Notifier(config, ch1)
	go Listener(config, ch2, ch1, last_id)

	host := ":" + strconv.FormatInt(int64(config.Port), 10)
	log.Printf("Starting web server at %s ...\n", host)

	server := &http.Server{
		ReadTimeout:  time.Duration(30) * time.Second,
		WriteTimeout: time.Duration(30) * time.Second,
		Handler:      r,
	}

	var listener net.Listener
	if listener, err = net.Listen("tcp", host); err != nil {
		return
	}
	go server.Serve(listener)

	zmqInit(config.ZmqURL)
	//launch the signal once avoiding waiting for a long time
	GetNewerBlock(config, ch2)

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
		case <-newBlockTicker.C:
			conf.SaveConfiguration(config, fConfigFile)
			GetNewerBlock(config, ch2)
		}

		if stop == 1 {
			break
		}

		err = zmqProcess(config, config.ChainName, ch2)
		if err != nil {
			zmqRestart(config.ZmqURL)
		}
	}

	zmqClose(config.ZmqURL)
	server.Close()
	closeDb()
	conf.SaveConfiguration(config, fConfigFile)
	log.Println("bye")
}
