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
	"encoding/json"
	"github.com/Loopring/relay/cache"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

//orderhash ringhash matchedstate
//owner tokenS orderhash
//ringhash orderhash+owner+tokens
//round orderhash
const (
	OrderHashPrefix          = "matcher_orderhash_"
	OwnerPrefix              = "matcher_owner_"
	RingHashPrefix           = "matcher_ringhash_"
	RoundPrefix              = "matcher_round_"
	FailedRingPrefix         = "failed_ring_"
	FailedOrderPrefix        = "failed_order_"
	RinghashToUniqueIdPrefix = "ringhash_uniqid_"
	cacheTtl                 = 86400 * 2
)

type OrderMatchedState struct {
	//ringHash      common.Hash `json:"ringhash"`
	FilledAmountS *types.Rat `json:"filled_amount_s"`
	FilledAmountB *types.Rat `json:"filled_amount_b"`
}

type ringCache struct {
	ringhash common.Hash
}

func (c ringCache) cacheKey() string {
	return RingHashPrefix + strings.ToLower(c.ringhash.Hex())
}

func (c ringCache) cacheFiled(orderhash common.Hash, owner, token common.Address) []byte {
	return append(append(orderhash.Bytes(), owner.Bytes()...), token.Bytes()...)
}

func (c ringCache) parseFiled(data []byte) (orderhash common.Hash, owner, token common.Address) {
	return common.BytesToHash(data[0:32]), common.BytesToAddress(data[32:52]), common.BytesToAddress(data[52:72])
}

func (c ringCache) save(fields ...[]byte) error {
	return cache.SAdd(c.cacheKey(), cacheTtl, fields...)
}

func (c ringCache) exists() (bool, error) {
	return cache.Exists(c.cacheKey())
}

type ownerCache struct {
	owner  common.Address
	tokenS common.Address
}

func (c ownerCache) cacheKey() string {
	return OwnerPrefix + strings.ToLower(c.owner.Hex()) + strings.ToLower(c.tokenS.Hex())
}

func (c ownerCache) cacheField(orderhash common.Hash) []byte {
	return []byte(strings.ToLower(orderhash.Hex()))
}

func (c ownerCache) parseField(data []byte) common.Hash {
	return common.HexToHash(string(data))
}

func (c ownerCache) removeOrder(orderhash common.Hash) error {
	_, err := cache.SRem(c.cacheKey(), c.cacheField(orderhash))
	return err
}

func (c ownerCache) save(orderhash common.Hash) error {
	return cache.SAdd(c.cacheKey(), cacheTtl, c.cacheField(orderhash))
}

func (c ownerCache) orderhashes() ([]common.Hash, error) {
	hashes := []common.Hash{}
	// lgh: redis SMembers ,返回集合 key 中的所有成员
	// key: OwnerPrefix + strings.ToLower(c.owner.Hex()) + strings.ToLower(c.tokenS.Hex())
	// 猜测返回的是：当前 owner 所有以 tokenS 为 sell 方的订单的 hash 值数组
	if hashesData, err := cache.SMembers(c.cacheKey()); nil != err {
		return hashes, err
	} else {
		for _, data := range hashesData {
			orderhash := c.parseField(data)
			hashes = append(hashes, orderhash)
		}
		return hashes, nil
	}
}

type orderCache struct {
	orderhash common.Hash
}

func (c orderCache) cacheKey() string {
	return OrderHashPrefix + strings.ToLower(c.orderhash.Hex())
}

func (c orderCache) cacheField(ringhash common.Hash) []byte {
	return []byte(strings.ToLower(ringhash.Hex()))
}

func (c orderCache) removeRinghash(ringhash common.Hash) error {
	_, err := cache.HDel(c.cacheKey(), c.cacheField(ringhash))
	return err
}

func (c orderCache) save(ringhash common.Hash, matchedState *OrderMatchedState) error {
	if matchedData, err := json.Marshal(matchedState); nil != err {
		return err
	} else {
		// lgh: c.cacheKey() === key , []byte(strings.ToLower(ringhash.Hex())) == field
		return cache.HMSet(c.cacheKey(), cacheTtl, []byte(strings.ToLower(ringhash.Hex())), matchedData)
	}
}

