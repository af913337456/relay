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
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/Loopring/relay/types"
)

func TestIntToHex(t *testing.T) {
	var blockNumberForRouteBig *big.Int
	blockNumberForRouteBig = new(big.Int)
	blockNumberForRouteBig.SetString("blockNumber", 0)
	blockNumber := *types.NewBigPtr(blockNumberForRouteBig)
	fmt.Println(blockNumber.BigInt().String())
}

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

func TestFour(t *testing.T) {
	s := big.NewInt(1400)
	b := big.NewInt(4000)
	rac1 := new(big.Rat).SetFrac(s, b)
	fmt.Println(rac1.Num()) // 返回分子 100/4000 = 1/40
	fmt.Println(rac1.Denom()) // 返回分母
	fmt.Println(new(big.Int).Div(rac1.Num(), rac1.Denom())) // div 除法，＜1 的返回0
}

func TestRatAdd(t *testing.T) {
	rat1 := big.NewRat(int64(5), int64(6))
	rat2 := big.NewRat(int64(15), int64(16))
	fmt.Println(rat1)
	rat1.Add(rat1, rat2) // rat.add 是相加，a/b.add(a/b,a1/b1) = a/b + a1/b1
	fmt.Println(rat1)
}

func TestRatSub(t *testing.T) {
	rat1 := big.NewRat(int64(5), int64(6))
	rat2 := big.NewRat(int64(15), int64(16))
	fmt.Println(rat1.Sub(rat1,rat2)) // rat.sub 是相减，a/b.sub(a/b,a1/b1) = a/b - a1/b1

	v := new(big.Rat).SetInt(big.NewInt(21))
	m := new(big.Rat).SetInt(big.NewInt(18))
	v.Quo(m, v) // 效果是 m/v
	fmt.Println(v)

}

func TestQuo(t *testing.T)  {
	/*
	{
        "id": "bitcoin",
        "name": "Bitcoin",
        "symbol": "BTC",
        "rank": "1",
        "price_usd": "7635.3",
        "price_btc": "1.0",
        "24h_volume_usd": "4707920000.0",
        "market_cap_usd": "130396607812",
        "available_supply": "17078125.0",
        "total_supply": "17078125.0",
        "max_supply": "21000000.0",
        "percent_change_1h": "0.15",
        "percent_change_24h": "2.63",
        "percent_change_7d": "1.57",
        "last_updated": "1528269274"
    }
	*/
	// v.Quo(amount, v)
	// v.Mul(price, v)
	p1 := new(big.Rat).SetInt(big.NewInt(549755813888))
	p2 := new(big.Rat).SetInt(big.NewInt(4197852931476685))
	price := p2.Quo(p2,p1)
	fmt.Println(price)

	p3 := new(big.Rat).Set(price)
	fmt.Println("price",p3)

	v := new(big.Rat).SetInt(big.NewInt(1000000000000000000)) // 小数位
	amount := new(big.Rat).SetInt(big.NewInt(2)) // 2 个
	v.Quo(amount, v) // 余额/小数位 * 当前一个的汇率
	v.Mul(price, v)
	fmt.Println(isValueDusted(v))
}

func isValueDusted(value *big.Rat) bool {
	// lgh: dustOrderValue 是最小的标准值，单位基于配置文件中而定，同时它也是配置文件中设置的。目前默认是 1
	minValue := big.NewInt(1) // 1 USD
	minRat := new(big.Rat).SetInt(minValue)
	if value == nil || value.Cmp(minRat) > 0 {
		return false
	}

	return true
}

func TestCmp(t *testing.T) {
	v := new(big.Rat).SetInt(big.NewInt(50))
	d := new(big.Rat).SetInt(big.NewInt(100000000000000))

	fmt.Println(new(big.Rat).Inv(d))

	v.Quo(d,v) // d/v
	fmt.Println(v)
	m := new(big.Rat).SetInt(big.NewInt(1))
	// 21/1  18/1
	fmt.Println(v.Cmp(m))
}

func TestJsonRat(t *testing.T) {
	type CurrencyMarketCap struct {
		PriceUsd     *big.Int       `json:"price_usd"`
	}
	v := new(big.Rat).SetInt(big.NewInt(549755813888))
	d := new(big.Rat).SetInt(big.NewInt(4197852931476685))
	fmt.Println(v.Quo(d,v).Float64())
	caps := CurrencyMarketCap{}
	body := []byte("{\"price_usd\":17}")
	if err := json.Unmarshal(body, &caps); nil != err {
		fmt.Println(err)
	} else {
		fmt.Println(caps)
	}
}

func TestMinValue(t *testing.T) {

}

func TestRingHash(t *testing.T) {
	type Ring struct {
		Hash        common.Hash    `json:"hash"`
	}
	ring := Ring{}
	fmt.Println(ring.Hash.Hex())

	var name *string
	if name == nil {
		fmt.Println("null")
	}
}

func TestQud(t *testing.T) {
	rat1 := big.NewRat(int64(5), int64(6))
	// 5/6 * 1/10 = 1/12
	rate := new(big.Rat).Quo(rat1, new(big.Rat).SetInt(big.NewInt(10)))
	fmt.Println(rate) // 1/12
	lrcFee := new(big.Rat).SetInt(big.NewInt(int64(2)))
	fmt.Println(lrcFee)
}

func TestGas(t *testing.T) {
	gasUsedMap := make(map[int]*big.Int)
	gasUsedMap[2] = big.NewInt(500000)
	gasUsedMap[3] = big.NewInt(500000)
	gasUsedMap[4] = big.NewInt(500000)
	gasUsedWithLength :=  make(map[int]*big.Int)
	gasUsedWithLength = gasUsedMap
	ringStateGas := new(big.Int)
	ringStateGas.Set(gasUsedWithLength[2])
	fmt.Println(ringStateGas)
	protocolCost := new(big.Int)
	protocolCost.Mul(ringStateGas, big.NewInt(1000000000))
	fmt.Println(protocolCost)

	rat1 := big.NewRat(int64(3), int64(6))
	fmt.Println(rat1.FloatString(0))
	s1b0, _ := new(big.Int).SetString(rat1.FloatString(0), 10)
	fmt.Println(s1b0)
}














