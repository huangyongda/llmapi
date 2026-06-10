package handlers

import (
	"fmt"
	"strconv"
	"testing"
)

func TestNumber(t *testing.T) {
	TotalTokens := 163699
	decNum := int(float64(TotalTokens)/50000.0 + 0.9999)
	fmt.Println(decNum)
	if decNum > 1 {
		test := strconv.Itoa(decNum - 1)
		fmt.Println(test)
	}
}

func TestCountTokens(t *testing.T) {

	str := "dddd"
	total := CountTokens(str)

	fmt.Println(total)

}
