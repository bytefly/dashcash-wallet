// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/base58"
	"strings"
)

const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var gen = []int{0x98f2bc8e61, 0x79b76d99e2, 0xf33e5fb3c4, 0xae2eabe2a8, 0x1e4f43e470}

// Decode decodes a bech32 encoded string, returning the human-readable
// part and the data part excluding the checksum.
func Decode(bech string) (string, []byte, error) {
	// The maximum allowed length for a bech32 string is 90. It must also
	// be at least 10 characters, since it needs a non-empty HRP, a
	// separator, and a 8 character checksum.
	if len(bech) < 10 || len(bech) > 90 {
		return "", nil, fmt.Errorf("invalid bech32 string length %d",
			len(bech))
	}
	// Only	ASCII characters between 33 and 126 are allowed.
	for i := 0; i < len(bech); i++ {
		if bech[i] < 33 || bech[i] > 126 {
			return "", nil, fmt.Errorf("invalid character in "+
				"string: '%c'", bech[i])
		}
	}

	// The characters must be either all lowercase or all uppercase.
	lower := strings.ToLower(bech)
	upper := strings.ToUpper(bech)
	if bech != lower && bech != upper {
		return "", nil, fmt.Errorf("string not all lowercase or all " +
			"uppercase")
	}

	// We'll work with the lowercase string from now on.
	bech = lower

	// The string is invalid if the last ':' is non-existent, it is the
	// first character of the string (no human-readable part) or one of the
	// last 8 characters of the string (since checksum cannot contain ':'),
	// or if the string is more than 90 characters in total.
	one := strings.LastIndexByte(bech, ':')
	if one < 1 || one+9 > len(bech) {
		return "", nil, fmt.Errorf("invalid index of :")
	}

	// The human-readable part is everything before the last '1'.
	hrp := bech[:one]
	data := bech[one+1:]

	// Each character corresponds to the byte with value of the index in
	// 'charset'.
	decoded, err := toBytes(data)
	if err != nil {
		return "", nil, fmt.Errorf("failed converting data to bytes: "+
			"%v", err)
	}

	if !bech32VerifyChecksum(hrp, decoded) {
		moreInfo := ""
		checksum := bech[len(bech)-8:]
		expected, err := toChars(bech32Checksum(hrp,
			decoded[:len(decoded)-8]))
		if err == nil {
			moreInfo = fmt.Sprintf("Expected %v, got %v.",
				expected, checksum)
		}
		return "", nil, fmt.Errorf("checksum failed. " + moreInfo)
	}

	// We exclude the last 8 bytes, which is the checksum.
	return hrp, decoded[:len(decoded)-8], nil
}

// Encode encodes a byte slice into a bech32 string with the
// human-readable part hrb. Note that the bytes must each encode 5 bits
// (base32).
func Encode(hrp string, data []byte) (string, error) {
	// Calculate the checksum of the data and append it at the end.
	checksum := bech32Checksum(hrp, data)
	combined := append(data, checksum...)

	// The resulting bech32 string is the concatenation of the hrp, the
	// separator 1, data and checksum. Everything after the separator is
	// represented using the specified charset.
	dataChars, err := toChars(combined)
	if err != nil {
		return "", fmt.Errorf("unable to convert data bytes to chars: "+
			"%v", err)
	}
	return hrp + ":" + dataChars, nil
}

// toBytes converts each character in the string 'chars' to the value of the
// index of the correspoding character in 'charset'.
func toBytes(chars string) ([]byte, error) {
	decoded := make([]byte, 0, len(chars))
	for i := 0; i < len(chars); i++ {
		index := strings.IndexByte(charset, chars[i])
		if index < 0 {
			return nil, fmt.Errorf("invalid character not part of "+
				"charset: %v", chars[i])
		}
		decoded = append(decoded, byte(index))
	}
	return decoded, nil
}

// toChars converts the byte slice 'data' to a string where each byte in 'data'
// encodes the index of a character in 'charset'.
func toChars(data []byte) (string, error) {
	result := make([]byte, 0, len(data))
	for _, b := range data {
		if int(b) >= len(charset) {
			return "", fmt.Errorf("invalid data byte: %v", b)
		}
		result = append(result, charset[b])
	}
	return string(result), nil
}

