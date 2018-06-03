package main

import (
	"github.com/Loopring/relay/ethaccessor"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/market/util"
)

/**

作者(Author): 林冠宏 / 指尖下的幽灵

Created on : 2018/6/2

*/

func main() {
	c := config.LoadConfig("config/relay.toml")
	log.Initialize(c.Log)
	ethaccessor.Initialize(c.Accessor, c.Common, util.WethTokenAddress())
}

