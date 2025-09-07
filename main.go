package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
)

func main() {

	// 1、连接连接 RPC 节点
	var ctx = context.Background()
	client, err := ethclient.DialContext(ctx, RpcUrl)
	if err != nil {
		log.Fatal("连接 RPC 失败:", err)
	}
	fmt.Println("已连接到 X Layer 节点")

	// 初始化值 需要变成入参
	tradeCoinParam := TradeCoinParam{
		tradeApiKey:        "apiKeyAddress",
		tradeType:          "buy",
		tradeCoinAddress:   "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e",
		tradeWalletAddress: "0xYourAddress",
		tradeSlippageFee:   "0.01",
		tradeQuantity:      "1.5",
	}

	// 2、获取当前账户下所有的okb eth 代币余额
	tokenAddressMap := make(map[string]string)
	tokenAddressMap["OKB"] = OKBAddress
	tokenAddressMap["ETH"] = ETHAddress

	// 卖 需要知道当前账户余额
	if tradeCoinParam.tradeType == "sell" {
		tokenAddressMap["tradeCoin"] = tradeCoinParam.tradeCoinAddress
	}

	// 4、发起买卖交易

	defer client.Close()
}
