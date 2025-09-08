package main

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"time"
)

// TradeInfo 交易入参
type TradeInfo struct {
	PrivateKey *ecdsa.PrivateKey // 钱包私钥
	TokenAddr  common.Address    // 代币地址
	OkbAddr    common.Address    // okb地址
	RouterAddr common.Address    // router地址
	ChainID    *big.Int          // x layer地址
	SlippageBP int               // 滑点
	Deadline   time.Duration     // 超时时间
}
