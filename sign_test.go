package main

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

/**

作者(Author): 林冠宏 / 指尖下的幽灵

Created on : 2018/5/27

*/


type Order struct {
	Protocol              []byte
	Owner                 []byte
	TokenS                []byte
	TokenB                []byte
	AmountS               []byte
	AmountB               []byte
	Timestamp             []byte
	Ttl                   []byte
	Salt                  []byte
	LrcFee                []byte
	BuyNoMoreThanAmountB  bool
	MarginSplitPercentage []byte
	V                     uint8
	R                     []byte
	S                     []byte
}

// 使用Keccak-256 算法对这个字节数组做散列计算得到订单的Hash
func (o *Order) Hash() []byte {
	return crypto.Keccak256(
		o.Protocol,
		o.Owner,
		o.TokenS,
		o.TokenB,
		common.LeftPadBytes(o.AmountS, 32),
		common.LeftPadBytes(o.AmountB, 32),
		common.LeftPadBytes(o.Timestamp, 32),
		common.LeftPadBytes(o.Ttl, 32),
		common.LeftPadBytes(o.Salt, 32),
		common.LeftPadBytes(o.LrcFee, 32),
		[]byte{boolToByte(o.BuyNoMoreThanAmountB)},
		o.MarginSplitPercentage,
	)
}

func (o *Order) GenerateAndSetSignature(privateKey *ecdsa.PrivateKey) error {
	hashWithPrefix := crypto.Keccak256(
		[]byte("\x19Ethereum Signed Message:\n32"),
		o.Hash(),
	)
	// Secp256k1签名算法对得到的Hash进行签名得到Sig
	if sig, err := crypto.Sign(hashWithPrefix, privateKey); nil != err {
		return err
	} else {
		o.V = uint8(sig[64]) + uint8(27)  // Sig的64位加上27转换成整型Order的v
		o.R = make([]byte, 32)
		o.S = make([]byte, 32)
		copy(o.R, sig[0:32])  // Sig的0-32 位转换成16进制字符串赋值给Order的r
		copy(o.S, sig[32:64]) // Sig的32-64 位转换成16进制字符串赋值给Order的s
	}
	return nil
}

func boolToByte(b bool) byte {
	if b {
		return byte(1)
	} else {
		return byte(0)
	}
}
//
//func TestMain2(t *testing.T) {
//	privateKey, _ := crypto.ToECDSA(common.FromHex("d1d194d90e52aeae4cd3a727b1dbb6ea5f1de8d5379827acc5f358bf1b0acba9"))
//
//	order := &Order{
//		Protocol:              common.FromHex("0xd02d3e40cde61c267a3886f5828e03aa4914073d"),
//		Owner:                 common.FromHex("0x81C4511396E26Ac12C83c3B178ba0690e863f8D2"),
//		TokenS:                common.FromHex("0x2956356cD2a2bf3202F771F50D3D14A367b48070"),
//		TokenB:                common.FromHex("0xEF68e7C694F40c8202821eDF525dE3782458639f"),
//		AmountS:               common.FromHex("0xa"),
//		AmountB:               common.FromHex("0xc350"),
//		Timestamp:             common.FromHex("0x59ccbe14"),
//		Ttl:                   common.FromHex("0xd2f00"),
//		Salt:                  common.FromHex("0x1"),
//		LrcFee:                common.FromHex("0x5"),
//		BuyNoMoreThanAmountB:  true,
//		MarginSplitPercentage: []byte{byte(50)},
//	}
//
//	if err := order.GenerateAndSetSignature(privateKey); nil != err {
//		panic(err)
//	}
//}
//
