/*

  Copyright 2017 Loopring Project Ltd (Loopring Foundation).

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.

*/

package types

import (
	"github.com/Loopring/relay/crypto"
	"github.com/Loopring/relay/log"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"time"
)

type OrderStatus uint8

const (
	ORDER_UNKNOWN  OrderStatus = 0
	ORDER_NEW      OrderStatus = 1
	ORDER_PARTIAL  OrderStatus = 2
	ORDER_FINISHED OrderStatus = 3
	ORDER_CANCEL   OrderStatus = 4
	ORDER_CUTOFF   OrderStatus = 5
	ORDER_EXPIRE   OrderStatus = 6
	ORDER_PENDING  OrderStatus = 7
	ORDER_PENDING_FOR_P2P  OrderStatus = 17
	//ORDER_BALANCE_INSUFFICIENT   OrderStatus = 7
	//ORDER_ALLOWANCE_INSUFFICIENT OrderStatus = 8

	ORDER_TYPE_MARKET = "market_order"
	ORDER_TYPE_P2P = "p2p_order"
)

//go:generate gencodec -type Order -field-override orderMarshaling -out gen_order_json.go
type Order struct {
	Protocol              common.Address             `json:"protocol" gencodec:"required"`        // 智能合约地址
	DelegateAddress       common.Address             `json:"delegateAddress" gencodec:"required"` // 智能合约地址
	AuthAddr              common.Address             `json:"authAddr" gencodec:"required"`        //
	AuthPrivateKey        crypto.EthPrivateKeyCrypto `json:"authPrivateKey" gencodec:"required"`  //
	WalletAddress         common.Address             `json:"walletAddress" gencodec:"required"`
	TokenS                common.Address             `json:"tokenS" gencodec:"required"`     // 卖出erc20代币智能合约地址
	TokenB                common.Address             `json:"tokenB" gencodec:"required"`     // 买入erc20代币智能合约地址
	AmountS               *big.Int                   `json:"amountS" gencodec:"required"`    // 卖出erc20代币数量上限
	AmountB               *big.Int                   `json:"amountB" gencodec:"required"`    // 买入erc20代币数量上限
	ValidSince            *big.Int                   `json:"validSince" gencodec:"required"` //
	ValidUntil            *big.Int                   `json:"validUntil" gencodec:"required"` // 订单过期时间
	LrcFee                *big.Int                   `json:"lrcFee" `                        // 交易总费用,部分成交的费用按该次撮合实际卖出代币额与比例计算
	BuyNoMoreThanAmountB  bool                       `json:"buyNoMoreThanAmountB" gencodec:"required"`
	MarginSplitPercentage uint8                      `json:"marginSplitPercentage" gencodec:"required"` // 不为0时支付给交易所的分润比例，否则视为100%
	V                     uint8                      `json:"v" gencodec:"required"`
	R                     Bytes32                    `json:"r" gencodec:"required"`
	S                     Bytes32                    `json:"s" gencodec:"required"`
	Price                 *big.Rat                   `json:"price"`
	Owner                 common.Address             `json:"owner"`
	Hash                  common.Hash                `json:"hash"`
	Market                string                     `json:"market"`
	CreateTime            int64                      `json:"createTime"`
	PowNonce              uint64                     `json:"powNonce"`
	Side                  string                     `json:"side"` // 买的，还是卖的。标识符
	OrderType             string                     `json:"orderType"`
}

type orderMarshaling struct {
	AmountS    *Big
	AmountB    *Big
	ValidSince *Big
	ValidUntil *Big
	LrcFee     *Big
}

