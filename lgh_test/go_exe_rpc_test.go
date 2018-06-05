package lgh

import (
	"fmt"
	"github.com/ethereum/go-ethereum/rpc"
	"testing"
)

func TestName(t *testing.T) {
	tes()
}

func tes() {
	client, err := rpc.Dial("http://localhost:8545")
	if err != nil {
		fmt.Println("rpc.Dial err", err)
		return
	}
	var account[]string
	err = client.Call(&account, "eth_accounts")
	var result string
	//var result hexutil.Big
	err = client.Call(&result, "eth_getBalance", account[0], "latest")
	//err = ec.c.CallContext(ctx, &result, "eth_getBalance", account, "latest")
	if err != nil {
		fmt.Println("client.Call err", err)
		return
	}
	fmt.Printf("account[0]: %s\nbalance[0]: %s\n", account[0], result)
	//fmt.Printf("accounts: %s\n", account[0])
}

