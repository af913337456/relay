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

package miner

import (
	"errors"
	"github.com/Loopring/relay/log"
	"math"
	"math/big"

	"fmt"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/ethaccessor"
	"github.com/Loopring/relay/marketcap"
	"github.com/Loopring/relay/types"
	"github.com/ethereum/go-ethereum/common"
)

type Evaluator struct {
	marketCapProvider         marketcap.MarketCapProvider
	rateRatioCVSThreshold     int64  // lgh: 汇率比率系数阈值
	gasUsedWithLength         map[int]*big.Int // lgh: 成环时候，订单数不同对于的 gas 的乘基准
	realCostRate, walletSplit *big.Rat
	minGasPrice, maxGasPrice *big.Int
	feeReceipt               common.Address // lgh: 当前矿工的收益地址
	matcher Matcher
}

// lgh: 从 GenerateCandidateRing 进入 ComputeRing 的情况，那么 ringState 里面的 order size == 2
// lgh: 计算 (1-折价率)。环中所有订单的: 1/[(所有订单卖的乘积/所有订单买的乘积)^(1/订单数)]
func ReducedRate(ringState *types.Ring) *big.Rat {

	productAmountS := big.NewRat(int64(1), int64(1)) // 初始化是 1/1 = 1
	productAmountB := big.NewRat(int64(1), int64(1))

	//compute price
	for _, order := range ringState.Orders {
		amountS := new(big.Rat).SetInt(order.OrderState.RawOrder.AmountS)
		amountB := new(big.Rat).SetInt(order.OrderState.RawOrder.AmountB)

		// lgh: 因为一个 ring 可以有多个 订单，所以这里是累乘，第一次是 amountS 和 amountB
		// 第二次是 (a->b.amountS * b-a>.amountS) 和 (a->b.amountB * b-a>.amountB)
		productAmountS.Mul(productAmountS, amountS)
		productAmountB.Mul(productAmountB, amountB)

		order.SPrice = new(big.Rat)
		order.SPrice.Quo(amountS, amountB) // 原始卖的 / 原始买的

		order.BPrice = new(big.Rat)
		order.BPrice.Quo(amountB, amountS) // 原始买的 / 原始卖的
	}

	// lgh: 由上面的分析，我们可以肯定，productAmountS 是 Orders 的所有卖的量的 乘积。同理 productAmountB 也是如此
	// lgh: 那么 productPrice 就是 Orders 的 (所有卖的量/所有买的量)。类似平均价
	productPrice := new(big.Rat)
	productPrice.Quo(productAmountS, productAmountB)

	//todo:change pow to big.Int
	priceOfFloat, _ := productPrice.Float64() // productPrice 转为浮点数的形式
	// math.Pow -> 返回 x 的 y 次幂的值。 priceOfFloat^(1/float64(len(ringState.Orders))) 事实以订单数开方
	rootOfRing := math.Pow(priceOfFloat, 1/float64(len(ringState.Orders)))
	rate := new(big.Rat).SetFloat64(rootOfRing)
	reducedRate := new(big.Rat)
	reducedRate.Inv(rate) // todo 为何要翻转一次？
	log.Debugf("Miner,rate:%s, priceFloat:%f , len:%d, rootOfRing:%f, reducedRate:%s ", rate.FloatString(2), priceOfFloat, len(ringState.Orders), rootOfRing, reducedRate.FloatString(2))

	return reducedRate // lgh: 返回的没有 1-
}

