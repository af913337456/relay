package lgh

import (
	"testing"
	"github.com/ethereum/go-ethereum/common"
	"fmt"
)

/**

作者(Author): 林冠宏 / 指尖下的幽灵

Created on : 2018/6/2

*/

func Test222(t *testing.T) {
	addressByte := common.HexToAddress("0x456044789a41b277f033e4d79fab2139d69cd154")
	fmt.Println(addressByte.String())
}

func TestEchoAccess(t *testing.T) {
	
}