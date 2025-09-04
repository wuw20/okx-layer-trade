package main

import (
	"context"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

// CoinBalance 查询代币后返回的数据
type CoinBalance struct {
	coinAddress        string     // 代币合约地址
	orgCoinBalance     *big.Float // 原始余额
	coinBalanceInOKB   *big.Float // 余额折算成 OKB
	coinBalanceInUSDT  *big.Float // 余额折算成 USDT
	blockTimestampLast uint32     // 调用时间
}

// Reserves 命名返回值用 struct + abi 标签来接
type Reserves struct {
	Reserve0           *big.Int `abi:"_reserve0"`
	Reserve1           *big.Int `abi:"_reserve1"`
	BlockTimestampLast uint32   `abi:"_blockTimestampLast"`
}

// VerifyAndTrade 验证apiKey和交易入参
type VerifyAndTrade struct {
	privateKeyHex        string                       // apiKey
	ctx                  context.Context              // rpc连接
	client               *ethclient.Client            // rpc节点clint
	tradePrice           *big.Int                     // 交易金额 okb
	chainID              *big.Int                     // X Layer 主网 chainId
	coinPriceMap         map[string]CoinBalance       // 代币价格 代币/OKB
	walletCoinBalanceMap map[string]WalletCoinBalance // 账户下代币余额
	tradeQuantity        *big.Int                     // 交易代币数量 单位wei
}

// WalletCoinBalance 钱包地址下指定代币的余额
type WalletCoinBalance struct {
	walletCoinAddress string     // 代币地址
	walletCoinBalance *big.Float // 余额
}
