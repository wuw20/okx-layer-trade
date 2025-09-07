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

// VerifyAndTrade 验证apiKey和交易入参
type VerifyAndTrade struct {
	privateKeyHex        string                       // apiKey
	ctx                  context.Context              // rpc连接
	client               *ethclient.Client            // rpc节点clint
	chainID              *big.Int                     // X Layer 主网 chainId
	walletCoinBalanceMap map[string]WalletCoinBalance // 账户下代币余额
	tradeQuantity        *big.Int                     // 交易代币数量
}

// WalletCoinBalance 钱包地址下指定代币的余额
type WalletCoinBalance struct {
	walletCoinAddress string   // 代币地址
	walletCoinBalance *big.Int // 余额
	coinDecimals      uint8    // 代币单位
}

// TradeCoinParam 交易传入值
type TradeCoinParam struct {
	tradeApiKey        string // apikey x layer
	tradeType          string // 交易类型 buy sell 买卖
	tradeCoinAddress   string // 交易代币地址
	tradeWalletAddress string // 交易钱包地址
	tradeSlippageFee   string // 滑点费用
	tradeQuantity      string // 交易数量
}

// SellCoinParam 卖代币入参
type SellCoinParam struct {
	tradeApiKey        string   // apikey x layer
	tradeCoinAddress   string   // 交易代币地址
	tradeWalletAddress string   // 交易钱包地址
	amountOutMin       *big.Int //滑点
	amountIn           *big.Int // 交易数量
}
