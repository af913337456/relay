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

package timing_matcher

import (
	"fmt"
	"github.com/Loopring/relay/ethaccessor"
	"github.com/Loopring/relay/eventemiter"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/miner"
	"github.com/Loopring/relay/ordermanager"
	"github.com/Loopring/relay/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"sort"
)

type Market struct {
	matcher      *TimingMatcher
	om           ordermanager.OrderManager
	protocolImpl *ethaccessor.ProtocolAddress

	TokenA     common.Address
	TokenB     common.Address
	AtoBOrders map[common.Hash]*types.OrderState
	BtoAOrders map[common.Hash]*types.OrderState

	AtoBOrderHashesExcludeNextRound []common.Hash
	BtoAOrderHashesExcludeNextRound []common.Hash
}

func (market *Market) match() {
	// lgh: market.protocolImpl.DelegateAddress 就是配置文件中的 common.protocolImpl.address 去获取对应的信息后设置好的
	market.getOrdersForMatching(market.protocolImpl.DelegateAddress)

	matchedOrderHashes := make(map[common.Hash]bool) //true:fullfilled, false:partfilled
	ringSubmitInfos := []*types.RingSubmitInfo{}
	candidateRingList := CandidateRingList{} // 候选环

	//step 1: evaluate received 接收评估
	for _, a2BOrder := range market.AtoBOrders {
		// lgh: 下面去 redis 中寻找下是否有当前订单失败的计数
		if failedCount, err1 := OrderExecuteFailedCount(a2BOrder.RawOrder.Hash);
			// market.matcher.maxFailedCount = 3
			nil == err1 && failedCount > market.matcher.maxFailedCount {
				// 当前的订单失败计数超过了范围限制，那么 continue 跳过它
			log.Debugf("orderhash:%s has been failed to submit %d times", a2BOrder.RawOrder.Hash.Hex(), failedCount)
			continue
		}
		for _, b2AOrder := range market.BtoAOrders {
			// 同上，这个循环是 b->a 的
			if failedCount, err1 := OrderExecuteFailedCount(b2AOrder.RawOrder.Hash); nil == err1 && failedCount > market.matcher.maxFailedCount {
				log.Debugf("orderhash:%s has been failed to submit %d times", b2AOrder.RawOrder.Hash.Hex(), failedCount)
				continue
			}
			//todo:move ‘a2BOrder.RawOrder.Owner != b2AOrder.RawOrder.Owner’ after contract fix bug
			// a2BOrder.RawOrder.Owner != b2AOrder.RawOrder.Owner 排除掉，找到自己的情况
			// lgh: miner.PriceValid 内部就是判断 s/b * s1/b1 >= 1 这里的就是路印成环规则
			if miner.PriceValid(a2BOrder, b2AOrder) && a2BOrder.RawOrder.Owner != b2AOrder.RawOrder.Owner {
				// lgh: 下面总是 1 和 1，两订单成一环。按照 总的是 2x2 来看，最多就是 4 环
				if candidateRing, err := market.GenerateCandidateRing(a2BOrder, b2AOrder); nil != err {
					log.Errorf("err:%s", err.Error())
					continue
				} else {
					if candidateRing.received.Sign() > 0 { // ----⑪
						// candidateRing.received 由 ringTmp.Received 初始化，是一样的值
						// 环矿工的撮合收益 > 0 的才入选候选环。限制条件之一
						candidateRingList = append(candidateRingList, *candidateRing)
					} else {
						log.Debugf("timing_matchher, market ringForSubmit received not enough, received:%s, cost:%s ", candidateRing.received.FloatString(0), candidateRing.cost.FloatString(0))
					}
				}
			}
		}
	}

	log.Debugf("match round:%s, market: A %s -> B %s , candidateRingList.length:%d", market.matcher.lastRoundNumber, market.TokenA.Hex(), market.TokenB.Hex(), len(candidateRingList))
	//the ring that can get max received
	list := candidateRingList
	for {
		if len(list) <= 0 {
			log.Debugf("match round ===> len(list) <= 0")
			break
		}

		sort.Sort(list)
		candidateRing := list[0]
		list = list[1:] // 提取一个候选，就从 list 中删除掉
		orders := []*types.OrderState{}
		for hash,_ := range candidateRing.filledOrders {
			if o, exists := market.AtoBOrders[hash]; exists {
				// 如果当前的订单是 a->b 的
				orders = append(orders, o)
			} else {
				// 否则就是 b->a 的
				orders = append(orders, market.BtoAOrders[hash])
			}
		}
		// 下面再计算了一次，生成'提交环'
		if ringForSubmit, err := market.generateRingSubmitInfo(orders...); nil != err {
			log.Debugf("generate RingSubmitInfo err:%s", err.Error())
			continue
		} else {
			// lgh: 由下面两句的初始化来看，ringForSubmit.Ringhash 总是 = 配置文件中的矿工的撮合收费地址的 hash，但是在
			// ringState.GenerateHash 内部使用了每个订单信息生成的 hash 值来参与生成 唯一 id 来参与生成 ringState.Hash
			// 所以订单信息不同，ringState.Hash 是不同的
			// ringSubmitInfo.Ringhash = ringState.Hash
			// ringState.Hash = ringState.GenerateHash(submitter.feeReceipt)
			// 第一次是不存在，在下面的 AddMinedRing 会添加记录
			exists, err := CachedMatchedRing(ringForSubmit.Ringhash)
			if  nil != err || exists {
				if nil != err {
					log.Error(err.Error())
				} else {
					log.Errorf("ringhash:%s has been submitted", ringForSubmit.Ringhash.Hex())
				}
				// 相同的环不再提交
				continue
			}

			uniqueId := ringForSubmit.RawRing.GenerateUniqueId()
			// 下面检查该环的提交失败次数是否超过限制。todo 按道理，这里不应该再会走入，因为上面做了 exists 的判断
			if failedCount, err := RingExecuteFailedCount(uniqueId); nil == err && failedCount > market.matcher.maxFailedCount {
				log.Debugf("ringSubmitInfo.UniqueId:%s , ringhash: %s , has been failed to submit %d times", uniqueId.Hex(), ringForSubmit.Ringhash.Hex(), failedCount)
				continue
			}

			//todo:for test, release this limit
			// lgh: todo 为什么又判断了一次？
			if ringForSubmit.RawRing.Received.Sign() > 0 {
				for _, filledOrder := range ringForSubmit.RawRing.Orders {
					orderState := market.reduceAmountAfterFilled(filledOrder)
					isFullFilled := market.om.IsOrderFullFinished(orderState)
					matchedOrderHashes[filledOrder.OrderState.RawOrder.Hash] = isFullFilled
					//market.matcher.rounds.AppendFilledOrderToCurrent(filledOrder, ringForSubmit.RawRing.Hash)

					list = market.reduceReceivedOfCandidateRing(list, filledOrder, isFullFilled)
				}
				AddMinedRing(ringForSubmit)
				ringSubmitInfos = append(ringSubmitInfos, ringForSubmit)
			} else {
				log.Debugf("ring:%s will not be submitted,because of received:%s", ringForSubmit.RawRing.Hash.Hex(), ringForSubmit.RawRing.Received.String())
			}
		}
	}

	for orderHash, _ := range market.AtoBOrders {
		if fullFilled, exists := matchedOrderHashes[orderHash]; exists && fullFilled {
			market.AtoBOrderHashesExcludeNextRound = append(market.AtoBOrderHashesExcludeNextRound, orderHash)
		}
	}

	for orderHash, _ := range market.BtoAOrders {
		if fullFilled, exists := matchedOrderHashes[orderHash]; exists && fullFilled {
			market.BtoAOrderHashesExcludeNextRound = append(market.BtoAOrderHashesExcludeNextRound, orderHash)
		}
	}
	if len(ringSubmitInfos) > 0 {
		log.Debugf("形成新环 : TokenA %s -> TokenB %s，分发 Miner_NewRing 事件",market.TokenA.Hex(), market.TokenB.Hex())
		eventemitter.Emit(eventemitter.Miner_NewRing, ringSubmitInfos)
	}else{
		log.Debugf("不足以形成新环 len(ringSubmitInfos) <= 0")
	}
}

