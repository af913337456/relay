package lgh

import (
	"testing"
	"math/big"
	"fmt"
	"github.com/Loopring/relay/market/util"
	"github.com/Loopring/relay/dao"
	"github.com/Loopring/relay/txmanager"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/cache"
	"time"
)

func TestMul(t *testing.T) {
	s := big.NewInt(300)
	b := big.NewInt(4000)

	b1 := big.NewInt(18)
	b2 := big.NewInt(12)
	// 300  个，300*10^18
	// 4000 个，4000*10^12
	rac1 := new(big.Rat).SetFrac(s, b)
	rac2 := new(big.Rat).SetFrac(b1, b2)

	fmt.Println("rac1",rac1)
	fmt.Println("rac2",rac2)
	// price 充当的应该是实际的比例
	price := new(big.Rat).Mul(
		rac1,
		rac2,
	)
	fmt.Println(price)
}

func TestWrapMarket(t *testing.T) {
	c := config.LoadConfig("../config/relay.toml")
	log.Initialize(c.Log)
	rds := dao.NewRdsService(c.Mysql)
	txmanager.NewTxView(rds)
	cache.NewCache(c.Redis)
	util.Initialize(c.Market)
	ta := "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
	tb := "0xEF68e7C694F40c8202821eDF525dE3782458639f"
	market, err := util.WrapMarketByAddress(ta, tb)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(market)
}

func TestThree(t *testing.T) {
	// 纳秒时间戳 / 1e6 = 毫秒时间戳
	fmt.Println(big.NewInt(time.Now().UnixNano() / 1e6))
	fmt.Println(big.NewInt(time.Now().Unix()*100))
	// 1528102161706611596
	// 1528102231806
}





