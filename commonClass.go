package main

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

// CoinPrice 查询代币后返回的数据
type CoinPrice struct {
	tokenContractAddress string     // 代币合约地址 代币/USDT 的地址
	coinPrice            *big.Float // usdt 为单位
	blockTimestampLast   uint32     // 调用时间
}

// Reserves 命名返回值用 struct + abi 标签来接
type Reserves struct {
	Reserve0           *big.Int `abi:"_reserve0"`
	Reserve1           *big.Int `abi:"_reserve1"`
	BlockTimestampLast uint32   `abi:"_blockTimestampLast"`
}

// VerifyAndTrade 验证apiKey和交易入参
type VerifyAndTrade struct {
	privateKeyHex string            // apiKey
	ctx           context.Context   // rpc连接
	client        *ethclient.Client // rpc节点clint
	targetAddress common.Address    // 交易目标地址
	tradeGasFee   *big.Int          // 交易gas费用
	tradePrice    *big.Int          // 交易金额 okx
	chainID       *big.Int          // X Layer 主网 chainId
	gasLimit      uint64            // gas费用最大限制
}

// TokenBalance 钱包地址下指定代币的余额
type TokenBalance struct {
	tokenAddress string     // 代币地址
	balance      *big.Float // 余额
}
