// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package types

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/Loopring/relay/crypto"
	"github.com/ethereum/go-ethereum/common"
	"fmt"
)

var _ = (*orderMarshaling)(nil)

func (o Order) MarshalJSON() ([]byte, error) {
	type Order struct {
		Protocol              common.Address             `json:"protocol" gencodec:"required"`
		DelegateAddress       common.Address             `json:"delegateAddress" gencodec:"required"`
		AuthAddr              common.Address             `json:"authAddr" gencodec:"required"`
		AuthPrivateKey        crypto.EthPrivateKeyCrypto `json:"authPrivateKey" gencodec:"required"`
		WalletAddress         common.Address             `json:"walletAddress" gencodec:"required"`
		TokenS                common.Address             `json:"tokenS" gencodec:"required"`
		TokenB                common.Address             `json:"tokenB" gencodec:"required"`
		AmountS               *Big                       `json:"amountS" gencodec:"required"`
		AmountB               *Big                       `json:"amountB" gencodec:"required"`
		ValidSince            *Big                       `json:"validSince" gencodec:"required"`
		ValidUntil            *Big                       `json:"validUntil" gencodec:"required"`
		LrcFee                *Big                       `json:"lrcFee" `
		BuyNoMoreThanAmountB  bool                       `json:"buyNoMoreThanAmountB" gencodec:"required"`
		MarginSplitPercentage uint8                      `json:"marginSplitPercentage" gencodec:"required"`
		V                     uint8                      `json:"v" gencodec:"required"`
		R                     Bytes32                    `json:"r" gencodec:"required"`
		S                     Bytes32                    `json:"s" gencodec:"required"`
		Price                 *big.Rat                   `json:"price"`
		Owner                 common.Address             `json:"owner"`
		Hash                  common.Hash                `json:"hash"`
		Market                string                     `json:"market"`
		CreateTime            int64                      `json:"createTime"`
		PowNonce              uint64                     `json:"powNonce"`
		Side                  string                     `json:"side"`
		OrderType             string                     `json:"orderType"`
	}
	var enc Order
	enc.Protocol = o.Protocol
	enc.DelegateAddress = o.DelegateAddress
	enc.AuthAddr = o.AuthAddr
	enc.AuthPrivateKey = o.AuthPrivateKey
	enc.WalletAddress = o.WalletAddress
	enc.TokenS = o.TokenS
	enc.TokenB = o.TokenB
	enc.AmountS = (*Big)(o.AmountS)
	enc.AmountB = (*Big)(o.AmountB)
	enc.ValidSince = (*Big)(o.ValidSince)
	enc.ValidUntil = (*Big)(o.ValidUntil)
	enc.LrcFee = (*Big)(o.LrcFee)
	enc.BuyNoMoreThanAmountB = o.BuyNoMoreThanAmountB
	enc.MarginSplitPercentage = o.MarginSplitPercentage
	enc.V = o.V
	enc.R = o.R
	enc.S = o.S
	enc.Price = o.Price
	enc.Owner = o.Owner
	enc.Hash = o.Hash
	enc.Market = o.Market
	enc.CreateTime = o.CreateTime
	enc.PowNonce = o.PowNonce
	enc.Side = o.Side
	enc.OrderType = o.OrderType
	return json.Marshal(&enc)
}