// lgh: 从 GenerateCandidateRing 进入的情况，那么 ringState 里面的 order size == 2
// lgh: 此方法要结合 zh_whitepaper.pdf 白皮书的 订单缩减 公式一起理解
func (e *Evaluator) ComputeRing(ringState *types.Ring) error {

	if len(ringState.Orders) <= 1 {
		// lgh: 在 Hash 没被赋值的情况，Hash.Hex() 输出的是 0x00000...
		return fmt.Errorf("length of ringState.Orders must > 1 , ringhash:%s", ringState.Hash.Hex())
	}

	// lgh: 计算`汇率折价`，它是基于环路订单组计算出的。1/[(所有订单卖的乘积/所有订单买的乘积)^(1/订单数)]
	// 矿工提交后，LPSC 会验证 `汇率折价`,汇率折价 0<=y<1
	ringState.ReducedRate = ReducedRate(ringState)

	//todo:get the fee for select the ring of mix income
	//LRC等比例下降，首先需要计算fillAmountS
	//分润的fee，首先需要计算fillAmountS，fillAmountS取决于整个环路上的完全匹配的订单
	//如何计算最小成交量的订单，计算下一次订单的卖出或买入，然后根据比例替换
	minVolumeIdx := 0 // lgh: 最小成交量的订单的下标

	for idx, filledOrder := range ringState.Orders {

		// lgh: todo 注意！ 每个 order SPrice 和 BPrice 在 ReducedRate 里面已经设置过一次了
		// lgh: 解决上面的 todo
		// 原因是：由白皮书可知，SPrice 代表的是当前环路订单的实际交易汇率。为 R(origin)*(汇率折价y)<=(R(origin)=Sell/Buy)
		filledOrder.SPrice.Mul(filledOrder.SPrice, ringState.ReducedRate)
		// lgh: todo 为什么 BPrice != (B/S)*ReducedRate = BPrice*ReducedRate
		// lgh: todo 下面倒置的事实是 1/[(S/B)*ReducedRate]
		// lgh: 解决上面的 todo s
		// 原因是: (Sell/Buy)*(Buy/Sell)==1，是恒成立的。而汇率的默认形式就是 (Sell/Buy)，所以在计算出了 SPrice，直接取倒数就是 BPrice
		filledOrder.BPrice.Inv(filledOrder.SPrice)

		// lgh: 目前看来，SPrice 和 BPrice 对应的含义分别是订单处于当前环路订单组的实际交易汇率 卖/买 和 买/卖

		amountS := new(big.Rat).SetInt(filledOrder.OrderState.RawOrder.AmountS)
		//amountB := new(big.Rat).SetInt(filledOrder.OrderState.RawOrder.AmountB)

		//根据用户设置，判断是以卖还是买为基准
		//买入不超过amountB
		filledOrder.RateAmountS = new(big.Rat).Set(amountS)
		// lgh: 有 RateAmountS 却没有 RateAmountB，猜测这个是 LPSC 验证需要而设置的。是单个订单的原始(Sell量*汇率折价后)的值
		filledOrder.RateAmountS.Mul(amountS, ringState.ReducedRate)

		//if BuyNoMoreThanAmountB , AvailableAmountS need to be reduced by the ratePrice
		//recompute availabeAmountS and availableAmountB by the latest price
		if filledOrder.OrderState.RawOrder.BuyNoMoreThanAmountB {
			// 不允许买入超过 amountB 个，下面又计算了一次 AvailableAmountS(剩下要购买的量)
			// 由买的决定卖的，因为最多不超过买的量
			// AvailableAmountS = 实际交易汇率 * 剩下要买入的量
			// AvailableAmountS = (Sell/Buy) * y * AvailableAmountB
			// 可以看出，这里的又一次的计算事实是引入了 实际交易汇率 的情况。
			filledOrder.AvailableAmountS.Mul(filledOrder.SPrice, filledOrder.AvailableAmountB)
		} else {
			// 同上
			// 由卖 的决定 买的，因为最多不超过卖的量
			filledOrder.AvailableAmountB.Mul(filledOrder.BPrice, filledOrder.AvailableAmountS)
		}
		log.Debugf("orderhash:%s availableAmountS:%s, availableAmountB:%s", filledOrder.OrderState.RawOrder.Hash.Hex(), filledOrder.AvailableAmountS.FloatString(2), filledOrder.AvailableAmountB.FloatString(2))

		//与上一订单的买入进行比较
		var lastOrder *types.FilledOrder
		if idx > 0 {
			lastOrder = ringState.Orders[idx-1]
		}

		filledOrder.FillAmountS = new(big.Rat)
		if lastOrder != nil && lastOrder.FillAmountB.Cmp(filledOrder.AvailableAmountS) >= 0 {
			// 稿纸上的 3
			// FillAmountB 第一次是 0
			// 上个订单买的 >= 当前订单剩下要卖的。那么当前订单卖出的 设置为 它自己剩下要卖的。因为它满足不了上个订单
			// 当前订单为最小订单
			filledOrder.FillAmountS.Set(filledOrder.AvailableAmountS) // 当前订单的卖量 设置为 FillAmountS
			minVolumeIdx = idx
			//根据minVolumeIdx进行最小交易量的计算,两个方向进行
		} else if lastOrder == nil {
			// 首次设置： 当前订单要卖的 为 当前订单剩下可以卖的
			filledOrder.FillAmountS.Set(filledOrder.AvailableAmountS)
		} else {
			// 稿纸上的 4
			// 否则： 上个订单买的 < 当前订单剩下要卖的。那么当前订单卖的 设置为 上个订单买的。这样才够卖，当前的能满足上一个的
			// 上一订单为最小订单，需要对remainAmountS进行折扣计算
			filledOrder.FillAmountS.Set(lastOrder.FillAmountB) // 上个订单买的为当前订单卖的
		}
		// 每次的循环之后，设置当前订单的 FillAmountB = AvailableAmountS * BPrice
		// FillAmountB = AvailableAmountS * (B/S*ReducedRate)
		filledOrder.FillAmountB = new(big.Rat).Mul(filledOrder.FillAmountS, filledOrder.BPrice)
	}

	//compute the volume of the ring by the min volume
	//todo:the first and the last
	//if (ring.RawRing.Orders[len(ring.RawRing.Orders) - 1].FillAmountB.Cmp(ring.RawRing.Orders[0].FillAmountS) < 0) {
	//	minVolumeIdx = len(ring.RawRing.Orders) - 1
	//	for i := minVolumeIdx-1; i >= 0; i-- {
	//		//按照前面的，同步减少交易量
	//		order := ring.RawRing.Orders[i]
	//		var nextOrder *types.FilledOrder
	//		nextOrder = ring.RawRing.Orders[i + 1]
	//		order.FillAmountB = nextOrder.FillAmountS
	//		order.FillAmountS.Mul(order.FillAmountB, order.EnlargedSPrice)
	//	}
	//}

	// lgh: 上个订单的卖出为下一个订单的买入。下面的循环是再循环一次。根据上面已经预处理了一次的情况
	for i := minVolumeIdx - 1; i >= 0; i-- {
		//按照前面的，同步减少交易量
		order := ringState.Orders[i]
		var nextOrder *types.FilledOrder
		nextOrder = ringState.Orders[i+1]

		order.FillAmountB = nextOrder.FillAmountS // 上一个买的是下一个卖的，nextOrder 是下一个
		order.FillAmountS.Mul(order.FillAmountB, order.SPrice)
	}
	// lgh: len(ringState.Orders) 提到外边，不用每次读一次 len
	orderLen := len(ringState.Orders)
	for i := minVolumeIdx + 1; i < orderLen; i++ {
		order := ringState.Orders[i]
		var lastOrder *types.FilledOrder
		lastOrder = ringState.Orders[i-1]
		order.FillAmountS = lastOrder.FillAmountB // 上一个买的是下一个卖的，lastOrder 是上一个
		order.FillAmountB.Mul(order.FillAmountS, order.BPrice)
	}

	//compute the fee of this ring and orders, and set the feeSelection
	// lgh: 这里面的函数内部进行了油费的赋值
	if err := e.computeFeeOfRingAndOrder(ringState); nil != err {
		return err
	}

	//cvs
	cvs, err := PriceRateCVSquare(ringState)
	if nil != err {
		return err
	} else {
		if cvs.Int64() <= e.rateRatioCVSThreshold {
			return nil
		} else {
			for _, o := range ringState.Orders {
				log.Debugf("cvs bigger than RateRatioCVSThreshold orderhash:%s", o.OrderState.RawOrder.Hash.Hex())
			}
			return errors.New("Miner,cvs must less than RateRatioCVSThreshold")
		}
	}

}