func (market *Market) reduceReceivedOfCandidateRing(list CandidateRingList, filledOrder *types.FilledOrder, isFullFilled bool) CandidateRingList {
	resList := CandidateRingList{}
	hash := filledOrder.OrderState.RawOrder.Hash
	for _, ring := range list {
		if amountS, exists := ring.filledOrders[hash]; exists {
			if isFullFilled {
				continue
			}
			availableAmountS := new(big.Rat)
			availableAmountS.Sub(filledOrder.AvailableAmountS, filledOrder.FillAmountS)
			if availableAmountS.Sign() > 0 {
				var remainedAmountS *big.Rat
				if amountS.Cmp(availableAmountS) >= 0 {
					remainedAmountS = availableAmountS
				} else {
					remainedAmountS = amountS
				}
				log.Debugf("reduceReceivedOfCandidateRing, filledOrder.availableAmountS:%s, filledOrder.FillAmountS:%s, amountS:%s", filledOrder.AvailableAmountS.FloatString(3), filledOrder.FillAmountS.FloatString(3), amountS.FloatString(3))
				rate := new(big.Rat)
				rate.Quo(remainedAmountS, amountS)
				remainedReceived := new(big.Rat).Add(ring.received, ring.cost)
				remainedReceived.Mul(remainedReceived, rate).Sub(remainedReceived, ring.cost)
				//todo:
				if remainedReceived.Sign() <= 0 {
					continue
				}
				for hash, amount := range ring.filledOrders {
					ring.filledOrders[hash] = amount.Mul(amount, rate)
				}
				resList = append(resList, ring)
			}
		} else {
			resList = append(resList, ring)
		}
	}
	return resList
}

