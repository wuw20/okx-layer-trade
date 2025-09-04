package main

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// VerifyAndTradeService VerifyAndTrade 验证apiKey和交易 并且返回txHash
func VerifyAndTradeService(verifyAndTrade VerifyAndTrade) (string, error) {

	client := verifyAndTrade.client
	ctx := verifyAndTrade.ctx

	// 钱包地址
	from := common.HexToAddress(WalletAddress)
	// router地址
	router := common.HexToAddress(RouterAddress)

	coinPriceMap := verifyAndTrade.coinPriceMap
	var coinAddress string
	var coinBalance CoinBalance
	for k, v := range coinPriceMap {
		coinAddress = k
		coinBalance = v
	}

	walletCoinBalanceMap := verifyAndTrade.walletCoinBalanceMap
	var okbCoinBalance CoinBalance

	// ===== 示例参数（替换成你的实际数据） =====
	from := common.HexToAddress(WalletAddress)
	router := common.HexToAddress(RouterAddress)

	tokenIn := common.HexToAddress("0xYourTokenIn")
	tokenOut := common.HexToAddress("0xYourTokenOut")
	amountIn := big.NewInt(1e18)       // 卖出 1 tokenIn
	amountOutMin := big.NewInt(0)      // 演示用，生产环境要设置合理滑点保护
	deadline := big.NewInt(9999999999) // 演示用

	return "", nil
}
