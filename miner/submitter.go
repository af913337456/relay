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
	"math/big"

	"encoding/json"
	"github.com/Loopring/relay/cache"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/dao"
	"github.com/Loopring/relay/ethaccessor"
	"github.com/Loopring/relay/eventemiter"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/marketcap"
	"github.com/Loopring/relay/types"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"strconv"
	"time"
)

const SubmitRingMethod_LastId = "submitringmethod_lastid"

//保存ring，并将ring发送到区块链，同样需要分为待完成和已完成
type RingSubmitter struct {
	minerAccountForSign accounts.Account
	//minerNameInfos      map[common.Address][]*types.NameRegistryInfo
	feeReceipt       common.Address
	currentBlockTime int64

	maxGasLimit *big.Int
	minGasLimit *big.Int

	normalMinerAddresses  []*NormalSenderAddress
	percentMinerAddresses []*SplitMinerAddress

	dbService         dao.RdsService
	marketCapProvider marketcap.MarketCapProvider
	matcher           Matcher

	stopFuncs []func()
}

type RingSubmitFailed struct {
	RingState *types.Ring
	err       error
}

func NewSubmitter(options config.MinerOptions, dbService dao.RdsService, marketCapProvider marketcap.MarketCapProvider) (*RingSubmitter, error) {
	// lgh: 环提交者
	submitter := &RingSubmitter{}
	// lgh: 初始化油费的收款范围
	submitter.maxGasLimit = big.NewInt(options.MaxGasLimit)
	submitter.minGasLimit = big.NewInt(options.MinGasLimit)
	if common.IsHexAddress(options.FeeReceipt) {
		// todo lgh: 提交者的收费人地址？
		// lgh: 解决上面的 todo FeeReceipt 是矿工的撮合收费地址
		submitter.feeReceipt = common.HexToAddress(options.FeeReceipt)
	} else {
		return submitter, errors.New("miner.feeReceipt must be a address")
	}

	// lgh: 下面初始化矿工账号 和 检验有效性
	for _, addr := range options.NormalMiners {
		var nonce types.Big
		normalAddr := common.HexToAddress(addr.Address)
		/* lgh: 以太坊方法 GetTransactionCount，根据区块号码blockNumber，矿工地址address，
		去查询当前区块的nonce数值，又称当前交易的nonce。
		有三个特殊的区块号码：
		1，创世块 earliest
		2，最近刚出的最新块 latest
		3，等待被区块链确认状态（pending)
		*/
		// lgh: 获取正在被确认的区块 nonce 号，防止传输错误
		// 相关解析文章：https://blog.csdn.net/wo541075754/article/details/78081478?locationNum=3&fps=1
		if err := ethaccessor.GetTransactionCount(&nonce, normalAddr, "pending"); nil != err {
			log.Errorf("err:%s", err.Error())
		}
		// lgh: 初始化矿工实体，根据设置后的矿工账号
		miner := &NormalSenderAddress{}
		miner.Address = normalAddr
		miner.GasPriceLimit = big.NewInt(addr.GasPriceLimit)
		miner.MaxPendingCount = addr.MaxPendingCount // lgh: 在区块等待确认状态下，能允许miner再次发送tx。直到最大的等待计数块超过 MaxPendingCount
		miner.MaxPendingTtl = addr.MaxPendingTtl
		miner.Nonce = nonce.BigInt() // lgh: 告知当前矿工正在交易的 nonce 数值，防止后续的交易设置nonce出错
		submitter.normalMinerAddresses = append(submitter.normalMinerAddresses, miner)
	}

	// lgh: PercentMiners 按百分比计费的矿工?
	for _, addr := range options.PercentMiners {
		var nonce types.Big
		normalAddr := common.HexToAddress(addr.Address)
		if err := ethaccessor.GetTransactionCount(&nonce, normalAddr, "pending"); nil != err {
			log.Errorf("err:%s", err.Error())
		}
		miner := &SplitMinerAddress{}
		miner.Nonce = nonce.BigInt()
		miner.Address = normalAddr
		miner.FeePercent = addr.FeePercent
		miner.StartFee = addr.StartFee
		submitter.percentMinerAddresses = append(submitter.percentMinerAddresses, miner)
	}

	submitter.dbService = dbService
	submitter.marketCapProvider = marketCapProvider

	submitter.stopFuncs = []func(){}
	return submitter, nil
}

