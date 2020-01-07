package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/base58"
	"github.com/btcsuite/btcutil/hdkeychain"
	conf "github.com/bytefly/dashcash-wallet/config"
	"golang.org/x/crypto/ripemd160"
	"log"
	"strings"
	"sync"
)

var addrs sync.Map

func AddressInit(xpub string, branch uint32, total int, param *chaincfg.Params) {
	masterKey, err := hdkeychain.NewKeyFromString(xpub)
	if err != nil {
		log.Println(err)
		return
	}

	acct, err := masterKey.Child(branch)
	if err != nil {
		log.Println(err)
		return
	}

	for i := 0; i < total; i++ {
		acctExt, err := acct.Child(uint32(i))
		if err != nil {
			log.Println(err)
			return
		}

		pubkey, _ := acctExt.ECPubKey()
		addr := getAddrByPubKey(pubkey.SerializeCompressed(), param)
		if strings.HasPrefix(strings.ToLower(param.Name), "bch") {
			addr, _ = ConvertLegacyToCashAddr(addr, param)
			addr = addr[len(param.Bech32HRPSegwit)+1:]
		}
		addrs.Store(addr, fmt.Sprintf("%d/%d", branch, i))
		if branch == 1 && i == 0 {
			log.Println("inner withdraw addr:", addr)
		}
	}
}

func getAddrByPubKey(pubKeyBytes []byte, param *chaincfg.Params) string {
	data := hash160(pubKeyBytes)
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, param.PubKeyHashAddrID)
	payload := make([]byte, 0)
	payload = append(payload, buf.Bytes()...)
	payload = append(payload, data...)

	h := sha256.Sum256(payload)
	h = sha256.Sum256(h[:])

	payload = append(payload, h[0:4]...)
	addr := base58.Encode(payload)

	return addr
}

// hash160 returns the RIPEMD160 hash of the SHA-256 HASH of the given data.
func hash160(data []byte) []byte {
	h := sha256.Sum256(data)
	return ripemd160h(h[:])
}

// ripemd160h returns the RIPEMD160 hash of the given data.
func ripemd160h(data []byte) []byte {
	h := ripemd160.New()
	h.Write(data)
	return h.Sum(nil)
}

func GetNewExternalAddr(config *conf.Config, index uint32) (addr string, err error) {
	addr, err = getNewAddrByBranch(config, 0, index)
	return
}

func GetNewChangeAddr(config *conf.Config, index uint32) (addr string, err error) {
	addr, err = getNewAddrByBranch(config, 1, index)
	return
}

func getNewAddrByBranch(config *conf.Config, branch, index uint32) (addr string, err error) {
	masterKey, err := hdkeychain.NewKeyFromString(config.Xpub)
	if err != nil {
		log.Println(err)
		return
	}

	acct, err := masterKey.Child(branch)
	if err != nil {
		log.Println(err)
		return
	}

	acctExt, err := acct.Child(index)
	if err != nil {
		log.Println(err)
		return
	}

	pubkey, _ := acctExt.ECPubKey()
	param := GetParamByName(config.ChainName)
	addr = getAddrByPubKey(pubkey.SerializeCompressed(), param)
	addrs.Store(addr, fmt.Sprintf("%d/%d", branch, index))
	return
}

func GetPrivateKey(xpriv string, branch int, index int) (privKey *btcec.PrivateKey, err error) {
	masterKey, err := hdkeychain.NewKeyFromString(xpriv)
	if err != nil {
		log.Println(err)
		return
	}

	acctExt, err := masterKey.Child(uint32(index))
	if err != nil {
		log.Println(err)
		return
	}

	privKey, err = acctExt.ECPrivKey()
	return
}

func LeftShift(str string, size int) string {
	if str == "" || size <= 0 {
		return str
	}

	index := strings.IndexByte(str, '.')
	//for float type
	if index >= 0 {
		//drop dot(.)
		raw := []byte(str[:index])
		raw = append(raw, str[index+1:]...)
		if index > size { //move dot
			return str[:index-size] + "." + string(raw[index-size:])
		} else { //pad with 0.0s in prefix
			bytes := []byte("0.")
			for i := 0; i < size-index; i++ {
				bytes = append(bytes, '0')
			}
			bytes = append(bytes, raw...)
			return string(bytes)
		}
	}

	//for int
	if len(str) <= size {
		bytes := []byte("0.")
		for i := 0; i < size-len(str); i++ {
			bytes = append(bytes, '0')
		}
		bytes = append(bytes, []byte(str)...)
		return string(bytes)
	}

	return str[:len(str)-size] + "." + str[len(str)-size:]
}

func RightShift(str string, size int) string {
	if str == "" || size <= 0 {
		return str
	}

	index := strings.IndexByte(str, '.')
	//for int
	if index == -1 {
		bytes := []byte(str)
		for i := 0; i < size; i++ {
			bytes = append(bytes, '0')
		}
		return string(bytes)
	}

	//drop dot(.)
	bytes := []byte(str[:index])
	bytes = append(bytes, str[index+1:]...)
	if index+size >= len(str)-1 {
		for i := 0; i < index+size-len(str)+1; i++ {
			bytes = append(bytes, '0')
		}
	} else {
		bytes = append(bytes[:index+size], append([]byte("."), bytes[index+size:]...)...)
	}

	//trim all 0s in the head
	stop := -1
	for i := 0; i < len(bytes); i++ {
		if bytes[i] != '0' {
			stop = i
			break
		}
	}
	if stop >= 0 {
		if bytes[stop] == '.' {
			stop -= 1
		}
		bytes = bytes[stop:]
	}

	return string(bytes)
}

func VerifyAddress(chain, address string) bool {
	if strings.ToLower(chain) == "bch" {
		_, err := ConvertCashAddrToLegacy(address, &BCHMainNetParams)
		if err != nil {
			return false
		}
		return true
	}

	param := GetParamByName(chain)
	addr, err := btcutil.DecodeAddress(address, param)
	if err != nil {
		return false
	}

	if false == addr.IsForNet(param) {
		log.Println("address not from mainnet")
		return false
	}

	return true
}

func IsNativeSegWitAddress(chain, address string) bool {
	param := GetParamByName(chain)
	addr, err := btcutil.DecodeAddress(address, param)
	if err != nil {
		return false
	}

	if false == addr.IsForNet(param) {
		log.Println("address not from mainnet")
		return false
	}

	switch addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash, *btcutil.AddressWitnessScriptHash:
		return true
	default:
		return false
	}
}

func LoadAddrPath(addr string) (string, bool) {
	path, ok := addrs.Load(addr)
	if !ok {
		return "", ok
	}
	return path.(string), ok
}

func StoreAddrPath(addr, path string) {
	addrs.Store(addr, path)
}
