package lgh_test

import (
	"testing"
	"github.com/ethereum/go-ethereum/common"
	"github.com/Loopring/relay/types"
	"github.com/Loopring/relay/market/util"
	"fmt"
	"encoding/json"
	"github.com/Loopring/relay/ethaccessor"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/txmanager"
	"github.com/Loopring/relay/cache"
	"github.com/Loopring/relay/dao"
)
var (
	SupportTokens  map[string]types.Token // token symbol to entity
	AllTokens      map[string]types.Token
	SupportMarkets map[string]types.Token // token symbol to contract hex address
	AllMarkets     []string
	AllTokenPairs  []util.TokenPair
	SymbolTokenMap map[common.Address]string
)

func TestName(t *testing.T) {

	SupportTokens := make(map[string]types.Token)
	SupportMarkets := make(map[string]types.Token)
	AllTokens := make(map[string]types.Token)
	SymbolTokenMap := make(map[common.Address]string)

	// lgh: 设置从 json 文件导入代币信息，和市场
	SupportTokens, SupportMarkets, AllTokens, AllMarkets, AllTokenPairs, SymbolTokenMap =
		util.GetTokenAndMarketFromDB_test("tokens.json")

	fmt.Println("SupportTokens:",len(SupportTokens),
		"SupportMarkets:",len(SupportMarkets),"AllTokens:",len(AllTokens),
			"AllMarkets:",len(AllMarkets),"SymbolTokenMap:", len(SymbolTokenMap),"AllTokenPairs:",len(AllTokenPairs))
	fmt.Println("===**SupportTokens**===")
	printTokens(SupportTokens)
	fmt.Println("===**SupportMarkets**===")
	printTokens(SupportMarkets)
	fmt.Println("===**AllTokens**===")
	printTokens(AllTokens)
	fmt.Println("===**AllMarkets**===")
	for _,v := range AllMarkets {
		fmt.Println(v)
		fmt.Println("=====")
	}
	fmt.Println("===**SymbolTokenMap**===")
	for k,v := range SymbolTokenMap {
		fmt.Println(k.String())
		fmt.Println(v)
	}
	fmt.Println("=====AllTokenPairs=====")
	for _,v := range AllTokenPairs {
		fmt.Println("sell: "+SymbolTokenMap[v.TokenS]+" ---> "+"buy: "+SymbolTokenMap[v.TokenB])
		fmt.Println("sell: "+v.TokenS.String()+" ---> "+"buy: "+v.TokenB.String())
		fmt.Println("---")
	}
	fmt.Println("==========")
}

func printTokens(target map[string]types.Token)  {
	for k,v := range target {
		fmt.Println(k)
		content,_ := json.Marshal(v)
		fmt.Println(string(content))
	}
	fmt.Println("==========")
}

func TestAA(t *testing.T) {
	c := config.LoadConfig("../config/relay.toml")
	log.Initialize(c.Log)
	rds := dao.NewRdsService(c.Mysql)
	txmanager.NewTxView(rds)
	cache.NewCache(c.Redis)
	ethaccessor.Initialize(c.Accessor, c.Common, AllTokens["WETH"].Protocol)
}




















