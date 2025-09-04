package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
)

// RpcUrl 主网 RPC
const RpcUrl = "https://xlayerrpc.okx.com"

// PrivateKeyHex 你的测试私钥
const PrivateKeyHex = "你的私钥"

// WalletAddress 钱包地址
const WalletAddress = "0xYourAddress"

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

	// 3、获取当前账户下所有的代币余额 指定使用okb
	var tokenAddressArray []string
	tokenAddressArray = append(tokenAddressArray, "0xe538905cf8410324e03a5a23c1c177a474d59b2b", "0x5a77f1443d16ee5761d310e38b62f77f726bc71c") // okb地址 eth地址
	tokenBalanceMap, err := GetMultiERC20Balances(client, WalletAddress, tokenAddressArray)
	if err != nil || len(tokenBalanceMap) == 0 {
		log.Fatal("获取钱包下面代币余额失败！！！")
	}
	fmt.Println("获取到代币余额数组", tokenBalanceMap)

	// 4、发起买卖交易

}