// lgh: 这部分对于白皮书的 费用模式 计算差价和手续费相关
func (e *Evaluator) computeFeeOfRingAndOrder(ringState *types.Ring) error {

	var err error
	var feeReceiptLrcAvailableAmount *big.Rat
	var lrcAddress common.Address
	if impl, exists := ethaccessor.ProtocolAddresses()[ringState.Orders[0].OrderState.RawOrder.Protocol]; exists {
		var err error
		lrcAddress = impl.LrcTokenAddress
		//todo:the address transfer lrcReward should be msg.sender not feeReceipt
		if feeReceiptLrcAvailableAmount, err =
			// lgh: 获取 e.feeReceipt 当前矿工收益地址 的 lrc 余额
			e.matcher.GetAccountAvailableAmount(e.feeReceipt, lrcAddress, impl.DelegateAddress); nil != err {
			return err
		}
	} else {
		return errors.New("not support this protocol: " + ringState.Orders[0].OrderState.RawOrder.Protocol.Hex())
	}

	ringState.LegalFee = big.NewRat(int64(0), int64(1)) // lgh: 初始化是0
	for _, filledOrder := range ringState.Orders {
		// 开始遍历所有订单
		legalAmountOfSaving := new(big.Rat)
		if filledOrder.OrderState.RawOrder.BuyNoMoreThanAmountB {
			amountS := new(big.Rat).SetInt(filledOrder.OrderState.RawOrder.AmountS)
			amountB := new(big.Rat).SetInt(filledOrder.OrderState.RawOrder.AmountB)
			sPrice := new(big.Rat)
			sPrice.Quo(amountS, amountB) // 原始的汇率
			savingAmount := new(big.Rat)
			// 使用原始的汇率(AmountS/AmountB) 乘上 实际汇率得出的要买的数量(FillAmountB) = savingAmount
			// 因为：实际汇率是 <= 原始汇率的，所以 savingAmount >= FillAmountS
			savingAmount.Mul(filledOrder.FillAmountB, sPrice) // 由 买的 决定 卖的
			// 再减去 根据实际汇率得出的要卖的 FillAmountS。看到这里 savingAmount 类似作差？
			savingAmount.Sub(savingAmount, filledOrder.FillAmountS)
			filledOrder.FeeS = savingAmount // 计算出差价

			// 下面基于 TokenS 的市值计算出 FeeS 价值多少 USD(单位在配置文件控制)
			legalAmountOfSaving, err = e.getLegalCurrency(filledOrder.OrderState.RawOrder.TokenS, filledOrder.FeeS)
			if nil != err {
				return err
			}
		} else {
			// 下面 BuyNoMoreThanAmountB = false 的情况
			savingAmount := new(big.Rat)
			// 折价汇率 y = 1-1/[math.pow(r1,r2,r3,3)]
			// ringState.ReducedRate 是 折价汇率，为什么此时 savingAmount = FillAmountB * 折价汇率 呢？
			savingAmount.Mul(filledOrder.FillAmountB, ringState.ReducedRate)
			// reduceDrate = 1-y
			// 连起来就是： savingAmount = FillAmountB * (1-reduceDrate)
			// ==> savingAmount = FillAmountB * y , savingAmount 是折价的数量
			// lgh: todo 解释为什么是 savingAmount = FillAmountB * (1-reduceDrate)
			/**
				算差价量，首先要计算出真实的卖量
				FillAmountS/FillAmountB = (amountS/amountB) * reducedRate
				FillAmountS = FillAmountB * (amountS/amountB)*reducedRate
				以原先的兑换率，计算出，原先可以买的量
				amountB‘ = FillAmountS * (amountB/amountS)
				差量
				savingAmount = FillAmountB - amountB‘，公式带入后，就是
				savingAmount = FillAmountB * (1-reducedRate) = FillAmountB - FillAmountS * (amountB/amountS)
			*/
			savingAmount.Sub(filledOrder.FillAmountB, savingAmount)
			filledOrder.FeeS = savingAmount
			legalAmountOfSaving, err = e.getLegalCurrency(
				filledOrder.OrderState.RawOrder.TokenB,
				filledOrder.FeeS)
			if nil != err {
				return err
			}
		}

		// lgh: FeeS 差价总结
		/*
			true  => savingAmount = (amountB/amountS) * FillAmountB - FillAmountS
			false => savingAmount = FillAmountB - (amountS/amountB) * FillAmountS
		*/

		//compute lrcFee 需要支付的 Lrc 值
		rate := new(big.Rat).
				// FillAmountS / AmountS = 真实要卖的比上原来要卖的比例
				Quo(
					filledOrder.FillAmountS,
					new(big.Rat).SetInt(filledOrder.OrderState.RawOrder.AmountS))

		filledOrder.LrcFee = new(big.Rat).SetInt(filledOrder.OrderState.RawOrder.LrcFee) // 先设置为订单中带过来的手续费
		filledOrder.LrcFee.Mul(filledOrder.LrcFee, rate) // 根据上面的比例来计算具体要付的手续费

		// 在 order.go 的 AvailableLrcBalance 方法中初始化 ConvertOrderStateToFilledOrder
		// 如果要买的 TokenB 是 Lrc，那么 AvailableLrcBalance 和 availableB 一样，就是剩下要买的 。
		// 否则就是用户的 lrc 余额
		// 下面是和手续费比较
		if filledOrder.AvailableLrcBalance.Cmp(filledOrder.LrcFee) <= 0 {
			// <= 手续费，那么手续费改为 AvailableLrcBalance。
			// 这里不用担心，LrcFee 会变得很大，因为上面计算的时候是根据 “订单中带过来的手续费”
			// 如果 AvailableLrcBalance <= LrcFee，那么下面的赋值只会更小
			filledOrder.LrcFee = filledOrder.AvailableLrcBalance
		}

		// 计算出手续费的实际价格 legalAmountOfLrc
		legalAmountOfLrc, err1 := e.getLegalCurrency(lrcAddress, filledOrder.LrcFee)
		if nil != err1 {
			return err1
		}

		filledOrder.LegalLrcFee = legalAmountOfLrc // 手续费的实际价格
		splitPer := new(big.Rat) // 比例的百分比分数形式
		// MarginSplitPercentage 分润比例
		if filledOrder.OrderState.RawOrder.MarginSplitPercentage > 100 { // 100%
			splitPer.SetFrac64(int64(100), int64(100)) // 1
		} else {
			// MarginSplitPercentage / 100
			splitPer.SetFrac64(int64(filledOrder.OrderState.RawOrder.MarginSplitPercentage), int64(100))
		}
		legalAmountOfSaving.Mul(legalAmountOfSaving, splitPer) // 差价 FeeS 的价值 * 分润比例
		filledOrder.LegalFeeS = legalAmountOfSaving // 这个时候才赋值 FeeS 差价真实的市值钱数 = 原FeeS 的价值 * 分润比例
		log.Debugf(
			"orderhash:%s, raw.lrc:%s, AvailableLrcBalance:%s, lrcFee:%s, feeS:%s, legalAmountOfLrc:%s,  legalAmountOfSaving:%s, minerLrcAvailable:%s",
			filledOrder.OrderState.RawOrder.Hash.Hex(),
			filledOrder.OrderState.RawOrder.LrcFee.String(),
			filledOrder.AvailableLrcBalance.FloatString(2),
			filledOrder.LrcFee.FloatString(2),
			filledOrder.FeeS.FloatString(2),
			legalAmountOfLrc.FloatString(2), legalAmountOfSaving.FloatString(2), feeReceiptLrcAvailableAmount.FloatString(2))

		lrcFee := new(big.Rat).SetInt(big.NewInt(int64(2)))
		lrcFee.Mul(lrcFee, filledOrder.LegalLrcFee) // lrcFee = 2 * 手续费的实际价格
		if lrcFee.Cmp(filledOrder.LegalFeeS) < 0 && feeReceiptLrcAvailableAmount.Cmp(filledOrder.LrcFee) > 0 {
			// (2 * 手续费的实际价格) < FeeS 差价真实的实际价格(原FeeS 的价值 * 分润比例)
			// && 当前矿工收益地址 的 lrc 余额 > 乘上比例后的手续费LrcFee
			// if LegalLrcFee == 0 ==> lrcFee == 0 也适合该模式
			// 白皮书: 分润收入超过2倍 LRx 手续费，矿工便会选取分润模式，向用户支付 LRx 手续费。
			filledOrder.FeeSelection = 1

			// 下面调整差价实际价格： LegalFeeS = LegalFeeS - LegalLrcFee(手续费的实际价格)
			// 为什么不会出现负数? 因为 lrcFee = 2*LegalLrcFee 且 lrcFee < LegalFeeS，证明 LegalLrcFee 的两倍都还要比 LegalFeeS 小
			filledOrder.LegalFeeS.Sub(filledOrder.LegalFeeS, filledOrder.LegalLrcFee)

			filledOrder.LrcReward = filledOrder.LegalLrcFee // 给用户的手续费 = LRx 手续费

			// 选取分润模式
			// 下面： 当前环的矿工总手续费 = 当前环的矿工总手续费 + 当前订单的(原FeeS 的价值 * 分润比例)。累加每个订单的 LegalFeeS 分润价格
			ringState.LegalFee.Add(ringState.LegalFee, filledOrder.LegalFeeS)

			// 下面:  当前矿工收益地址的lrc 余额 = 当前矿工收益地址的lrc 余额 - 手续费
			// 因为上面 LrcReward 给了用户手续费，这部分从矿工账户扣，说白了，给用户的是从矿工处扣
			// lgh: todo 如果 feeReceiptLrcAvailableAmount < 0 是否应该报错?
			/*
			lgh: 解答上面 todo
			todo，报错会发生在LPSC合约，而不是撮合。而如果不够合约就会选择lrc而不是分润，
			todo miner这边的逻辑只是模拟合约执行计算收益，白皮书的完整逻辑是在合约中实现的
			*/
			feeReceiptLrcAvailableAmount.Sub(feeReceiptLrcAvailableAmount, filledOrder.LrcFee)
			//log.Debugf("Miner,lrcReward:%s  legalFee:%s", lrcReward.FloatString(10), filledOrder.LegalFee.FloatString(10))
		} else {
			// 白皮书: 如果分润为 0，矿工选取 LRx 手续费，仍能得到奖励。
			filledOrder.FeeSelection = 0 // 给用户的手续费 = 0
			filledOrder.LegalFeeS = filledOrder.LegalLrcFee // 差价价格 = 手续费实际价格
			filledOrder.LrcReward = new(big.Rat).SetInt(big.NewInt(int64(0))) // 0

			// 下面： 当前环的矿工总手续费 = 当前订单的手续费实际价格 + 当前环的矿工总手续费。累加每个订单的 LegalLrcFee
			ringState.LegalFee.Add(ringState.LegalFee, filledOrder.LegalLrcFee)
			// 这里给用户的是 0 ， 就不需要从矿工账户扣
		}
	}
	e.evaluateReceived(ringState)
	return nil
}

