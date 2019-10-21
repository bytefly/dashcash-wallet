package main

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
