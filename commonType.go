package main

import (
	"context"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

// CoinPrice 查询代币后返回的数据
type CoinPrice struct {
	tokenContractAddress string     // 代币合约地址
	coinPrice            *big.Float // okb 为单位
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
	privateKeyHex  string                 // apiKey
	ctx            context.Context        // rpc连接
	client         *ethclient.Client      // rpc节点clint
	tradePrice     *big.Int               // 交易金额 okb
	chainID        *big.Int               // X Layer 主网 chainId
	coinPriceMap   map[string]CoinPrice   // 代币价格
	coinBalanceMap map[string]CoinBalance // 账户下代币余额
	tradeType      string                 // 交易类型  买 卖
}

// CoinBalance 钱包地址下指定代币的余额
type CoinBalance struct {
	coinAddress string     // 代币地址
	balance     *big.Float // 余额
}