//成环之后才可计算能否成交，否则不需计算，判断是否能够成交，不能使用除法计算
func PriceValid(a2BOrder *types.OrderState, b2AOrder *types.OrderState) bool {
	amountS := new(big.Int).Mul(a2BOrder.RawOrder.AmountS, b2AOrder.RawOrder.AmountS)
	amountB := new(big.Int).Mul(a2BOrder.RawOrder.AmountB, b2AOrder.RawOrder.AmountB)
	// lgh: s/b * s1/b1 >= 1 这里的就是路印成环规则
	return amountS.Cmp(amountB) >= 0
}

func PriceRateCVSquare(ringState *types.Ring) (*big.Int, error) {
	rateRatios := []*big.Int{}
	scale, _ := new(big.Int).SetString("10000", 0)
	for _, filledOrder := range ringState.Orders {
		rawOrder := filledOrder.OrderState.RawOrder
		s1b0, _ := new(big.Int).SetString(filledOrder.RateAmountS.FloatString(0), 10)
		//s1b0 = s1b0.Mul(s1b0, rawOrder.AmountB)

		s0b1 := new(big.Int).SetBytes(rawOrder.AmountS.Bytes())
		//s0b1 = s0b1.Mul(s0b1, rawOrder.AmountB)
		if s1b0.Cmp(s0b1) > 0 {
			return nil, errors.New("Miner,rateAmountS must less than amountS")
		}
		ratio := new(big.Int).Set(scale)
		ratio.Mul(ratio, s1b0).Div(ratio, s0b1)
		rateRatios = append(rateRatios, ratio)
	}
	return CVSquare(rateRatios, scale), nil
}