/**
get orders from ordermanager
*/
func (market *Market) getOrdersForMatching(delegateAddress common.Address) {
	market.AtoBOrders = make(map[common.Hash]*types.OrderState)
	market.BtoAOrders = make(map[common.Hash]*types.OrderState)

	// log.Debugf("timing matcher,market tokenA:%s, tokenB:%s, atob hash length:%d, btoa hash length:%d", market.TokenA.Hex(), market.TokenB.Hex(), len(market.AtoBOrderHashesExcludeNextRound), len(market.BtoAOrderHashesExcludeNextRound))
	currentRoundNumber := market.matcher.lastRoundNumber.Int64() // lgh: 一个毫秒级别的时间戳
	deleyedNumber := market.matcher.delayedNumber + currentRoundNumber

	atoBOrders := market.om.MinerOrders(
		delegateAddress, // lgh: 配置文件的地址
		market.TokenA, // lgh: AllTokenPairs 的 tokenS
		market.TokenB, // lgh: AllTokenPairs 的 tokenB
		// lgh: 目前看来 roundOrderCount 是要取的条数，从数据库中获取订单
		market.matcher.roundOrderCount, // 配置文件中的 roundOrderCount，默认是 2
		market.matcher.reservedTime, // 保留的提交时间，默认是 45，单位未知
		int64(0),
		currentRoundNumber, // 开始进入循环时候的当前的毫秒级别的时间戳
		// lgh: AtoBOrderHashesExcludeNextRound 一开始是空切片。应该是用来过滤某些订单的
		// deleyedNumber 是开始时候的毫秒数 + 10000，就是多了10秒
		&types.OrderDelayList{OrderHash: market.AtoBOrderHashesExcludeNextRound, DelayedCount: deleyedNumber})

	// 如果：len(atoBOrders) = 1，market.matcher.roundOrderCount - len(atoBOrders) = 1
	if len(atoBOrders) < market.matcher.roundOrderCount {
		orderCount := market.matcher.roundOrderCount - len(atoBOrders)
		orders := market.om.MinerOrders(
			delegateAddress,
			market.TokenA,
			market.TokenB,
			orderCount,  // 1
			market.matcher.reservedTime, // 45
			currentRoundNumber+1, // 加了一个毫秒
			// 因为前一次搜索的是 0 < x <= currentRoundNumber 的订单，发现条数不够，所以现在改为
			// currentRoundNumber+1 < x <= currentRoundNumber+10s
			currentRoundNumber+market.matcher.delayedNumber) // 开始时候的毫秒数 + 10000，就是多了10秒

		atoBOrders = append(atoBOrders, orders...)
	}
	// lgh: 上面最多就是 orderCount 条。下面的 bToa 是一样的

	btoAOrders := market.om.MinerOrders(delegateAddress, market.TokenB, market.TokenA, market.matcher.roundOrderCount, market.matcher.reservedTime, int64(0), currentRoundNumber, &types.OrderDelayList{OrderHash: market.BtoAOrderHashesExcludeNextRound, DelayedCount: deleyedNumber})
	if len(btoAOrders) < market.matcher.roundOrderCount {
		orderCount := market.matcher.roundOrderCount - len(btoAOrders)
		orders := market.om.MinerOrders(delegateAddress, market.TokenB, market.TokenA, orderCount, market.matcher.reservedTime, currentRoundNumber+1, currentRoundNumber+market.matcher.delayedNumber)
		btoAOrders = append(btoAOrders, orders...)
	}

	//log.Debugf("#### %s,%s %d,%d %d",market.TokenA.Hex(),market.TokenB.Hex(), len(atoBOrders), len(btoAOrders),market.matcher.roundOrderCount)
	market.AtoBOrderHashesExcludeNextRound = []common.Hash{}
	market.BtoAOrderHashesExcludeNextRound = []common.Hash{}

	for _, order := range atoBOrders {
		// lgh: 统计当前订单在之前的环中已经买卖了的量
		market.reduceRemainedAmountBeforeMatch(order)
		if !market.om.IsOrderFullFinished(order) {
			market.AtoBOrders[order.RawOrder.Hash] = order
		} else {
			market.AtoBOrderHashesExcludeNextRound = append(market.AtoBOrderHashesExcludeNextRound, order.RawOrder.Hash)
		}
		log.Debugf("order status in this new round:%s, orderhash:%s, DealtAmountS:%s, ", market.matcher.lastRoundNumber.String(), order.RawOrder.Hash.Hex(), order.DealtAmountS.String())
	}

	for _, order := range btoAOrders {
		market.reduceRemainedAmountBeforeMatch(order)
		if !market.om.IsOrderFullFinished(order) {
			market.BtoAOrders[order.RawOrder.Hash] = order
		} else {
			market.BtoAOrderHashesExcludeNextRound = append(market.BtoAOrderHashesExcludeNextRound, order.RawOrder.Hash)
		}
		log.Debugf("order status in this new round:%s, orderhash:%s, DealtAmountS:%s", market.matcher.lastRoundNumber.String(), order.RawOrder.Hash.Hex(), order.DealtAmountS.String())
	}
}