//go:generate gencodec -type OrderJsonRequest -field-override orderJsonRequestMarshaling -out gen_order_request_json.go
type OrderJsonRequest struct {
	Protocol        common.Address             `json:"protocol" gencodec:"required"`        // 智能合约地址
	DelegateAddress common.Address             `json:"delegateAddress" gencodec:"required"` // 智能合约地址
	TokenS          common.Address             `json:"tokenS" gencodec:"required"`          // 卖出erc20代币智能合约地址
	TokenB          common.Address             `json:"tokenB" gencodec:"required"`          // 买入erc20代币智能合约地址
	AuthAddr        common.Address             `json:"authAddr" gencodec:"required"`        //
	AuthPrivateKey  crypto.EthPrivateKeyCrypto `json:"authPrivateKey"`  //
	WalletAddress   common.Address             `json:"walletAddress" gencodec:"required"`
	AmountS         *big.Int                   `json:"amountS" gencodec:"required"`    // 卖出erc20代币数量上限
	AmountB         *big.Int                   `json:"amountB" gencodec:"required"`    // 买入erc20代币数量上限
	ValidSince      *big.Int                   `json:"validSince" gencodec:"required"` //
	ValidUntil      *big.Int                   `json:"validUntil" gencodec:"required"` // 订单过期时间
	// Salt                  int64          `json:"salt" gencodec:"required"`
	LrcFee                *big.Int       `json:"lrcFee" ` // 交易总费用,部分成交的费用按该次撮合实际卖出代币额与比例计算
	BuyNoMoreThanAmountB  bool           `json:"buyNoMoreThanAmountB" gencodec:"required"`
	MarginSplitPercentage uint8          `json:"marginSplitPercentage" gencodec:"required"` // 不为0时支付给交易所的分润比例，否则视为100%
	V                     uint8          `json:"v" gencodec:"required"`
	R                     Bytes32        `json:"r" gencodec:"required"`
	S                     Bytes32        `json:"s" gencodec:"required"`
	Price                 *big.Rat       `json:"price"`
	Owner                 common.Address `json:"owner"`
	Hash                  common.Hash    `json:"hash"`
	CreateTime            int64          `json:"createTime"`
	PowNonce              uint64         `json:"powNonce"`
	Side                  string         `json:"side"`
	OrderType             string         `json:"orderType"`
}

type orderJsonRequestMarshaling struct {
	AmountS    *Big
	AmountB    *Big
	ValidSince *Big
	ValidUntil *Big
	LrcFee     *Big
}

func (o *Order) GenerateHash() common.Hash {
	h := &common.Hash{}

	buyNoMoreThanAmountB := byte(0)
	if o.BuyNoMoreThanAmountB {
		buyNoMoreThanAmountB = byte(1)
	}

	hashBytes := crypto.GenerateHash(
		o.DelegateAddress.Bytes(),
		o.Owner.Bytes(),
		o.TokenS.Bytes(),
		o.TokenB.Bytes(),
		o.WalletAddress.Bytes(),
		o.AuthAddr.Bytes(),
		common.LeftPadBytes(o.AmountS.Bytes(), 32),
		common.LeftPadBytes(o.AmountB.Bytes(), 32),
		common.LeftPadBytes(o.ValidSince.Bytes(), 32),
		common.LeftPadBytes(o.ValidUntil.Bytes(), 32),
		common.LeftPadBytes(o.LrcFee.Bytes(), 32),
		[]byte{buyNoMoreThanAmountB},
		[]byte{byte(o.MarginSplitPercentage)},
	)

	h.SetBytes(hashBytes)
	return *h
}

func (o *Order) GenerateAndSetSignature(singerAddr common.Address) error {
	if IsZeroHash(o.Hash) {
		o.Hash = o.GenerateHash()
	}

	if sig, err := crypto.Sign(o.Hash.Bytes(), singerAddr); nil != err {
		return err
	} else {
		v, r, s := crypto.SigToVRS(sig)
		o.V = uint8(v)
		o.R = BytesToBytes32(r)
		o.S = BytesToBytes32(s)
		return nil
	}
}

func (o *Order) ValidateSignatureValues() bool {
	return crypto.ValidateSignatureValues(byte(o.V), o.R.Bytes(), o.S.Bytes())
}

func (o *Order) SignerAddress() (common.Address, error) {
	address := &common.Address{}
	if IsZeroHash(o.Hash) {
		o.Hash = o.GenerateHash()
	}

	sig, _ := crypto.VRSToSig(o.V, o.R.Bytes(), o.S.Bytes())

	if addressBytes, err := crypto.SigToAddress(o.Hash.Bytes(), sig); nil != err {
		log.Errorf("type,order signer address error:%s", err.Error())
		return *address, err
	} else {
		address.SetBytes(addressBytes)
		return *address, nil
	}
}

func (o *Order) GeneratePrice() {
	o.Price = new(big.Rat).SetFrac(o.AmountS, o.AmountB)
}

// 根据big.Rat价格计算big.int remainAmount
// buyNoMoreThanAmountB == true  已知remainAmountB计算remainAmountS
// buyNoMoreThanAmountB == false 已知remainAmountS计算remainAmountB
//func (ord *OrderState) CalculateRemainAmount() {
//	const RATE = 1.0e18
//
//	price, _ := ord.RawOrder.Price.Float64()
//	price = price * RATE
//	bigPrice := big.NewInt(int64(price))
//	bigRate := big.NewInt(RATE)
//
//	if ord.RawOrder.BuyNoMoreThanAmountB == true {
//		beenRateAmountB := new(big.Int).Mul(ord.DealtAmountB, bigPrice)
//		ord.DealtAmountS = new(big.Int).Div(beenRateAmountB, bigRate)
//	} else {
//		beenRateAmountS := new(big.Int).Mul(ord.DealtAmountS, bigRate)
//		ord.DealtAmountB = new(big.Int).Div(beenRateAmountS, bigPrice)
//	}
//}