func CVSquare(rateRatios []*big.Int, scale *big.Int) *big.Int {
	avg := big.NewInt(0)
	length := big.NewInt(int64(len(rateRatios)))
	length1 := big.NewInt(int64(len(rateRatios) - 1))
	for _, ratio := range rateRatios {
		avg.Add(avg, ratio)
	}
	avg = avg.Div(avg, length)

	cvs := big.NewInt(0)
	for _, ratio := range rateRatios {
		sub := big.NewInt(0)
		sub.Sub(ratio, avg)

		subSquare := new(big.Int).Mul(sub, sub)
		cvs.Add(cvs, subSquare)
	}
	//log.Debugf("CVSquare, scale:%s", scale)
	//log.Debugf("CVSquare, avg:%s", scale)
	//log.Debugf("CVSquare, length1:%s", scale)
	if avg.Sign() <= 0 {
		return new(big.Int).SetInt64(math.MaxInt64)
	}
	//todo:avg may be zero??
	return cvs.Mul(cvs, scale).Div(cvs, avg).Mul(cvs, scale).Div(cvs, avg).Div(cvs, length1)
}

func (e *Evaluator) getLegalCurrency(tokenAddress common.Address, amount *big.Rat) (*big.Rat, error) {
	return e.marketCapProvider.LegalCurrencyValue(tokenAddress, amount)
}

