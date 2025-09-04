package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"log"
)

// VerifyAndTradeService VerifyAndTrade 验证apiKey和交易 并且返回txHash
func VerifyAndTradeService(verifyAndTrade VerifyAndTrade) (string, error) {

	// 交易方向
	tradeType := verifyAndTrade.tradeType
	client := verifyAndTrade.client
	ctx := verifyAndTrade.ctx

	// 钱包地址
	from := common.HexToAddress(WalletAddress)
	// router地址
	router := common.HexToAddress(RouterAddress)

	coinPriceMap := verifyAndTrade.coinPriceMap
	var coinAddress string
	var coinPrice CoinPrice
	for k, v := range coinPriceMap {
		coinAddress = k
		coinPrice = v
	}
	fmt.Println("卖出的代币地址，和当前代币价格", coinAddress, coinPrice)
	coinBalanceMap := verifyAndTrade.coinBalanceMap
	var okbCoinBalance CoinBalance
	okbCoinBalance = coinBalanceMap["OKB"]

	var tokenOut common.Address
	var tokenIn common.Address
	if tradeType == "in" {
		tokenOut := common.HexToAddress(coinAddress)               // 你买入的代币
		tokenIn := common.HexToAddress(okbCoinBalance.coinAddress) // 你卖出的代币

	} else if tradeType == "out" {
		tokenOut := common.HexToAddress(okbCoinBalance.coinAddress)
		tokenIn := common.HexToAddress(coinAddress)
	} else {
		log.Fatal("交易不合法")
		return "", fmt.Errorf("交易不合法！！！")
	}

	path := []common.Address{tokenIn, tokenOut}

	return "", nil
}
