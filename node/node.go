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

package node

import (
	"sync"

	"fmt"
	"github.com/Loopring/relay/cache"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/crypto"
	"github.com/Loopring/relay/dao"
	"github.com/Loopring/relay/ethaccessor"
	"github.com/Loopring/relay/extractor"
	"github.com/Loopring/relay/gateway"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/market"
	"github.com/Loopring/relay/market/util"
	"github.com/Loopring/relay/marketcap"
	"github.com/Loopring/relay/miner"
	"github.com/Loopring/relay/miner/timing_matcher"
	"github.com/Loopring/relay/ordermanager"
	"github.com/Loopring/relay/txmanager"
	"github.com/Loopring/relay/usermanager"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"go.uber.org/zap"
)

const (
	MODEL_RELAY = "relay"
	MODEL_MINER = "miner"
)

type Node struct {
	globalConfig      *config.GlobalConfig
	rdsService        dao.RdsService
	ipfsSubService    gateway.IPFSSubService
	orderManager      ordermanager.OrderManager
	userManager       usermanager.UserManager
	marketCapProvider marketcap.MarketCapProvider // 市值
	accountManager    market.AccountManager
	relayNode         *RelayNode
	mineNode          *MineNode

	stop   chan struct{}
	lock   sync.RWMutex
	logger *zap.Logger
}

type RelayNode struct {
	extractorService extractor.ExtractorService
	trendManager     market.TrendManager
	tickerCollector  market.CollectorImpl
	jsonRpcService   gateway.JsonrpcServiceImpl
	websocketService gateway.WebsocketServiceImpl
	socketIOService  gateway.SocketIOServiceImpl
	walletService    gateway.WalletServiceImpl
	txManager        txmanager.TransactionManager
}

func (n *RelayNode) Start() {
	n.txManager.Start()
	n.extractorService.Start()

	//gateway.NewJsonrpcService("8080").Start()
	fmt.Println("step in relay node start")
	n.tickerCollector.Start()
	go n.jsonRpcService.Start()
	//n.websocketService.Start()
	go n.socketIOService.Start()

}

func (n *RelayNode) Stop() {
	n.txManager.Stop()
}

type MineNode struct {
	miner *miner.Miner
}

func (n *MineNode) Start() {
	n.miner.Start()
}
func (n *MineNode) Stop() {
	n.miner.Stop()
}

func NewNode(logger *zap.Logger, globalConfig *config.GlobalConfig) *Node {
	n := &Node{}
	n.logger = logger
	n.globalConfig = globalConfig

	// register
	n.registerMysql() // lgh:初始化数据库引擎句柄和创建对应的表格，使用了 gorm 框架
	fmt.Println("准备初始化 redis")
	cache.NewCache(n.globalConfig.Redis) // lgh:初始化Redis,内存存储三方框架

	util.Initialize(n.globalConfig.Market) // lgh:设置从 json 文件导入代币信息，和市场
	n.registerMarketCap() // lgh: 初始化货币市值信息，去网络同步

	n.registerAccessor()  // lgh: 初始化指定合约的ABI和通过json-rpc请求eth_call去以太坊获取它们的地址，以及启动了定时任务同步本地区块数目，仅数目

	n.registerUserManager() // lgh: 初始化用户白名单相关操作，内存缓存部分基于 go-cache 库，以及启动了定时任务更新白名单列表

	n.registerOrderManager() // lgh: 初始化订单相关配置，含内存缓存-redis，以及系列的订单事件监听者，如cancel,submit,newOrder 等
	n.registerAccountManager() // lgh: 初始化账号管理实例的一些简单参数。内部主要是和订单管理者一样，拥有用户交易动作事件监听者，例如转账，确认等
	n.registerGateway()  // lgh:初始化了系列的过滤规则，包含订单请求规则等。以及 GatewayNewOrder 新订单事件的订阅
	n.registerCrypto(nil) // lgh: 初始化加密器，目前主要是Keccak-256

	if "relay" == globalConfig.Mode {
		n.registerRelayNode()
	} else if "miner" == globalConfig.Mode {
		n.registerMineNode()
	} else {
		n.registerMineNode()
		n.registerRelayNode()
	}

	return n
}

func (n *Node) registerRelayNode() {
	n.relayNode = &RelayNode{}
	n.registerExtractor()
	n.registerTransactionManager() // lgh:事务管理器
	n.registerTrendManager()   // lgh: 趋势数据管理器，市场变化趋势信息
	n.registerTickerCollector() // lgh: 负责统计24小时市场变化统计数据。目前支持的平台有OKEX，币安
	n.registerWalletService() // lgh: 初始化钱包服务实例
	n.registerJsonRpcService()// lgh: 初始化 json-rpc 端口和绑定钱包WalletServiceHandler，start 的时候启动服务
	n.registerWebsocketService() // lgh: 初始化 webSocket
	n.registerSocketIOService()
	txmanager.NewTxView(n.rdsService)
}

func (n *Node) registerMineNode() {
	n.mineNode = &MineNode{}
	// lgh: NewKeyStore 用来导入矿工的 keyStore 文件
	ks := keystore.NewKeyStore(n.globalConfig.Keystore.Keydir, keystore.StandardScryptN, keystore.StandardScryptP)
	n.registerCrypto(ks)
	n.registerMiner()
}