// ConvertBits converts a byte slice where each byte is encoding fromBits bits,
// to a byte slice where each byte is encoding toBits bits.
func ConvertBits(data []byte, fromBits, toBits uint8, pad bool) ([]byte, error) {
	if fromBits < 1 || fromBits > 8 || toBits < 1 || toBits > 8 {
		return nil, fmt.Errorf("only bit groups between 1 and 8 allowed")
	}

	// The final bytes, each byte encoding toBits bits.
	var regrouped []byte

	// Keep track of the next byte we create and how many bits we have
	// added to it out of the toBits goal.
	nextByte := byte(0)
	filledBits := uint8(0)

	for _, b := range data {

		// Discard unused bits.
		b = b << (8 - fromBits)

		// How many bits remaining to extract from the input data.
		remFromBits := fromBits
		for remFromBits > 0 {
			// How many bits remaining to be added to the next byte.
			remToBits := toBits - filledBits

			// The number of bytes to next extract is the minimum of
			// remFromBits and remToBits.
			toExtract := remFromBits
			if remToBits < toExtract {
				toExtract = remToBits
			}

			// Add the next bits to nextByte, shifting the already
			// added bits to the left.
			nextByte = (nextByte << toExtract) | (b >> (8 - toExtract))

			// Discard the bits we just extracted and get ready for
			// next iteration.
			b = b << toExtract
			remFromBits -= toExtract
			filledBits += toExtract

			// If the nextByte is completely filled, we add it to
			// our regrouped bytes and start on the next byte.
			if filledBits == toBits {
				regrouped = append(regrouped, nextByte)
				filledBits = 0
				nextByte = 0
			}
		}
	}

	// We pad any unfinished group if specified.
	if pad && filledBits > 0 {
		nextByte = nextByte << (toBits - filledBits)
		regrouped = append(regrouped, nextByte)
		filledBits = 0
		nextByte = 0
	}

	// Any incomplete group must be <= 4 bits, and all zeroes.
	if filledBits > 0 && (filledBits > 4 || nextByte != 0) {
		return nil, fmt.Errorf("invalid incomplete group")
	}

	return regrouped, nil
}

// For more details on the checksum calculation, please refer to BIP 173.
func bech32Checksum(hrp string, data []byte) []byte {
	// Convert the bytes to list of integers, as this is needed for the
	// checksum calculation.
	integers := make([]int, len(data))
	for i, b := range data {
		integers[i] = int(b)
	}
	values := append(bech32HrpExpand(hrp), integers...)
	values = append(values, []int{0, 0, 0, 0, 0, 0, 0, 0}...)
	polymod := bech32Polymod(values) ^ 1
	var res []byte
	for i := 0; i < 8; i++ {
		res = append(res, byte((polymod>>uint(5*(7-i)))&31))
	}
	return res
}

// For more details on the polymod calculation, please refer to BIP 173.
func bech32Polymod(values []int) int {
	chk := 1
	for _, v := range values {
		b := chk >> 35
		chk = (chk&0x07ffffffff)<<5 ^ v
		for i := 0; i < 5; i++ {
			//if (b>>uint(i))&1 == 1 {
			if (b & (1 << i)) != 0 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

// For more details on HRP expansion, please refer to BIP 173.
func bech32HrpExpand(hrp string) []int {
	v := make([]int, 0, len(hrp)+1)
	for i := 0; i < len(hrp); i++ {
		v = append(v, int(hrp[i]&31))
	}
	v = append(v, 0)
	return v
}

// For more details on the checksum verification, please refer to BIP 173.
func bech32VerifyChecksum(hrp string, data []byte) bool {
	integers := make([]int, len(data))
	for i, b := range data {
		integers[i] = int(b)
	}
	concat := append(bech32HrpExpand(hrp), integers...)
	return bech32Polymod(concat) == 1
}

func ConvertLegacyToCashAddr(legacyAddr string, param *chaincfg.Params) (string, error) {
	var (
		pkHash  []byte
		version byte
		isP2sh  bool
	)

	addr, _ := btcutil.DecodeAddress(legacyAddr, param)
	switch addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash, *btcutil.AddressWitnessScriptHash, *btcutil.AddressPubKey:
		return "", errors.New("not a legacy address")
	case *btcutil.AddressPubKeyHash:
		pkHash = addr.ScriptAddress()
	case *btcutil.AddressScriptHash:
		pkHash = addr.ScriptAddress()
		isP2sh = true
	}

	if len(pkHash) == 20 {
		if isP2sh {
			version = (1 << 3) | 0
		} else {
			version = (0 << 3) | 0
		}
	} else {
		return "", errors.New("unsupport hash size")
	}

	data := make([]byte, 1)
	data[0] = version
	data = append(data, pkHash...)

	// Convert data to base32:
	conv, _ := ConvertBits(data, 8, 5, true)
	encoded, _ := Encode(param.Bech32HRPSegwit, conv)
	return encoded, nil
}

func ConvertCashAddrToLegacy(cashAddr string, param *chaincfg.Params) (string, error) {
	buf := new(bytes.Buffer)
	hrp, decoded, err := Decode(cashAddr)
	if err != nil {
		return "", err
	}

	conv, _ := ConvertBits(decoded, 5, 8, false)

	if hrp != param.Bech32HRPSegwit || len(conv) != 21 {
		return "", errors.New("invalid cash address")
	}

	if conv[0] == 0 {
		binary.Write(buf, binary.BigEndian, param.PubKeyHashAddrID)
	} else if conv[0] == 8 {
		binary.Write(buf, binary.BigEndian, param.ScriptHashAddrID)
	} else {
		return "", errors.New("invalid version byte")
	}
	payload := make([]byte, 0)
	payload = append(payload, buf.Bytes()...)
	payload = append(payload, conv[1:]...)

	h := sha256.Sum256(payload)
	h = sha256.Sum256(h[:])

	payload = append(payload, h[0:4]...)
	addr := base58.Encode(payload)
	return addr, nil
}
