package main

import (
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	zmq "github.com/pebbe/zmq4"

	"bytes"
	"errors"
	"log"
	"time"
)

var (
	subSocket *zmq.Socket
	poller    *zmq.Poller
)

func zmqInit(url string) (err error) {
	subSocket, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		log.Println("new zmq subscript err:", err)
		return
	}
	err = subSocket.SetSubscribe("rawtx")
	if err != nil {
		log.Println("set zmq subscribe err:", err)
		return
	}

	err = subSocket.Connect(url)
	if err != nil {
		log.Println("zmq connect err:", err)
		return
	}

	poller = zmq.NewPoller()
	poller.Add(subSocket, zmq.POLLIN)
	log.Println("zmq init ok")
	return
}

func zmqProcess(client *rpcclient.Client, chainName string) error {
	if poller == nil || subSocket == nil {
		return errors.New("zmq not init yet")
	}

	sockets, err := poller.Poll(100 * time.Millisecond)
	if err != nil {
		log.Println("Poll err:", err)
		return err //  Interrupted
	}

	for _, s := range sockets {
		if s.Socket == subSocket {
			msg, err := subSocket.RecvBytes(0)
			if err != nil {
				log.Println("zmq recv err:", err)
				break
			}

			for {
				more, err := subSocket.GetRcvmore()
				if err != nil {
					log.Println("zmq rcvmore err:", err)
					break
				}
				if !more {
					break
				}

				msg, err = subSocket.RecvBytes(0)
				if err != nil {
					log.Println("zmq recv err:", err)
					break
				}
				if len(msg) != 4 {
					var tx wire.MsgTx
					rbuf := bytes.NewReader(msg)
					err = tx.Deserialize(rbuf)
					if err == nil {
						ParseMempoolTransaction(client, &tx, chainName)
					}
				} else { //4 bytes nSequence
					//log.Println(binary.LittleEndian.Uint32(msg))
					break
				}
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func zmqClose(url string) {
	err := subSocket.Disconnect(url)
	if err != nil {
		log.Println("zmq disconnect err:", err)
	}

	subSocket.SetLinger(0)
	subSocket.Close()
}

func zmqRestart(url string) {
	zmqClose(url)
	zmqInit(url)
}