func (n *Node) Start() {
	n.orderManager.Start()
	n.marketCapProvider.Start()

	if n.globalConfig.Mode != MODEL_MINER {
		n.accountManager.Start()
		n.relayNode.Start()
		go ethaccessor.IncludeGasPriceEvaluator()
	}
	if n.globalConfig.Mode != MODEL_RELAY {
		n.mineNode.Start()
		ethaccessor.IncludeGasPriceEvaluator()
	}
}

func (n *Node) Wait() {
	n.lock.RLock()

	// TODO(fk): states should be judged

	stop := n.stop
	n.lock.RUnlock()

	<-stop
}

func (n *Node) Stop() {
	n.lock.RLock()
	n.mineNode.Stop()
	//
	//n.p2pListener.Stop()
	//n.chainListener.Stop()
	//n.orderbook.Stop()
	//n.miner.Stop()

	//close(n.stop)

	n.lock.RUnlock()
}

func (n *Node) registerCrypto(ks *keystore.KeyStore) {
	c := crypto.NewKSCrypto(true, ks)
	crypto.Initialize(c)
}

func (n *Node) registerMysql() {
	n.rdsService = dao.NewRdsService(n.globalConfig.Mysql)
	n.rdsService.Prepare()
}

func (n *Node) registerAccessor() {
	err := ethaccessor.Initialize(n.globalConfig.Accessor, n.globalConfig.Common, util.WethTokenAddress())
	if nil != err {
		log.Fatalf("err:%s", err.Error())
	}
}

func (n *Node) registerExtractor() {
	n.relayNode.extractorService = extractor.NewExtractorService(n.globalConfig.Extractor, n.rdsService)
}

func (n *Node) registerIPFSSubService() {
	n.ipfsSubService = gateway.NewIPFSSubService(n.globalConfig.Ipfs)
}

func (n *Node) registerOrderManager() {
	n.orderManager = ordermanager.NewOrderManager(&n.globalConfig.OrderManager, n.rdsService, n.userManager, n.marketCapProvider)
}

func (n *Node) registerTrendManager() {
	n.relayNode.trendManager = market.NewTrendManager(n.rdsService, n.globalConfig.Market.CronJobLock)
}

func (n *Node) registerAccountManager() {
	n.accountManager = market.NewAccountManager(n.globalConfig.AccountManager)
}

func (n *Node) registerTransactionManager() {
	n.relayNode.txManager = txmanager.NewTxManager(n.rdsService, &n.accountManager)
}

func (n *Node) registerTickerCollector() {
	n.relayNode.tickerCollector = *market.NewCollector(n.globalConfig.Market.CronJobLock)
}

func (n *Node) registerWalletService() {
	n.relayNode.walletService = *gateway.NewWalletService(n.relayNode.trendManager, n.orderManager,
		n.accountManager, n.marketCapProvider, n.relayNode.tickerCollector, n.rdsService, n.globalConfig.Market.OldVersionWethAddress)
}

func (n *Node) registerJsonRpcService() {
	n.relayNode.jsonRpcService = *gateway.NewJsonrpcService(n.globalConfig.Jsonrpc.Port, &n.relayNode.walletService)
}

func (n *Node) registerWebsocketService() {
	n.relayNode.websocketService = *gateway.NewWebsocketService(n.globalConfig.Websocket.Port, n.relayNode.trendManager, n.accountManager, n.marketCapProvider)
}

func (n *Node) registerSocketIOService() {
	n.relayNode.socketIOService = *gateway.NewSocketIOService(n.globalConfig.Websocket.Port, n.relayNode.walletService)
}

func (n *Node) registerMiner() {
	//ethaccessor.IncludeGasPriceEvaluator()
	// lgh: 初始化环提交者
	submitter, err := miner.NewSubmitter(n.globalConfig.Miner, n.rdsService, n.marketCapProvider)
	if nil != err {
		log.Fatalf("failed to init submitter, error:%s", err.Error())
	}
	evaluator := miner.NewEvaluator(n.marketCapProvider, n.globalConfig.Miner)
	matcher := timing_matcher.NewTimingMatcher(
		n.globalConfig.Miner.TimingMatcher,
		submitter, evaluator, n.orderManager, &n.accountManager, n.rdsService)
	evaluator.SetMatcher(matcher)
	// lgh: 一个矿工实体包含有 提交者，匹配者，计费者
	n.mineNode.miner = miner.NewMiner(submitter, matcher, evaluator, n.marketCapProvider)
}

func (n *Node) registerGateway() {
	gateway.Initialize(&n.globalConfig.GatewayFilters, &n.globalConfig.Gateway, &n.globalConfig.Ipfs, n.orderManager, n.marketCapProvider, n.accountManager)
}

func (n *Node) registerUserManager() {
	n.userManager = usermanager.NewUserManager(&n.globalConfig.UserManager, n.rdsService)
}

func (n *Node) registerMarketCap() {
	n.marketCapProvider = marketcap.NewMarketCapProvider(n.globalConfig.MarketCap)
}




















