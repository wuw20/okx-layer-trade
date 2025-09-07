package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math/big"
)

// Sell 卖代币 默认转化成 OKB
func Sell(sellCoinParam SellCoinParam, client *ethclient.Client) (string, error) {

	//tradeApiKey := sellCoinParam.tradeApiKey
	tradeCoinAddress := sellCoinParam.tradeCoinAddress
	tradeWalletAddress := sellCoinParam.tradeWalletAddress

	// 卖 需要查到钱包代币余额 & ETH余额（用于手续费）
	tokenAddressMap := make(map[string]string)
	tokenAddressMap["ETH"] = ETHAddress
	tokenAddressMap["sellCoin"] = tradeCoinAddress

	// 返回代币数量 & 精度
	tokenBalanceMap, err := GetMultiERC20Balances(client, tradeWalletAddress, tokenAddressMap)
	if err != nil || len(tokenBalanceMap) == 0 {
		log.Fatal("获取钱包下面代币余额失败！！！")
	}

	// 代币 or ETH余额为0 交易直接失败
	for coinKey, coinBalance := range tokenBalanceMap {
		if len(coinBalance.walletCoinBalance.Bits()) == 0 {
			fmt.Println("该钱包代币 or ETH余额为0，无法进行交易！！！", coinKey)
			log.Fatal("该钱包代币 or ETH余额为0，无法进行交易！！！")
		}
	}

	// 获取代币精度
	// 卖代币 和 余额对比

	// 计算交易需要的gas费用
	// 钱包地址
	from := common.HexToAddress(tradeWalletAddress)
	// router地址
	router := common.HexToAddress(RouterAddress)

	// 使用参数中的 tokenIn, tokenOut, amountIn
	tokenIn := common.HexToAddress(tradeCoinAddress)
	tokenOut := common.HexToAddress(OKBAddress)
	amountIn := sellCoinParam.amountIn
	amountOutMin := sellCoinParam.amountOutMin // 演示用，生产环境要设置合理滑点保护
	deadline := big.NewInt(9999999999)         // 演示用

	// Step1: Approve
	approveGas, approvePrice, approveEth, err := EstimateApproveEthCost(client, from, tokenIn, router, amountIn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Approve 预计: gasLimit=%d gasPrice=%s wei, 费用=%s ETH\n", approveGas, approvePrice, approveEth.Text('f', 18))

	// Step2: Swap
	swapGas, swapPrice, swapEth, err := EstimateSwapEthCost(client, from, router, []common.Address{tokenIn, tokenOut}, amountIn, amountOutMin, deadline)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Swap 预计: gasLimit=%d gasPrice=%s wei, 费用=%s ETH\n", swapGas, swapPrice, swapEth.Text('f', 18))

	// 总费用
	totalEth := new(big.Float).Add(approveEth, swapEth)
	fmt.Printf("执行一笔代币买卖总费用: %s ETH\n", totalEth.Text('f', 18))

	return "", nil
}