//RateAmountS、FeeSelection 需要提交到contract
type FilledOrder struct {
	OrderState       OrderState `json:"orderState" gencodec:"required"`
	FeeSelection     uint8      `json:"feeSelection"`     //0 -> lrc
	RateAmountS      *big.Rat   `json:"rateAmountS"`      //提交需要
	AvailableAmountS *big.Rat   `json:"availableAmountS"` //需要，也是用于计算fee
	AvailableAmountB *big.Rat   //需要，也是用于计算fee
	FillAmountS      *big.Rat   `json:"fillAmountS"`
	FillAmountB      *big.Rat   `json:"fillAmountB"` //计算需要
	LrcReward        *big.Rat   `json:"lrcReward"`

	LrcFee           *big.Rat   `json:"lrcFee"` // 需要支付的 Lrc 值，支付给矿工
	LegalLrcFee      *big.Rat   `json:"legalLrcFee"` // 需要支付的 Lrc 换算成法币有多少￥，就是USD的价值

	FeeS             *big.Rat   `json:"feeS"`      // 差价 值
	LegalFeeS        *big.Rat   `json:"legalFeeS"` // 差价 法币有多少￥，就是USD的价值

	// LegalFee 是根据fees以及lrcfee计算出最终的法币金额
	// 简单说，LegalFee 是根据LegalLrcFee和LegalFeeS 选出的一个最大的
	// 其实是比较啰嗦的，为了方便记录和日志debug
	LegalFee         *big.Rat   `json:"legalFee"`  //法币计算的fee，是最终选择的法币金额

	SPrice *big.Rat `json:"SPrice"`
	BPrice *big.Rat `json:"BPrice"`

	AvailableLrcBalance    *big.Rat
	AvailableTokenSBalance *big.Rat
}

func ConvertOrderStateToFilledOrder(
	orderState OrderState, lrcBalance, tokenSBalance *big.Rat, lrcAddress common.Address) *FilledOrder {

	filledOrder := &FilledOrder{}
	filledOrder.OrderState = orderState
	filledOrder.AvailableLrcBalance = new(big.Rat).Set(lrcBalance) // 分母是1
	filledOrder.AvailableTokenSBalance = new(big.Rat).Set(tokenSBalance) // 分母是1

	// lgh: 计算当前订单还剩下多少卖的和买的
	filledOrder.AvailableAmountS, filledOrder.AvailableAmountB = filledOrder.OrderState.RemainedAmount() //----⑥

	// 原始的比例 RawOrder --> 卖/买
	sellPrice := new(big.Rat).SetFrac(filledOrder.OrderState.RawOrder.AmountS, filledOrder.OrderState.RawOrder.AmountB)

	// lgh: todo 为什么还要实例化一次 availableBalance？这个等同于  AvailableTokenSBalance
	availableBalance := new(big.Rat).Set(filledOrder.AvailableTokenSBalance)

	// AvailableAmountS 订单里剩下要 sell 卖的。
	// availableBalance 要卖的代币的余额
	if availableBalance.Cmp(filledOrder.AvailableAmountS) < 0 { // ----⑦
		// 自己的余额比自己当前订单的要少
		filledOrder.AvailableAmountS = availableBalance // 订单的变成余额的。因为最多就余额那么多，不能超过
		// lgh: 因为上面的 S 变了，所以下面要重新计算一次 B
		// Inv 是倒过来。下面再次计算一次剩下要买的
		// AvailableAmountB = (AvailableAmountS * (原始的AmountB / 原始的AmountS))
		// 假设 (原始的AmountB / 原始的AmountS) = 10 ETH/3000 USD，卖 300 USD 买一个 ETH
		// AvailableAmountS = 500，订单里剩下要卖 500 USD
		// 那么剩下要买的(ETH AvailableAmountB) = 剩下要卖的(USD AvailableAmountS)* (1/300)，这是对的
		// AvailableAmountB/AvailableAmountS = 原始的AmountB / 原始的AmountS
		// ==> AvailableAmountB = AvailableAmountS*(原始的AmountB / 原始的AmountS)
		filledOrder.AvailableAmountB.Mul(filledOrder.AvailableAmountS, new(big.Rat).Inv(sellPrice)) // ----⑤
	}
	if filledOrder.OrderState.RawOrder.BuyNoMoreThanAmountB {
		// BuyNoMoreThanAmountB = true ====> 不允许最终成交的TokenB超过AmountB
		// AvailableAmountS/AvailableAmountB = (原始的AmountS / 原始的AmountB)
		// AvailableAmountS = AvailableAmountB * (原始的AmountS / 原始的AmountB)
		// lgh: todo 下面行应该也是多余操作， ----⑥ 已经计算超过一次了，如果进入了上面的 ----⑦ 也无需再计算一次
		filledOrder.AvailableAmountS.Mul(filledOrder.AvailableAmountB, sellPrice)
	} else {
		// lgh: todo 下面这行是多余的操作，完全和 ----⑤ 一样。假设没进入 ----⑦ S 也就没改变，而----⑥ 已经计算超过一次
		filledOrder.AvailableAmountB.Mul(filledOrder.AvailableAmountS, new(big.Rat).Inv(sellPrice))
	}

	if orderState.RawOrder.TokenB == lrcAddress && lrcBalance.Cmp(filledOrder.AvailableAmountB) < 0 {
		// 如果要买的是 LRC，且用户的LRC的余额比剩下要买的少，那么设置用户的可用LRC余额为 剩下要买的
		// 要买的代币 = LRC && (用户LRC余额<剩下要买的) => 用户可用LRC余额=剩下要买的
		filledOrder.AvailableLrcBalance.Set(filledOrder.AvailableAmountB)
	}
	return filledOrder
}

