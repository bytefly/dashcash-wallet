package util

import (
	"fmt"
	"testing"
)

func TestLeftShift(t *testing.T) {
	fmt.Println("LeftShift(25893180161173005034, 18)=", LeftShift("25893180161173005034", 18))
	fmt.Println("LeftShift(25893180161173005034, 3)=", LeftShift("25893180161173005034", 3))
	fmt.Println("LeftShift(100998995000000000000126, 3)=", LeftShift("100998995000000000000126", 3))
	fmt.Println("LeftShift(100998995000000000000126, 18)=", LeftShift("100998995000000000000126", 18))
	fmt.Println("LeftShift(10099899500000000, 18)=", LeftShift("10099899500000000", 18))
	fmt.Println("LeftShift(0, 18)=", LeftShift("0", 18))
	fmt.Println("LeftShift(12343434.55555, 8)=", LeftShift("12343434.55555", 8))
	fmt.Println("LeftShift(12343434.55555, 10)=", LeftShift("12343434.55555", 10))
	fmt.Println("LeftShift(12343434.55555, 4)=", LeftShift("12343434.55555", 4))
}

func TestRightShift(t *testing.T) {
	fmt.Println("RightShift(0.00000000000005, 8)=", RightShift("0.00000000000005", 8))
	fmt.Println("RightShift(0.005, 18)=", RightShift("0.005", 18))
	fmt.Println("RightShift(0.005983903883, 8)=", RightShift("0.005983903883", 8))
	fmt.Println("RightShift(1111333.0059, 8)=", RightShift("1111333.0059", 8))
	fmt.Println("RightShift(1111, 8)=", RightShift("1111", 8))
}

func TestEncode(t *testing.T) {
	cases := make(map[string]string)
	cases["1b1itzeSKYEKhdcthUSnNJ47Fx2U8Zwwn"] = "bitcoincash:qqrxa0h9jqnc7v4wmj9ysetsp3y7w9l36u8gnnjulq"
	cases["3KTuim99rePpvzNRPBMuYb4ZCTWthpY1N9"] = "bitcoincash:prp00td3tddhwrxp89x34p8lxc7p8eyepg2v8a3tml"
	for k, v := range cases {
		encoded, _ := ConvertLegacyToCashAddr(k, &BCHMainNetParams)
		if encoded != v {
			fmt.Println(encoded, "!=", v)
		}
	}
}

func TestDecode(t *testing.T) {
	cases := make(map[string]string)
	cases["bitcoincash:qqrxa0h9jqnc7v4wmj9ysetsp3y7w9l36u8gnnjulq"] = "1b1itzeSKYEKhdcthUSnNJ47Fx2U8Zwwn"
	cases["bitcoincash:prp00td3tddhwrxp89x34p8lxc7p8eyepg2v8a3tml"] = "3KTuim99rePpvzNRPBMuYb4ZCTWthpY1N9"
	for k, v := range cases {
		legacyAddr, err := ConvertCashAddrToLegacy(k, &BCHMainNetParams)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		if legacyAddr != v {
			fmt.Println(legacyAddr, "!=", v)
		}
	}
}
