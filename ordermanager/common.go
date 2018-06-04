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

package ordermanager

import (
	"fmt"
	"github.com/Loopring/relay/dao"
	"github.com/Loopring/relay/ethaccessor"
	"github.com/Loopring/relay/market/util"
	"github.com/Loopring/relay/marketcap"
	"github.com/Loopring/relay/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

var dustOrderValue int64

// lgh: 目前新订单到来的情况，blockNumber 是 nil
func newOrderEntity(state *types.OrderState, mc marketcap.MarketCapProvider, blockNumber *big.Int) (*dao.Order, error) {
	blockNumberStr := blockNumberToString(blockNumber) // latest，最新形成的

	state.DealtAmountS = big.NewInt(0)
	state.DealtAmountB = big.NewInt(0)
	state.SplitAmountS = big.NewInt(0)
	state.SplitAmountB = big.NewInt(0)
	state.CancelledAmountB = big.NewInt(0)
	state.CancelledAmountS = big.NewInt(0)

	// lgh: 给订单表上标识，是 买的，还是卖的
	state.RawOrder.Side = util.GetSide(state.RawOrder.TokenS.Hex(), state.RawOrder.TokenB.Hex())

	protocol := state.RawOrder.DelegateAddress
	// lgh: cancelAmount 取消那的部分。dealtAmount 已经处理卖了的那部分。目前返回全是 0
	cancelAmount, dealtAmount, getAmountErr :=
		// lgh: todo 目前这个方法内部是直接返回的
		getCancelledAndDealtAmount(protocol, state.RawOrder.Hash, blockNumberStr)
	if getAmountErr != nil {
		return nil, getAmountErr
	}

	// lgh: 下面的总是 0。
	if state.RawOrder.BuyNoMoreThanAmountB {
		state.DealtAmountB = dealtAmount
		state.CancelledAmountB = cancelAmount
	} else {
		state.DealtAmountS = dealtAmount
		state.CancelledAmountS = cancelAmount
	}

	// check order finished status
	settleOrderStatus(state, mc, ORDER_FROM_FILL) // lgh: 目前这里内部总是直接设置为新订单状态

	if blockNumber == nil {
		state.UpdatedBlock = big.NewInt(0) // 这个
	} else {
		state.UpdatedBlock = blockNumber
	}

	model := &dao.Order{}
	var err error
	// lgh: model.market 的格式是 --> LRC-WETH
	model.Market, err = util.WrapMarketByAddress(state.RawOrder.TokenB.Hex(), state.RawOrder.TokenS.Hex())
	if err != nil {
		return nil, fmt.Errorf("order manager,newOrderEntity error:%s", err.Error())
	}
	model.ConvertDown(state)

	return model, nil
}

// 写入订单状态
type OrderFillOrCancelType string

const (
	ORDER_FROM_FILL   OrderFillOrCancelType = "fill"
	ORDER_FROM_CANCEL OrderFillOrCancelType = "cancel"
)

func settleOrderStatus(state *types.OrderState, mc marketcap.MarketCapProvider, source OrderFillOrCancelType) {
	zero := big.NewInt(0)
	finishAmountS := big.NewInt(0).Add(state.CancelledAmountS, state.DealtAmountS) // 0+0
	totalAmountS := big.NewInt(0).Add(finishAmountS, state.SplitAmountS)// 0+0
	finishAmountB := big.NewInt(0).Add(state.CancelledAmountB, state.DealtAmountB) // 0+0
	totalAmountB := big.NewInt(0).Add(finishAmountB, state.SplitAmountB) // 0+0
	totalAmount := big.NewInt(0).Add(totalAmountS, totalAmountB) // 0+0

	if totalAmount.Cmp(zero) <= 0 {
		state.Status = types.ORDER_NEW // lgh: 直接返回 新订单
		return
	}

	if !isOrderFullFinished(state, mc) {
		state.Status = types.ORDER_PARTIAL
		return
	}

	if source == ORDER_FROM_FILL {
		state.Status = types.ORDER_FINISHED
		return
	}
	if source == ORDER_FROM_CANCEL {
		state.Status = types.ORDER_CANCEL
		return
	}
}

func isOrderFullFinished(state *types.OrderState, mc marketcap.MarketCapProvider) bool {
	remainedAmountS, _ := state.RemainedAmount()
	remainedValue, _ := mc.LegalCurrencyValue(state.RawOrder.TokenS, remainedAmountS)

	return isValueDusted(remainedValue)
}

// 判断cancel的量大于灰尘丁价值，如果是则为cancel，如果不是则为finished
func isOrderCancelled(state *types.OrderState, mc marketcap.MarketCapProvider) bool {
	if state.Status != types.ORDER_CANCEL && state.Status != types.ORDER_FINISHED {
		return false
	}

	var cancelValue *big.Rat
	if state.RawOrder.BuyNoMoreThanAmountB {
		cancelValue, _ = mc.LegalCurrencyValue(state.RawOrder.TokenB, new(big.Rat).SetInt(state.CancelledAmountB))
	} else {
		cancelValue, _ = mc.LegalCurrencyValue(state.RawOrder.TokenS, new(big.Rat).SetInt(state.CancelledAmountS))
	}

	return !isValueDusted(cancelValue)
}

func isValueDusted(value *big.Rat) bool {
	minValue := big.NewInt(dustOrderValue)
	if value == nil || value.Cmp(new(big.Rat).SetInt(minValue)) > 0 {
		return false
	}

	return true
}

func blockNumberToString(blockNumber *big.Int) string {
	var blockNumberStr string
	if blockNumber == nil {
		blockNumberStr = "latest"
	} else {
		blockNumberStr = types.BigintToHex(blockNumber)
	}

	return blockNumberStr
}

func getCancelledAndDealtAmount(protocol common.Address, orderhash common.Hash, blockNumberStr string) (*big.Int, *big.Int, error) {
	// TODO(fuk): 系统暂时只会从gateway接收新订单,而不会有部分成交的订单

	return big.NewInt(0), big.NewInt(0), nil

	var (
		cancelled, cancelOrFilled, dealt *big.Int
		err                              error
	)

	// get order cancelled amount from chain
	if cancelled, err = ethaccessor.GetCancelled(protocol, orderhash, blockNumberStr); err != nil {
		return nil, nil, fmt.Errorf("order manager,handle gateway order,order %s getCancelled error:%s", orderhash.Hex(), err.Error())
	}

	// get order cancelledOrFilled amount from chain
	if cancelOrFilled, err = ethaccessor.GetCancelledOrFilled(protocol, orderhash, blockNumberStr); err != nil {
		return nil, nil, fmt.Errorf("order manager,handle gateway order,order %s getCancelledOrFilled error:%s", orderhash.Hex(), err.Error())
	}

	if cancelOrFilled.Cmp(cancelled) < 0 {
		return nil, nil, fmt.Errorf("order manager,handle gateway order,order %s cancelOrFilledAmount:%s < cancelledAmount:%s", orderhash.Hex(), cancelOrFilled.String(), cancelled.String())
	}

	dealt = big.NewInt(0).Sub(cancelOrFilled, cancelled)

	return cancelled, dealt, nil
}