//sub the matched amount in new round.
func (market *Market) reduceRemainedAmountBeforeMatch(orderState *types.OrderState) {
	orderHash := orderState.RawOrder.Hash

	// lgh: 找出当前的订单 order 在参与之前的环中，已经买卖了多少的 总量
	if amountS, amountB, err := DealtAmount(orderHash); nil != err {
		log.Errorf("err:%s", err.Error())
	} else {
		log.Debugf("reduceRemainedAmountBeforeMatch:%s, %s, %s", orderState.RawOrder.Owner.Hex(), amountS.String(), amountB.String())
		orderState.DealtAmountB.Add(orderState.DealtAmountB, ratToInt(amountB))
		orderState.DealtAmountS.Add(orderState.DealtAmountS, ratToInt(amountS))
	}
}

func (market *Market) reduceAmountAfterFilled(filledOrder *types.FilledOrder) *types.OrderState {
	filledOrderState := filledOrder.OrderState
	var orderState *types.OrderState

	//only one of DealtAmountB and DealtAmountS is precise
	if filledOrderState.RawOrder.TokenS == market.TokenA {
		orderState = market.AtoBOrders[filledOrderState.RawOrder.Hash]
		orderState.DealtAmountB.Add(orderState.DealtAmountB, ratToInt(filledOrder.FillAmountB))
		orderState.DealtAmountS.Add(orderState.DealtAmountS, ratToInt(filledOrder.FillAmountS))
	} else {
		orderState = market.BtoAOrders[filledOrderState.RawOrder.Hash]
		orderState.DealtAmountB.Add(orderState.DealtAmountB, ratToInt(filledOrder.FillAmountB))
		orderState.DealtAmountS.Add(orderState.DealtAmountS, ratToInt(filledOrder.FillAmountS))
	}
	log.Debugf("order status after matched, orderhash:%s,filledAmountS:%s, DealtAmountS:%s, ", orderState.RawOrder.Hash.Hex(), filledOrder.FillAmountS.String(), orderState.DealtAmountS.String())
	//reduced account balance

	return orderState
}

