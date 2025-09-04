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
	privateKeyHex   string                  // apiKey
	ctx             context.Context         // rpc连接
	client          *ethclient.Client       // rpc节点clint
	tradePrice      *big.Int                // 交易金额 okb
	chainID         *big.Int                // X Layer 主网 chainId
	coinPriceMap    map[string]CoinPrice    // 代币价格
	tokenBalanceMap map[string]TokenBalance // 账户下代币余额
}

// TokenBalance 钱包地址下指定代币的余额
type TokenBalance struct {
	tokenAddress string     // 代币地址
	balance      *big.Float // 余额
}

const pairABI = `[{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"token0","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"token1","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}]`
const erc20ABI = `[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`