// 从[]byte解析时使用json.Unmarshal
type OrderState struct {
	RawOrder         Order       `json:"rawOrder"`
	UpdatedBlock     *big.Int    `json:"updatedBlock"`
	DealtAmountS     *big.Int    `json:"dealtAmountS"`
	DealtAmountB     *big.Int    `json:"dealtAmountB"`
	SplitAmountS     *big.Int    `json:"splitAmountS"`
	SplitAmountB     *big.Int    `json:"splitAmountB"`
	CancelledAmountS *big.Int    `json:"cancelledAmountS"`
	CancelledAmountB *big.Int    `json:"cancelledAmountB"`
	Status           OrderStatus `json:"status"`
	BroadcastTime    int         `json:"broadcastTime"`
}

type OrderDelayList struct {
	OrderHash    []common.Hash
	DelayedCount int64
}

func InUnchangeableStatus(status OrderStatus) bool {
	unchangeableList := []OrderStatus{
		ORDER_FINISHED, ORDER_UNKNOWN, ORDER_CANCEL, ORDER_CUTOFF}

	for _, v := range unchangeableList {
		if status == v {
			return true
		}
	}

	return false
}

func (ord *OrderState) IsExpired() bool {
	if (ord.Status == ORDER_NEW || ord.Status == ORDER_PARTIAL) && ord.RawOrder.ValidUntil.Int64() < time.Now().Unix() {
		return true
	}
	return false
}

func (ord *OrderState) IsEffective() bool {
	if (ord.Status == ORDER_NEW || ord.Status == ORDER_PARTIAL) &&
		ord.RawOrder.ValidSince.Int64() <= time.Now().Unix() &&
		ord.RawOrder.ValidUntil.Int64() > time.Now().Unix() {
		return true
	}
	return false
}

// 解释订单最终状态
func (ord *OrderState) ResolveStatus(allowance, balance *big.Int) {
	if InUnchangeableStatus(ord.Status) {
		return
	}

	if ord.RawOrder.ValidUntil.Int64() < time.Now().Unix() {
		ord.Status = ORDER_EXPIRE
		return
	}

	//cancelOrFilled := new(big.Int).Add(ord.CancelledAmountS, ord.DealtAmountS)
	//finished := new(big.Int).Add(cancelOrFilled, ord.SplitAmountS)

	//if finished.Cmp(allowance) >= 0 {
	//	ord.Status = ORDER_ALLOWANCE_INSUFFICIENT
	//	return
	//}
	//
	//if finished.Cmp(balance) >= 0 {
	//	ord.Status = ORDER_BALANCE_INSUFFICIENT
	//	return
	//}
}

//const (
//	SIDE_SELL    = "sell"
//	SIDE_BUY     = "buy"
//	SIDE_INVALID = ""
//)

// 根据市场确定订单方向
//func (ord *OrderState) Side(market common.Address) string {
//	var side string
//	switch market {
//	case ord.RawOrder.TokenS:
//		side = SIDE_SELL
//	case ord.RawOrder.TokenB:
//		side = SIDE_BUY
//	default:
//		side = SIDE_INVALID
//	}
//
//	return side
//}