// lgh: 貌似主要是计算给以太坊矿工的gas的多少
// lgh: 从这里可以看出的算法基础是，根据 ring 的环数是 gas 计算算法决定了 gas 最终的实际价格是多少。而和其它无关
func (e *Evaluator) evaluateReceived(ringState *types.Ring) {
	ringState.Received = big.NewRat(int64(0), int64(1)) // 0/1 = 0
	// lgh: 计算油费标准
	ringState.GasPrice = ethaccessor.EstimateGasPrice(e.minGasPrice, e.maxGasPrice)
	//log.Debugf("len(ringState.Orders):%d", len(ringState.Orders))
	ringState.Gas = new(big.Int)

	// gasUsedWithLength 初始化的时候都是 500000，猜测它的含义是，orders 成环的量导致不同的 gas 不同，目前都是 500000 起乘
	ringState.Gas.Set(e.gasUsedWithLength[len(ringState.Orders)])
	protocolCost := new(big.Int)
	protocolCost.Mul(ringState.Gas, ringState.GasPrice) // 500000 * GasPrice

	costEth := new(big.Rat).SetInt(protocolCost) // costEth = 500000 * GasPrice

	// lgh: all
	// eth 的小数点是 18，那么 1 ETH = 10^18
	// 5*10^6 * 10^9 < 5*10^6 * GasPrice < 5*10^6 * 10^11
	// ==> 5*10^14 < 5*10^6 * GasPrice < 5*10^17
	// ==> costEth 目前基于默认的配置文件最大是 0.5 个ETH，最小是 5/10^4 ETH
	// 下面获取 costEth 基于 eth 的情况下，价值多少 USD
	ringState.LegalCost, _ = e.marketCapProvider.LegalCurrencyValueOfEth(costEth) // 当前环 LegalCost gas 的实际价格

	log.Debugf("legalFee:%s, cost:%s, realCostRate:%s, protocolCost:%s, gas:%s, gasPrice:%s", ringState.LegalFee.FloatString(2), ringState.LegalCost.FloatString(2), e.realCostRate.FloatString(2), protocolCost.String(), ringState.Gas.String(), ringState.GasPrice.String())
	ringState.LegalCost.Mul(ringState.LegalCost, e.realCostRate) // todo 按照配置文件，这个就是 0 了啊
	log.Debugf("legalFee:%s, cost:%s, realCostRate:%s", ringState.LegalFee.FloatString(2), ringState.LegalCost.FloatString(2), e.realCostRate.FloatString(2))
	ringState.Received.Sub(ringState.LegalFee, ringState.LegalCost)
	ringState.Received.Mul(ringState.Received, e.walletSplit)
	return
}

