package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math/big"
)

func main() {

	// 1、连接连接 RPC 节点
	var ctx = context.Background()
	client, err := ethclient.DialContext(ctx, RpcUrl)
	if err != nil {
		log.Fatal("连接 RPC 失败:", err)
	}
	fmt.Println("已连接到 X Layer 节点")

	// 2、获取代币当前价格 代币地址 可以当作配置传入
	var coinAddress = "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e" // 代币地址
	server := &DefaultCoinPriceServer{}
	coinPriceMap, err := server.GetCoinPrice(client, coinAddress)
	if coinPriceMap == nil || err != nil {
		fmt.Println(err)
		log.Fatal("查询代币价格失败！！！")
	}

	// 3、获取当前账户下所有的okb eth 代币余额
	tokenAddressMap := make(map[string]string)
	tokenAddressMap["OKB"] = OKBAddress
	tokenAddressMap["ETH"] = ETHAddress
	tokenBalanceMap, err := GetMultiERC20Balances(client, WalletAddress, tokenAddressMap)
	if err != nil || len(tokenBalanceMap) == 0 {
		log.Fatal("获取钱包下面代币余额失败！！！")
	}
	fmt.Println("获取到钱包下代币余额结果为", tokenBalanceMap)

	// 4、单独校验 OKB或者ETH余额为0无法交易
	for coinKey, coinBalance := range tokenBalanceMap {
		if coinBalance.walletCoinBalance.Cmp(big.NewFloat(0)) <= 0 {
			fmt.Println("该钱包下OKB账户余额为0，无法进行交易！！！", coinKey)
			log.Fatal("该钱包下OKB账户余额为0，无法进行交易！！！")
		}
	}

	// 5、发起买卖交易

	defer client.Close()
}