func (c orderCache) matchedStates() ([]*OrderMatchedState, error) {
	states := []*OrderMatchedState{}
	// lgh: redis.HVals 根据key返回它对应的hash表的值列表，一对多
	// lgh: 根据当前的订单的 orderhash 去获取。因为当前的对应关系是一对多
	// 所以下面会获取回所有含有该订单的环相关的买卖量
	if filledData, err := cache.HVals(c.cacheKey()); nil != err {
		log.Errorf("matchedStates orderhash:%s, err:%s", c.orderhash.Hex(), err.Error())
		return states, err
	} else {
		for _, data := range filledData {
			matchedState := &OrderMatchedState{}
			if err := json.Unmarshal(data, matchedState); nil == err {
				states = append(states, matchedState)
			} else {
				log.Errorf("matchedStates orderhash:%s, err:%s", c.orderhash.Hex(), err.Error())
			}
		}
	}
	return states, nil
}

// lgh: 主要的作用是找出当前的订单 order 在参与之前的环中，已经买卖了多少的量
func (c orderCache) dealtAmount() (dealtAmountS *big.Rat, dealtAmountB *big.Rat, err error) {
	dealtAmountS = big.NewRat(int64(0), int64(1))
	dealtAmountB = big.NewRat(int64(0), int64(1))

	// lgh: 下面去 redis 中获取。对于的设置的地方在 AddMinedRing 方法中
	if states, err := c.matchedStates(); nil != err {
		log.Errorf("orderhash:%s err:%s", c.orderhash.Hex(), err.Error())
		return dealtAmountS, dealtAmountB, err
	} else {
		for _, state := range states {
			// lgh: 找出后，进行累加 dealtAmountS 就是已经处理了，即买卖了的总量
			dealtAmountS.Add(dealtAmountS, state.FilledAmountS.BigRat())
			dealtAmountB.Add(dealtAmountB, state.FilledAmountB.BigRat())
		}
	}
	return dealtAmountS, dealtAmountB, err
}

func RemoveMinedRingAndReturnOrderhashes(ringhash common.Hash) ([]common.Hash, error) {
	c := ringCache{}
	c.ringhash = ringhash

	cacheKey := c.cacheKey()
	orderhashes := []common.Hash{}
	if data, err := cache.SMembers(cacheKey); nil != err {
		return orderhashes, err
	} else {
		// lgh: 清空 买卖量 的缓存 - removeRinghash
		for _, d := range data {
			orderhash, owner, tokenS := c.parseFiled(d)
			orderhashes = append(orderhashes, orderhash)
			ordCache := orderCache{}
			ordCache.orderhash = orderhash
			if err := ordCache.removeRinghash(ringhash); nil != err {
				log.Errorf("RemoveMinedRing err:%s", err.Error())
			}
			ownerC := ownerCache{}
			ownerC.owner = owner
			ownerC.tokenS = tokenS
			if err := ownerC.removeOrder(orderhash); nil != err {
				log.Errorf("RemoveMinedRing err:%s", err.Error())
			}
		}
	}

	if err := cache.Del(cacheKey); nil != err {
		return orderhashes, err
	} else {
		return orderhashes, nil
	}
}

//添加已经提交了的环路
func AddMinedRing(ringState *types.RingSubmitInfo) {
	ringC := ringCache{}
	ringC.ringhash = ringState.RawRing.Hash // 环整体的 hash
	//ringFieldData := [][]byte{}
	for _, filledOrder := range ringState.RawRing.Orders {
		orderhash := filledOrder.OrderState.RawOrder.Hash // 单个订单 order 的 hash
		owner := filledOrder.OrderState.RawOrder.Owner
		tokenS := filledOrder.OrderState.RawOrder.TokenS
		//ringFieldData = append(ringFieldData, ringC.cacheFiled(orderhash, owner, tokenS))
		ringC.save(ringC.cacheFiled(orderhash, owner, tokenS))

		ordC := orderCache{}
		ordC.orderhash = orderhash // 订单的 hash 在 redis 的 hmset 中作 key
		matchedState := &OrderMatchedState{}
		matchedState.FilledAmountB = types.NewBigRat(filledOrder.FillAmountB)
		matchedState.FilledAmountS = types.NewBigRat(filledOrder.FillAmountS)

		// lgh: 设置到 redis 缓存中，记录当前订单 order 买卖了多少量
		// redis 中的 field 是 order 的 hash
		/*
		对应的关系就是：
			订单 hash 对应 多个环 hash
		*/
		ordC.save(ringC.ringhash, matchedState)

		ownerC := ownerCache{}
		ownerC.owner = owner
		ownerC.tokenS = tokenS
		ownerC.save(orderhash)
	}
	//ringC.save(ringFieldData...)
}