func (o *Order) UnmarshalJSON(input []byte) error {
	fmt.Println("自定义 订单 json 解析")
	type Order struct {
		Protocol              *common.Address             `json:"protocol" gencodec:"required"`
		DelegateAddress       *common.Address             `json:"delegateAddress" gencodec:"required"`
		AuthAddr              *common.Address             `json:"authAddr" gencodec:"required"`
		AuthPrivateKey        *crypto.EthPrivateKeyCrypto `json:"authPrivateKey" gencodec:"required"`
		WalletAddress         *common.Address             `json:"walletAddress" gencodec:"required"`
		TokenS                *common.Address             `json:"tokenS" gencodec:"required"`
		TokenB                *common.Address             `json:"tokenB" gencodec:"required"`
		AmountS               *Big                        `json:"amountS" gencodec:"required"`
		AmountB               *Big                        `json:"amountB" gencodec:"required"`
		ValidSince            *Big                        `json:"validSince" gencodec:"required"`
		ValidUntil            *Big                        `json:"validUntil" gencodec:"required"`
		LrcFee                *Big                        `json:"lrcFee" `
		BuyNoMoreThanAmountB  *bool                       `json:"buyNoMoreThanAmountB" gencodec:"required"`
		MarginSplitPercentage *uint8                      `json:"marginSplitPercentage" gencodec:"required"`
		V                     *uint8                      `json:"v" gencodec:"required"`
		R                     *Bytes32                    `json:"r" gencodec:"required"`
		S                     *Bytes32                    `json:"s" gencodec:"required"`
		Price                 *big.Rat                    `json:"price"`
		Owner                 *common.Address             `json:"owner"`
		Hash                  *common.Hash                `json:"hash"`
		Market                *string                     `json:"market"`
		CreateTime            *int64                      `json:"createTime"`
		PowNonce              *uint64                     `json:"powNonce"`
		Side                  *string                     `json:"side"`
		OrderType             *string                     `json:"orderType"`
	}
	var dec Order
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Protocol == nil {
		return errors.New("missing required field 'protocol' for Order")
	}
	o.Protocol = *dec.Protocol
	if dec.DelegateAddress == nil {
		return errors.New("missing required field 'delegateAddress' for Order")
	}
	o.DelegateAddress = *dec.DelegateAddress
	if dec.AuthAddr == nil {
		return errors.New("missing required field 'authAddr' for Order")
	}
	o.AuthAddr = *dec.AuthAddr
	if dec.AuthPrivateKey == nil {
		return errors.New("missing required field 'authPrivateKey' for Order")
	}
	o.AuthPrivateKey = *dec.AuthPrivateKey
	if dec.WalletAddress == nil {
		return errors.New("missing required field 'walletAddress' for Order")
	}
	o.WalletAddress = *dec.WalletAddress
	if dec.TokenS == nil {
		return errors.New("missing required field 'tokenS' for Order")
	}
	o.TokenS = *dec.TokenS
	if dec.TokenB == nil {
		return errors.New("missing required field 'tokenB' for Order")
	}
	o.TokenB = *dec.TokenB
	if dec.AmountS == nil {
		return errors.New("missing required field 'amountS' for Order")
	}
	o.AmountS = (*big.Int)(dec.AmountS)
	if dec.AmountB == nil {
		return errors.New("missing required field 'amountB' for Order")
	}
	o.AmountB = (*big.Int)(dec.AmountB)
	if dec.ValidSince == nil {
		return errors.New("missing required field 'validSince' for Order")
	}
	o.ValidSince = (*big.Int)(dec.ValidSince)
	if dec.ValidUntil == nil {
		return errors.New("missing required field 'validUntil' for Order")
	}
	o.ValidUntil = (*big.Int)(dec.ValidUntil)
	if dec.LrcFee != nil {
		o.LrcFee = (*big.Int)(dec.LrcFee)
	}
	if dec.BuyNoMoreThanAmountB == nil {
		return errors.New("missing required field 'buyNoMoreThanAmountB' for Order")
	}
	o.BuyNoMoreThanAmountB = *dec.BuyNoMoreThanAmountB
	if dec.MarginSplitPercentage == nil {
		return errors.New("missing required field 'marginSplitPercentage' for Order")
	}
	o.MarginSplitPercentage = *dec.MarginSplitPercentage
	if dec.V == nil {
		return errors.New("missing required field 'v' for Order")
	}
	o.V = *dec.V
	if dec.R == nil {
		return errors.New("missing required field 'r' for Order")
	}
	o.R = *dec.R
	if dec.S == nil {
		return errors.New("missing required field 's' for Order")
	}
	o.S = *dec.S
	if dec.Price != nil {
		o.Price = dec.Price
	}
	if dec.Owner != nil {
		o.Owner = *dec.Owner
	}
	if dec.Hash != nil {
		o.Hash = *dec.Hash
	}
	if dec.Market != nil {
		o.Market = *dec.Market
	}
	if dec.CreateTime != nil {
		o.CreateTime = *dec.CreateTime
	}
	if dec.PowNonce != nil {
		o.PowNonce = *dec.PowNonce
	}
	if dec.Side != nil {
		o.Side = *dec.Side
	}
	if dec.OrderType != nil {
		o.OrderType = *dec.OrderType
	}
	return nil
}