func (submitter *RingSubmitter) listenBlockNew() {
	blockEventChan := make(chan *types.BlockEvent)
	go func() {
		for {
			select {
			case blockEvent := <-blockEventChan:
				submitter.currentBlockTime = blockEvent.BlockTime
			}
		}
	}()

	watcher := &eventemitter.Watcher{
		Concurrent: false,
		Handle: func(eventData eventemitter.EventData) error {
			e := eventData.(*types.BlockEvent)
			log.Debugf("submitter.listenBlockNew blockNumber:%s, blocktime:%d", e.BlockNumber.String(), e.BlockTime)
			blockEventChan <- e
			return nil
		},
	}
	eventemitter.On(eventemitter.Block_New, watcher)
	submitter.stopFuncs = append(submitter.stopFuncs, func() {
		close(blockEventChan)
		eventemitter.Un(eventemitter.Block_New, watcher)
	})
}

func (submitter *RingSubmitter) listenNewRings() {
	//ringSubmitInfoChan := make(chan []*types.RingSubmitInfo)
	//go func() {
	//	for {
	//		select {
	//		case ringInfos := <-ringSubmitInfoChan:
	//			if nil != ringInfos {
	//				for _, ringState := range ringInfos {
	//					txHash, status, err1 := submitter.submitRing(ringState)
	//					ringState.SubmitTxHash = txHash
	//
	//					daoInfo := &dao.RingSubmitInfo{}
	//					daoInfo.ConvertDown(ringState, err1)
	//					if err := submitter.dbService.Add(daoInfo); nil != err {
	//						log.Errorf("Miner submitter,insert new ring err:%s", err.Error())
	//					} else {
	//						for _, filledOrder := range ringState.RawRing.Orders {
	//							daoOrder := &dao.FilledOrder{}
	//							daoOrder.ConvertDown(filledOrder, ringState.Ringhash)
	//							if err1 := submitter.dbService.Add(daoOrder); nil != err1 {
	//								log.Errorf("Miner submitter,insert filled Order err:%s", err1.Error())
	//							}
	//						}
	//					}
	//					submitter.submitResult(ringState.Ringhash, ringState.RawRing.GenerateUniqueId(), txHash, status, big.NewInt(0), big.NewInt(0), big.NewInt(0), err1)
	//				}
	//			}
	//		}
	//	}
	//}()
	watcher := &eventemitter.Watcher{
		Concurrent: false,
		Handle: func(eventData eventemitter.EventData) error {
			ringInfos := eventData.([]*types.RingSubmitInfo)
			log.Debugf("received ringstates length:%d", len(ringInfos))
			//ringSubmitInfoChan <- e
			if nil != ringInfos {
				for _, ringState := range ringInfos {
					txHash, status, err1 := submitter.submitRing(ringState)
					ringState.SubmitTxHash = txHash

					daoInfo := &dao.RingSubmitInfo{}
					daoInfo.ConvertDown(ringState, err1)
					if err := submitter.dbService.Add(daoInfo); nil != err {
						log.Errorf("Miner submitter,insert new ring err:%s", err.Error())
					} else {
						for _, filledOrder := range ringState.RawRing.Orders {
							daoOrder := &dao.FilledOrder{}
							daoOrder.ConvertDown(filledOrder, ringState.Ringhash)
							if err1 := submitter.dbService.Add(daoOrder); nil != err1 {
								log.Errorf("Miner submitter,insert filled Order err:%s", err1.Error())
							}
						}
					}
					submitter.submitResult(ringState.Ringhash, ringState.RawRing.GenerateUniqueId(), txHash, status, big.NewInt(0), big.NewInt(0), big.NewInt(0), err1)
				}
			}
			return nil
		},
	}
	eventemitter.On(eventemitter.Miner_NewRing, watcher)
	submitter.stopFuncs = append(submitter.stopFuncs, func() {
		//close(ringSubmitInfoChan)
		eventemitter.Un(eventemitter.Miner_NewRing, watcher)
	})
}

//todo: 不在submit中的才会提交
func (submitter *RingSubmitter) canSubmit(ringState *types.RingSubmitInfo) error {
	return errors.New("had been processed")
}