// lgh: Remained 留下来。这个方法就是用来计算当前订单还剩下多少卖的和买的
func (orderState *OrderState) RemainedAmount() (remainedAmountS *big.Rat, remainedAmountB *big.Rat) {
	remainedAmountS = new(big.Rat)
	remainedAmountB = new(big.Rat)
	// lgh: BuyNoMoreThanAmountB 在这里影响的是，以谁为基础标准算出另一个。感觉就是一样的
	if orderState.RawOrder.BuyNoMoreThanAmountB {
		// (BuyNoMoreThanAmountB = true) ====> 不允许最终成交的TokenB超过AmountB
		reducedAmountB := new(big.Rat)
		reducedAmountB.
			// reducedAmountB 是要减去的总的买量
			// lgh: DealtAmountB 已经处理了的，已经取消了的 CancelledAmountB，已经拆分了的 SplitAmountB
			Add(reducedAmountB, new(big.Rat).SetInt(orderState.DealtAmountB)).
			Add(reducedAmountB, new(big.Rat).SetInt(orderState.CancelledAmountB)).
			Add(reducedAmountB, new(big.Rat).SetInt(orderState.SplitAmountB))

		sellPrice := new(big.Rat).SetFrac(orderState.RawOrder.AmountS, orderState.RawOrder.AmountB)
		remainedAmountB. // 以买剩多少为标准
			Sub( // 减法  AmountB - reducedAmountB = AmountB - 0
				new(big.Rat).SetInt(orderState.RawOrder.AmountB),
				reducedAmountB) // reducedAmountB 首次是0
		// remainedAmountB = AmountB
		remainedAmountS.Mul(remainedAmountB, sellPrice) // AmountB * (卖的/买的) = 剩下要卖的 remainedAmountS。
	} else {
		reducedAmountS := new(big.Rat)
		reducedAmountS.
			Add(reducedAmountS, new(big.Rat).SetInt(orderState.DealtAmountS)).
			Add(reducedAmountS, new(big.Rat).SetInt(orderState.CancelledAmountS)).
			Add(reducedAmountS, new(big.Rat).SetInt(orderState.SplitAmountS))

		buyPrice := new(big.Rat).SetFrac(orderState.RawOrder.AmountB, orderState.RawOrder.AmountS)

		remainedAmountS. // 以卖剩多少为标准
			Sub(
				new(big.Rat).SetInt(orderState.RawOrder.AmountS),
				reducedAmountS)
		// remainedAmountB = remainedAmountS*(买的/卖的)
		remainedAmountB.Mul(remainedAmountS, buyPrice)
	}

	return remainedAmountS, remainedAmountB
}

func (state *OrderState) DealtAndSplitAmount() (totalAmountS *big.Rat, totalAmountB *big.Rat) {
	totalAmountS = new(big.Rat)
	totalAmountB = new(big.Rat)

	if state.RawOrder.BuyNoMoreThanAmountB {
		totalAmountB = totalAmountB.SetInt(new(big.Int).Add(state.DealtAmountB, state.SplitAmountB))
		sellPrice := new(big.Rat).SetFrac(state.RawOrder.AmountS, state.RawOrder.AmountB)
		totalAmountS = totalAmountS.Mul(totalAmountB, sellPrice)
	} else {
		totalAmountS = totalAmountS.SetInt(new(big.Int).Add(state.DealtAmountS, state.SplitAmountS))
		buyPrice := new(big.Rat).SetFrac(state.RawOrder.AmountB, state.RawOrder.AmountS)
		totalAmountB = totalAmountB.Mul(totalAmountS, buyPrice)
	}

	return totalAmountS, totalAmountB
}

func ToOrder(request *OrderJsonRequest) *Order {
	order := &Order{}
	order.Protocol = request.Protocol
	order.DelegateAddress = request.DelegateAddress
	order.TokenS = request.TokenS
	order.TokenB = request.TokenB
	order.AmountS = request.AmountS
	order.AmountB = request.AmountB
	order.ValidSince = request.ValidSince
	order.ValidUntil = request.ValidUntil
	order.AuthAddr = request.AuthAddr
	order.AuthPrivateKey = request.AuthPrivateKey
	order.LrcFee = request.LrcFee
	order.BuyNoMoreThanAmountB = request.BuyNoMoreThanAmountB
	order.MarginSplitPercentage = request.MarginSplitPercentage
	order.V = request.V
	order.R = request.R
	order.S = request.S
	order.Owner = request.Owner
	order.WalletAddress = request.WalletAddress
	order.PowNonce = request.PowNonce
	order.OrderType = request.OrderType
	return order
}
