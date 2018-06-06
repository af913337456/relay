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
	"io/ioutil"
	"net/http"
	"math/big"
)
var (
	SupportTokens  map[string]types.Token // token symbol to entity
	AllTokens      map[string]types.Token
	SupportMarkets map[string]types.Token // token symbol to contract hex address
	AllMarkets     []string
	AllTokenPairs  []util.TokenPair
	SymbolTokenMap map[common.Address]string
)

func TestName2(t *testing.T) {

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
		fmt.Println("sell: TokenS "+SymbolTokenMap[v.TokenS]+" ---> "+"buy: TokenB "+SymbolTokenMap[v.TokenB])
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

func TestMinValue(t *testing.T) {
	c := config.LoadConfig("../config/relay.toml")
	log.Initialize(c.Log)
	minValue := big.NewInt(c.OrderManager.DustOrderValue) // 1 USD
	minRat := new(big.Rat).SetInt(minValue)
	fmt.Println(minRat)
}

func TestSyncMarket(t *testing.T) {
	url := fmt.Sprintf("https://api.coinmarketcap.com/v1/ticker/?limit=0&convert=%s", "USD")
	resp, err := http.Get(url) // lgh: 进行第三方货币数据接口请求，下面再解析然后初始化好所有的货币信息，含价格等信息
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if nil != resp && nil != resp.Body {
			resp.Body.Close()
		}
	}()
	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
	if nil != err {
		fmt.Println(err)
		return
	} else {
		var caps []*types.CurrencyMarketCap
		if err := json.Unmarshal(body, &caps); nil != err {
			fmt.Println(err)
			return
		} else {
			for _, tokenCap := range caps {
				// "price_usd":"123627328208529/8796093022208"
				//data,_ := json.Marshal(tokenCap)
				fmt.Println("price_usd",tokenCap.PriceUsd)
			}
		}
	}
}


