func (submitter *RingSubmitter) submitRing(ringSubmitInfo *types.RingSubmitInfo) (common.Hash, types.TxStatus, error) {
	status := types.TX_STATUS_PENDING
	ordersStr, _ := json.Marshal(ringSubmitInfo.RawRing.Orders)
	log.Debugf("submitring hash:%s, orders:%s", ringSubmitInfo.Ringhash.Hex(), string(ordersStr))

	txHash := types.NilHash
	var err error

	if nil == err {
		txHashStr := "0x"
		txHashStr, err = ethaccessor.SignAndSendTransaction(
			ringSubmitInfo.Miner,
			ringSubmitInfo.ProtocolAddress,
			ringSubmitInfo.ProtocolGas,
			ringSubmitInfo.ProtocolGasPrice, nil, ringSubmitInfo.ProtocolData, false)
		if nil != err {
			log.Errorf("submitring hash:%s, err:%s", ringSubmitInfo.Ringhash.Hex(), err.Error())
			status = types.TX_STATUS_FAILED
		}
		txHash = common.HexToHash(txHashStr)
	} else {
		log.Errorf("submitring hash:%s, protocol:%s, err:%s", ringSubmitInfo.Ringhash.Hex(), ringSubmitInfo.ProtocolAddress.Hex(), err.Error())
		status = types.TX_STATUS_FAILED
	}

	return txHash, status, err
}

func (submitter *RingSubmitter) listenSubmitRingMethodEventFromMysql() {

	processSubmitRingMethod := func() {
		lastId := int(0)

		if exists, err := cache.Exists(SubmitRingMethod_LastId); exists && nil == err {
			if idBytes, err := cache.Get(SubmitRingMethod_LastId); nil == err {
				if len(idBytes) > 0 {
					var err1 error
					if lastId, err1 = strconv.Atoi(string(idBytes)); nil != err1 {
						log.Errorf("err:%s", err1.Error())
					}
				}
			} else {
				log.Errorf("err:%s", err.Error())
			}
		}

		if methodEvents, err2 := submitter.dbService.GetRingminedMethods(lastId, 500); nil == err2 {
			for _, daoEvt := range methodEvents {
				if lastId < daoEvt.ID {
					lastId = daoEvt.ID
				}
				evt := &types.RingMinedEvent{}
				if err3 := daoEvt.ConvertUp(evt); nil == err3 {
					if infos, err := submitter.dbService.GetRingHashesByTxHash(evt.TxHash); nil != err {
						log.Errorf("err:%s", err.Error())
					} else {
						var err1 error
						if nil != evt.Err {
							err1 = evt.Err
						} else {
							err1 = errors.New("")
						}

						for _, info := range infos {
							ringhash := common.HexToHash(info.RingHash)
							uniqueId := common.HexToHash(info.UniqueId)

							submitter.submitResult(ringhash, uniqueId, evt.TxHash, evt.Status, big.NewInt(0), evt.BlockNumber, evt.GasUsed, err1)
						}
					}
				} else {
					log.Errorf("err:%s", err3.Error())
				}
			}
		} else {
			log.Errorf("err:%s", err2.Error())
		}

		cache.Set(SubmitRingMethod_LastId, []byte(strconv.Itoa(lastId)), int64(0))
	}
	go func() {
		processSubmitRingMethod()
		for {
			select {
			case <-time.After(5 * time.Second):
				processSubmitRingMethod()
			}
		}
	}()

}

//func (submitter *RingSubmitter) listenSubmitRingMethodEvent() {
//	submitRingMethodChan := make(chan *types.SubmitRingMethodEvent)
//	go func() {
//		for {
//			select {
//			case event := <-submitRingMethodChan:
//				if nil != event {
//					//if event.Status == types.TX_STATUS_FAILED {
//					if ringhashes, err := submitter.dbService.GetRingHashesByTxHash(event.TxHash); nil != err {
//						log.Errorf("err:%s", err.Error())
//					} else {
//						var err1 error
//						if nil != event.Err {
//							err1 = errors.New("failed to execute ring:" + event.Err.Error())
//						} else {
//							err1 = errors.New("success")
//						}
//
//						for _, ringhash := range ringhashes {
//							submitter.submitResult(ringhash, event.TxHash, event.Status, big.NewInt(0), event.BlockNumber, event.GasUsed, err1)
//						}
//					}
//					//}
//				}
//			}
//		}
//	}()
//
//	watcher := &eventemitter.Watcher{
//		Concurrent: false,
//		Handle: func(eventData eventemitter.EventData) error {
//			e := eventData.(*types.SubmitRingMethodEvent)
//			log.Debugf("eventemitter.Watchereventemitter.Watcher:%s", e.TxHash.Hex())
//			submitRingMethodChan <- e
//			return nil
//		},
//	}
//	eventemitter.On(eventemitter.Miner_SubmitRing_Method, watcher)
//	submitter.stopFuncs = append(submitter.stopFuncs, func() {
//		close(submitRingMethodChan)
//		eventemitter.Un(eventemitter.Miner_SubmitRing_Method, watcher)
//	})
//}