// lgh: 撮合的时候，进入这里的 orders 总是 2，且分别是 a->b，b->a
func (market *Market) GenerateCandidateRing(orders ...*types.OrderState) (*CandidateRing, error) {
	filledOrders := []*types.FilledOrder{}
	//miner will received nothing, if miner set FeeSelection=1 and he doesn't have enough lrc
	for _, order := range orders { // size == 2
		if filledOrder, err := market.generateFilledOrder(order); nil != err {
			log.Errorf("err:%s", err.Error())
			return nil, err
		} else {
			filledOrders = append(filledOrders, filledOrder)
		}
	}
	if len(filledOrders) <= 0 {
		return nil,fmt.Errorf("GenerateCandidateRing orders is empty")
	}
	ringTmp := miner.NewRing(filledOrders) // 创建一个环
	// lgh: ComputeRing
	if err := market.matcher.evaluator.ComputeRing(ringTmp); nil != err {
		return nil, err
	} else {
		candidateRing := &CandidateRing{
			cost: ringTmp.LegalCost, // 当前的环矿工应该承担多少 以太坊的 油费
			received: ringTmp.Received, // 当前的环矿工最终的收益
			filledOrders: make(map[common.Hash]*big.Rat)} // 存储每个订单的 hash 值并使之对应到 真实要卖的

		for _, filledOrder := range ringTmp.Orders {
			log.Debugf("match, orderhash:%s, filledOrder.FilledAmountS:%s", filledOrder.OrderState.RawOrder.Hash.Hex(), filledOrder.FillAmountS.FloatString(3))
			// lgh: 做了个赋值 每个订单的 hash 值并使之对应到 真实要卖的
			candidateRing.filledOrders[filledOrder.OrderState.RawOrder.Hash]= filledOrder.FillAmountS
		}
		return candidateRing, nil
	}
}

func (market *Market) generateFilledOrder(order *types.OrderState) (*types.FilledOrder, error) {

	// lgh: 获取用户的 lrc 代币余额
	lrcTokenBalance, err :=
		market.matcher.GetAccountAvailableAmount(
		order.RawOrder.Owner,
		market.protocolImpl.LrcTokenAddress,
		market.protocolImpl.DelegateAddress)

	if nil != err {
		return nil, err
	}

	// lgh: 获取用户的 tokenS 代币余额
	tokenSBalance, err :=
		market.matcher.GetAccountAvailableAmount(
			order.RawOrder.Owner,
			order.RawOrder.TokenS,
			market.protocolImpl.DelegateAddress)

	if nil != err {
		return nil, err
	}
	if tokenSBalance.Sign() <= 0 {
		// lgh: rag.Sign 函数是用来判断 x 的
		return nil, fmt.Errorf("owner:%s token:%s balance or allowance is zero", order.RawOrder.Owner.Hex(), order.RawOrder.TokenS.Hex())
	}
	//todo:
	// lgh: tokenSBalance 并没有在 IsValueDusted 内部被修改
	// lgh: IsValueDusted 主要是计算出 tokenSBalance 对应的总市值，即价值多少。再判断它的市值是否合理
	if market.om.IsValueDusted(
		order.RawOrder.TokenS, tokenSBalance) {
		return nil, fmt.Errorf("owner:%s token:%s balance or allowance is not enough", order.RawOrder.Owner.Hex(), order.RawOrder.TokenS.Hex())
	}
	// lgh: 下面主要计算出当前订单剩下的要买卖的代币数量
	return types.ConvertOrderStateToFilledOrder(
		*order,
		lrcTokenBalance,
		tokenSBalance, // 所以这里应该不是乘上汇率后的值
		market.protocolImpl.LrcTokenAddress),
		nil
}

// lgh: 这里面的操作和 GenerateCandidateRing 方法的差不多。最终生成提交环的信息
func (market *Market) generateRingSubmitInfo(orders ...*types.OrderState) (*types.RingSubmitInfo, error) {
	filledOrders := []*types.FilledOrder{}
	//miner will received nothing, if miner set FeeSelection=1 and he doesn't have enough lrc
	for _, order := range orders {
		if filledOrder, err := market.generateFilledOrder(order); nil != err {
			log.Errorf("err:%s", err.Error())
			return nil, err
		} else {
			filledOrders = append(filledOrders, filledOrder)
		}
	}

	ringTmp := miner.NewRing(filledOrders)
	if err := market.matcher.evaluator.ComputeRing(ringTmp); nil != err {
		return nil, err
	} else {
		res, err := market.matcher.submitter.GenerateRingSubmitInfo(ringTmp)
		return res, err
	}
}

func NewMarket(protocolAddress *ethaccessor.ProtocolAddress, tokenS, tokenB common.Address, matcher *TimingMatcher, om ordermanager.OrderManager) *Market {

	m := &Market{}
	m.om = om
	m.protocolImpl = protocolAddress
	m.matcher = matcher
	m.TokenA = tokenS
	m.TokenB = tokenB
	m.AtoBOrderHashesExcludeNextRound = []common.Hash{}
	m.BtoAOrderHashesExcludeNextRound = []common.Hash{}
	return m
}

func ratToInt(rat *big.Rat) *big.Int {
	return new(big.Int).Div(rat.Num(), rat.Denom())
}