// lgh: 计算费用的实例
func NewEvaluator(marketCapProvider marketcap.MarketCapProvider, minerOptions config.MinerOptions) *Evaluator {
	gasUsedMap := make(map[int]*big.Int)
	// lgh: 下面的 没有 0 和 1 的原因是在 evaluateReceived 函数中取下标的时候，是根据订单数来做下标的
	// 自然，订单数不能是 0 和 1
	// 猜测 gasUsedWithLength 的含义是，orders 成环的量导致不同的 gas 不同
	gasUsedMap[2] = big.NewInt(500000)
	//todo:confirm this value
	gasUsedMap[3] = big.NewInt(500000)
	gasUsedMap[4] = big.NewInt(500000)
	e := &Evaluator{
		marketCapProvider: marketCapProvider,
		rateRatioCVSThreshold: minerOptions.RateRatioCVSThreshold,
		gasUsedWithLength: gasUsedMap}
	e.realCostRate = new(big.Rat) // gas 最终价格要乘上的比例
	if int64(minerOptions.Subsidy) >= 1 { // 配置文件是 1.0
		e.realCostRate.SetInt64(int64(0)) // realCostRate = 0
	} else {
		// 1 - Subsidy
		e.realCostRate.SetFloat64(float64(1.0) - minerOptions.Subsidy)
	}
	e.feeReceipt = common.HexToAddress(minerOptions.FeeReceipt)
	e.walletSplit = new(big.Rat)
	e.walletSplit.SetFloat64(minerOptions.WalletSplit)
	e.minGasPrice = big.NewInt(minerOptions.MinGasLimit)
	e.maxGasPrice = big.NewInt(minerOptions.MaxGasLimit)
	return e
}

func (e *Evaluator) SetMatcher(matcher Matcher) {
	e.matcher = matcher
}