func (submitter *RingSubmitter) submitResult(ringhash, uniqeId, txhash common.Hash, status types.TxStatus, ringIndex, blockNumber, usedGas *big.Int, err error) {
	resultEvt := &types.RingSubmitResultEvent{
		RingHash:     ringhash,
		RingUniqueId: uniqeId,
		TxHash:       txhash,
		Status:       status,
		Err:          err,
		RingIndex:    ringIndex,
		BlockNumber:  blockNumber,
		UsedGas:      usedGas,
	}
	if err := submitter.dbService.UpdateRingSubmitInfoResult(resultEvt); nil != err {
		log.Errorf("err:%s", err.Error())
	}
	eventemitter.Emit(eventemitter.Miner_RingSubmitResult, resultEvt)
}

////提交错误，执行错误
//func (submitter *RingSubmitter) submitFailed(ringhashes []common.Hash, err error) {
//	if err := submitter.dbService.UpdateRingSubmitInfoFailed(ringhashes, err.Error()); nil != err {
//		log.Errorf("err:%s", err.Error())
//	} else {
//		for _, ringhash := range ringhashes {
//			failedEvent := &types.RingSubmitResultEvent{RingHash: ringhash, Status:types.TX_STATUS_FAILED}
//			eventemitter.Emit(eventemitter.Miner_RingSubmitResult, failedEvent)
//		}
//	}
//}

func (submitter *RingSubmitter) GenerateRingSubmitInfo(ringState *types.Ring) (*types.RingSubmitInfo, error) {
	//todo:change to advice protocolAddress
	protocolAddress := ringState.Orders[0].OrderState.RawOrder.Protocol // 每个订单的这个应该是一样的，所以这里直接取第一个的
	//var (
	////signer *types.NameRegistryInfo
	////err error
	//)

	ringSubmitInfo := &types.RingSubmitInfo{
		RawRing: ringState,
		ProtocolGasPrice: ringState.GasPrice,
		ProtocolGas: ringState.Gas}

	if types.IsZeroHash(ringState.Hash) {
		// GenerateHash 地址转 hash
		ringState.Hash = ringState.GenerateHash(submitter.feeReceipt) // 使用矿工的撮合收费地址初始化该 hash
		// 矿工可以有多个环提交地址
	}

	ringSubmitInfo.ProtocolAddress = protocolAddress // 文档提到：Loopring合约地址，伴随着合约升级，地址是不同版本的
	ringSubmitInfo.OrdersCount = big.NewInt(int64(len(ringState.Orders))) // 订单总数
	ringSubmitInfo.Ringhash = ringState.Hash

	protocolAbi := ethaccessor.ProtocolImplAbi() // 由 commonOptions.ProtocolImpl.ImplAbi 初始化

	// lgh: 目前 selectSenderAddress 总是直接返回下标是 0 的提交地址
	if senderAddress, err := submitter.selectSenderAddress(); nil != err {
		return ringSubmitInfo, err
	} else {
		ringSubmitInfo.Miner = senderAddress
	}
	//submitter.computeReceivedAndSelectMiner(ringSubmitInfo)
	if protocolData, err :=
		ethaccessor.GenerateSubmitRingMethodInputsData(ringState, submitter.feeReceipt, protocolAbi); nil != err {
		return nil, err
	} else {
		ringSubmitInfo.ProtocolData = protocolData
	}
	//预先判断是否会提交成功
	lastTime := ringSubmitInfo.RawRing.ValidSinceTime()
	if submitter.currentBlockTime > 0 && lastTime <= submitter.currentBlockTime {
		var err error
		_, _, err = ethaccessor.EstimateGas(ringSubmitInfo.ProtocolData, ringSubmitInfo.ProtocolAddress, "latest")
		//ringSubmitInfo.ProtocolGas, ringSubmitInfo.ProtocolGasPrice, err = ethaccessor.EstimateGas(ringSubmitInfo.ProtocolData, protocolAddress, "latest")
		if nil != err {
			log.Errorf("can't generate ring ,err:%s", err.Error())
			return nil,err
		}
	}

	//if nil != err {
	//	return nil, err
	//}
	if submitter.maxGasLimit.Sign() > 0 && ringSubmitInfo.ProtocolGas.Cmp(submitter.maxGasLimit) > 0 {
		ringSubmitInfo.ProtocolGas.Set(submitter.maxGasLimit)
	}
	if submitter.minGasLimit.Sign() > 0 && ringSubmitInfo.ProtocolGas.Cmp(submitter.minGasLimit) < 0 {
		ringSubmitInfo.ProtocolGas.Set(submitter.minGasLimit)
	}
	return ringSubmitInfo, nil
}

