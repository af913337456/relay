package lgh
//
//
//
//// 订单处理入口
//func HandleInputOrder(input eventemitter.EventData) (orderHash string, err error) {
//	var (
//		state *types.OrderState
//	)
//
//	order := input.(*types.Order)
//	order.Hash = order.GenerateHash()
//	orderHash = order.Hash.Hex()
//	//TODO(xiaolu) 这里需要测试一下，超时error和查询数据为空的error，处理方式不应该一样
//	if state, err = gateway.om.GetOrderByHash(order.Hash); err != nil && err.Error() == "record not found" {
//		// lgh: 如果该订单本地数据库没有记录，那么进入这里，触发新订单事件，否则触发订单已经存在的错误
//		// lgh: generate 生成，下面是生成实际价格比例，generatePrice 内部会判断交易的代币是否是 allToken 里面的，是否是被支持的
//		if err = generatePrice(order); err != nil {
//			return orderHash, err
//		}
//
//		// lgh: 订单数值的格式各种判断
//		for _, v := range gateway.filters {
//			valid, err := v.filter(order)
//			if !valid {
//				log.Errorf(err.Error())
//				return orderHash, err
//			}
//		}
//		state = &types.OrderState{}
//		state.RawOrder = *order // 这里赋值 RawOrder
//		//broadcastTime = 0
//		eventemitter.Emit(eventemitter.NewOrder, state)
//	} else {
//		//broadcastTime = state.BroadcastTime
//		log.Infof("gateway,order %s exist,will not insert again", order.Hash.Hex())
//		return orderHash, errors.New("order existed, please not submit again")
//	}
//	return orderHash, err
//}
//
//// OrderState 转为 Order 的方法
//func (o *Order) ConvertDown(state *types.OrderState) error {
//	src := state.RawOrder // RawOrder  Order  `json:"rawOrder"`
//
//	o.Price, _ = src.Price.Float64()
//	o.AmountS = src.AmountS.String()
//	o.AmountB = src.AmountB.String()
//	o.DealtAmountS = state.DealtAmountS.String()
//	o.DealtAmountB = state.DealtAmountB.String()
//	o.SplitAmountS = state.SplitAmountS.String()
//	o.SplitAmountB = state.SplitAmountB.String()
//	o.CancelledAmountS = state.CancelledAmountS.String()
//	o.CancelledAmountB = state.CancelledAmountB.String()
//	o.LrcFee = src.LrcFee.String()
//
//	o.Protocol = src.Protocol.Hex()
//
//	// 文档得知，order 的 DelegateAddress 来自于客户端的传参
//	// https://github.com/af913337456/relay/blob/master/LOOPRING_RELAY_API_SPEC_V2.md
//	o.DelegateAddress = src.DelegateAddress.Hex() // 这行和 delegate_address 挂钩
//	o.Owner = src.Owner.Hex()
//
//	auth, _ := src.AuthPrivateKey.MarshalText()
//	o.PrivateKey = string(auth)
//	o.AuthAddress = src.AuthAddr.Hex()
//	o.WalletAddress = src.WalletAddress.Hex()
//
//	o.OrderHash = src.Hash.Hex()
//	o.TokenB = src.TokenB.Hex()
//	o.TokenS = src.TokenS.Hex()
//	o.CreateTime = time.Now().Unix()
//	o.ValidSince = src.ValidSince.Int64()
//	o.ValidUntil = src.ValidUntil.Int64()
//
//	o.BuyNoMoreThanAmountB = src.BuyNoMoreThanAmountB
//	o.MarginSplitPercentage = src.MarginSplitPercentage
//	if state.UpdatedBlock != nil {
//		o.UpdatedBlock = state.UpdatedBlock.Int64()
//	}
//	o.Status = uint8(state.Status)
//	o.V = src.V
//	o.S = src.S.Hex()
//	o.R = src.R.Hex()
//	o.PowNonce = src.PowNonce
//	o.BroadcastTime = state.BroadcastTime
//	o.Side = state.RawOrder.Side
//	o.OrderType = state.RawOrder.OrderType
//
//	return nil
//}
//
//atoBOrders := market.om.MinerOrders(
//		delegateAddress, // lgh: 配置文件的地址
//		market.TokenA, // lgh: AllTokenPairs 的 tokenS
//		market.TokenB, // lgh: AllTokenPairs 的 tokenB
//		market.matcher.roundOrderCount, // 配置文件中的 roundOrderCount，默认是 2
//		market.matcher.reservedTime, // 保留的提交时间，默认是 45，单位未知
//		int64(0),
//		currentRoundNumber, // 开始进入循环时候的当前的毫秒级别的时间戳
//		// lgh: AtoBOrderHashesExcludeNextRound 一开始是空切片。应该是用来过滤某些订单的
//		// deleyedNumber 是开始时候的毫秒数 + 10000，就是多了10秒
//		&types.OrderDelayList{OrderHash: market.AtoBOrderHashesExcludeNextRound, DelayedCount: deleyedNumber})
//
//
//// 开始获取矿工订单
//func (om *OrderManagerImpl) MinerOrders(
//	protocol, tokenS, tokenB common.Address,
//	length int, reservedTime, startBlockNumber,
//	endBlockNumber int64, filterOrderHashLists ...*types.OrderDelayList) []*types.OrderState {
//	var list []*types.OrderState
//
//	// 订单在extractor同步结束后才可以提供给miner进行撮合
//	//if !om.ordersValidForMiner {
//	//	return list
//	//}
//
//	var (
//		modelList    []*dao.Order
//		err          error
//		// lgh: 过滤掉订单处于完成，切断，取消状态的。状态的更改在 abi event 类型的方法被触发后进行。
//		// 具体见函数 loadProtocolContract。由调用合约函数的人触发
//		filterStatus = []types.OrderStatus{types.ORDER_FINISHED, types.ORDER_CUTOFF, types.ORDER_CANCEL}
//	)
//
//	// lgh: filterOrderHashLists 目前的 size 总是 1
//	for _, orderDelay := range filterOrderHashLists {
//		orderHashes := []string{}
//		for _, hash := range orderDelay.OrderHash {
//			orderHashes = append(orderHashes, hash.Hex())
//		}
//		if len(orderHashes) > 0 && orderDelay.DelayedCount != 0 {
//			// lgh: 如果存在要延时的订单，下面去数据库更新它们的 blockNumber，排队号
//			if err = om.rds.MarkMinerOrders(orderHashes, orderDelay.DelayedCount); err != nil {
//				log.Debugf("order manager,provide orders for miner error:%s", err.Error())
//			}
//		}
//	}
//
//	// 从数据库获取订单
//	if modelList, err = om.rds.GetOrdersForMiner(
//		protocol.Hex(), // 配置文件的地址
//		tokenS.Hex(),
//		tokenB.Hex(),
//		length, // 是2
//		filterStatus,
//		reservedTime, // 保留的时间 45
//		startBlockNumber, // 有时候是0
//		// endBlockNumber 进入循环 + 10000 = 10 秒
//		endBlockNumber); err != nil {
//
//		log.Errorf("err:%s", err.Error())
//		return list
//	}
//
//	for _, v := range modelList {
//		state := &types.OrderState{}
//		v.ConvertUp(state)
//		if om.um.InWhiteList(state.RawOrder.Owner) {
//			list = append(list, state)
//		} else {
//			log.Debugf("order manager,owner:%s not in white list", state.RawOrder.Owner.Hex())
//		}
//	}
//
//	return list
//}
//
//// 所有来自gateway的订单都是新订单，订单存入数据库的函数
//func (om *OrderManagerImpl) handleGatewayOrder(input eventemitter.EventData) error {
//	state := input.(*types.OrderState)
//	log.Debugf("order manager,handle gateway order,order.hash:%s amountS:%s", state.RawOrder.Hash.Hex(), state.RawOrder.AmountS.String())
//
//	//lgh: 内部做一些信息的包装
//	model, err := newOrderEntity(state, om.mc, nil)
//	if err != nil {
//		log.Errorf("order manager,handle gateway order:%s error: %s", state.RawOrder.Hash.Hex(),err.Error())
//		return err
//	}
//
//	// lgh: 深度更新事件
//	eventemitter.Emit(eventemitter.DepthUpdated,
//		types.DepthUpdateEvent{
//			DelegateAddress: model.DelegateAddress,
//			Market: model.Market})
//
//	return om.rds.Add(model) // lgh: 这个时候才把订单放入到本地数据库
//}
//
//// lgh: 根据文档显示，配置文件中的 ProtocolImpl.Address 应该设置为 LoopringProtocolImpl
//// lgh: lrcTokenAddress,tokenRegistryAddress,delegateAddress 在 LoopringProtocolImpl 中获取
//// lgh: 且 delegateAddress 还是应该对应客户端订单中的 DelegateAddress 字段，这样才能在数据库中匹配中
//// https://github.com/Loopring/token-listing/blob/master/ethereum/deployment.md
//
//
//// 从数据库获取矿工订单
//func (s *RdsServiceImpl) GetOrdersForMiner(protocol, tokenS, tokenB string, length int, filterStatus []types.OrderStatus, reservedTime, startBlockNumber, endBlockNumber int64) ([]*Order, error) {
//	var (
//		list []*Order
//		err  error
//	)
//
//	if len(filterStatus) < 1 {
//		return list, errors.New("should filter cutoff and finished orders")
//	}
//
//	nowtime := time.Now().Unix()
//	sinceTime := nowtime
//	untilTime := nowtime + reservedTime// reservedTime 默认是 45
//	err = s.db.Where(
//		// delegate_address 配置文件的地址。要去对照下，存入数据库的时候，这个是什么
//		"delegate_address = ? and token_s = ? and token_b = ?", protocol, tokenS, tokenB).
//		Where("valid_since < ?", sinceTime).// valid_since 订单的创建时间
//		Where("valid_until >= ? ", untilTime). // valid_until 订单的过期时间
//		Where("status not in (?) ", filterStatus).
//		Where("order_type = ? ", types.ORDER_TYPE_MARKET).
//		Where("miner_block_mark between ? and ?", startBlockNumber, endBlockNumber).
//		Order("price desc").
//		Limit(length). // 取2条
//		Find(&list).
//		Error
//
//	return list, err
//}
