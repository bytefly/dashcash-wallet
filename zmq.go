package main

import (
	"github.com/btcsuite/btcd/wire"
	conf "github.com/bytefly/dashcash-wallet/config"
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
	subSocket.SetSubscribe("rawtx")
	subSocket.SetSubscribe("hashblock")

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

func zmqProcess(config *conf.Config, chainName string, ch chan<- ObjMessage) error {
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
			cmd, err := subSocket.RecvBytes(0)
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

				msg, err := subSocket.RecvBytes(0)
				if err != nil {
					log.Println("zmq recv err:", err)
					break
				}
				if len(msg) != 4 {
					if err == nil {
						switch string(cmd) {
						case "rawtx":
							var tx wire.MsgTx
							rbuf := bytes.NewReader(msg)
							err = tx.Deserialize(rbuf)
							if err == nil {
								ParseMempoolTransaction(config, &tx, chainName)
							}
						case "hashblock":
							GetNewerBlock(config, ch)
						}
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