func (submitter *RingSubmitter) stop() {
	for _, stop := range submitter.stopFuncs {
		stop()
	}
}

func (submitter *RingSubmitter) start() {
	submitter.listenNewRings()
	submitter.listenSubmitRingMethodEventFromMysql()
	submitter.listenBlockNew()
	//submitter.listenSubmitRingMethodEvent()
}

// lgh: 下面这个方法就是所说的 矿工 的环提交地址。具备候选条件
// lgh: 下面的条件是: 区块等待状态中的数 <= 区块等待状态中的最大数
func (submitter *RingSubmitter) availableSenderAddresses() []*NormalSenderAddress {
	senderAddresses := []*NormalSenderAddress{}
	for _, minerAddress := range submitter.normalMinerAddresses {
		// 配置文件中 normalMinerAddresses 目前只有一个，可以多个
		var blockedTxCount, txCount types.Big
		//todo:change it by event

		// 下面获取当前的提交地址 Address 在以太坊截止目前提交了多少 block 数量
		ethaccessor.GetTransactionCount(
			&blockedTxCount,
			minerAddress.Address, "latest")

		// 下面获取当前的提交地址 Address 在以太坊截止目前提交了多少
		// 正处于等待被处理的block 数量
		ethaccessor.GetTransactionCount(
			&txCount,
			minerAddress.Address, "pending")

		//submitter.Accessor.Call("latest", &blockedTxCount, "eth_getTransactionCount", minerAddress.Address.Hex(), "latest")
		//submitter.Accessor.Call("latest", &txCount, "eth_getTransactionCount", minerAddress.Address.Hex(), "pending")

		//todo:check ethbalance
		pendingCount := big.NewInt(int64(0))

		// 下面 pendingCount = txCount - blockedTxCount，要求 txCount > blockedTxCount
		pendingCount.Sub(txCount.BigInt(), blockedTxCount.BigInt())
		if pendingCount.Int64() <= minerAddress.MaxPendingCount {
			// MaxPendingCount 是区块等待状态中的最大数，在 NewSubmitter 的时候初始化一次
			// pendingCount <= MaxPendingCount，还没超出范围，添加入发送者地址。这里有可能 0<= MaxPendingCount
			senderAddresses = append(senderAddresses, minerAddress)
		}
	}
	if len(senderAddresses) <= 0 {
		// lgh todo 是否保持？
		// lgh: 这里强制设为 第一个，即使它目前提交的 pending block 数 > MaxPendingCount
		senderAddresses = append(senderAddresses, submitter.normalMinerAddresses[0])
	}
	return senderAddresses
}

func (submitter *RingSubmitter) selectSenderAddress() (common.Address, error) {
	senderAddresses := submitter.availableSenderAddresses()
	if len(senderAddresses) <= 0 {
		// lgh: 下面这里不会进入 availableSenderAddresses 做了强制设置
		return types.NilAddress, errors.New("there isn't an available sender address")
	} else {
		// lgh: todo availableSenderAddresses 内部是批量获取，下面却直接返回下标是 0 的，不需要做个散列算法来选择地址？
		return senderAddresses[0].Address, nil
	}
}



