func CachedMatchedRing(ringhash common.Hash) (bool, error) {
	ringC := ringCache{}
	ringC.ringhash = ringhash
	return ringC.exists()

}

func FilledAmountS(owner, tokenS common.Address) (filledAmountS *big.Rat, err error) {
	filledAmountS = big.NewRat(int64(0), int64(1))
	ownerC := ownerCache{}
	ownerC.owner = owner
	ownerC.tokenS = tokenS

	// lgh: orderhashes，猜测返回的是：当前 owner 所有以 tokenS 为 sell 方的订单的 hash 值数组
	if orderhashes, err := ownerC.orderhashes(); nil != err {
		return filledAmountS, err
	} else {
		for _, hash := range orderhashes {
			ordC := orderCache{}
			ordC.orderhash = hash
			// lgh: todo dealtAmount 是个不知道干什么的函数
			if dealtAmountS, _, err := ordC.dealtAmount(); nil == err {
				// lgh: rat.add 是相加，a/b.add(a/b,a1/b1) = a/b + a1/b1
				filledAmountS.Add(filledAmountS, dealtAmountS)
			} else {
				log.Errorf("FilledAmount err:%s", err.Error())
				return filledAmountS, err
			}
		}
		return filledAmountS, nil
	}
}

func DealtAmount(orderhash common.Hash) (dealtAmountS, dealtAmountB *big.Rat, err error) {
	ordC := orderCache{}
	ordC.orderhash = orderhash
	return ordC.dealtAmount()
}

func CachedRinghashes() ([]common.Hash, error) {
	prefixBytes := []byte(RingHashPrefix)
	prefixLen := len(prefixBytes)
	hashes := []common.Hash{}
	if keysBytes, err := cache.Keys(RingHashPrefix + "*"); nil == err {
		for _, key := range keysBytes {
			hashBytes := key[prefixLen:]
			ringhash := common.HexToHash(string(hashBytes))
			hashes = append(hashes, ringhash)
		}
		return hashes, nil
	} else {
		return hashes, err
	}
}

func CacheRinghashToUniqueId(ringhash, uniqueId common.Hash) {
	cache.Set(RinghashToUniqueIdPrefix+strings.ToLower(ringhash.Hex()), []byte(uniqueId.Hex()), cacheTtl)
}

func GetUniqueIdByRinghash(ringhash common.Hash) (common.Hash, error) {
	if data, err := cache.Get(RinghashToUniqueIdPrefix + strings.ToLower(ringhash.Hex())); nil == err {
		return common.BytesToHash(data), err
	} else {
		return types.NilHash, err
	}
}

func AddFailedRingCache(uniqueId, txhash common.Hash, orderhashes []common.Hash) {
	cache.SAdd(FailedRingPrefix+strings.ToLower(uniqueId.Hex()), cacheTtl, []byte(strings.ToLower(txhash.Hex())))
	for _, orderhash := range orderhashes {
		cache.SAdd(FailedOrderPrefix+strings.ToLower(orderhash.Hex()), cacheTtl, []byte(strings.ToLower(uniqueId.Hex())))
	}
}

func RingExecuteFailedCount(uniqueId common.Hash) (int64, error) {
	return cache.SCard(FailedRingPrefix + strings.ToLower(uniqueId.Hex()))
}

func OrderExecuteFailedCount(orderhash common.Hash) (int64, error) {
	// lgh: redis SCard 命令返回集合中元素的数量。
	return cache.SCard(FailedOrderPrefix + strings.ToLower(orderhash.Hex()))
}
