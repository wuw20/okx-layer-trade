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

// WalletAddress 当前钱包余额
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
	var coinAddressArray []string
	coinAddressArray = append(coinAddressArray, "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e") // 代币地址

	var coinPriceArray []CoinPrice
	coinPriceArray = GetCoinArrayPrice(client, coinAddressArray)
	if len(coinPriceArray) == 0 {
		fmt.Println("批量查询代币价格金额返回空数组！！！")
		log.Fatal("返回无效数组")
	}

	// 3、获取当前账户下所有的代币余额 指定使用okb
	var tokenAddressArray []string
	tokenAddressArray = append(tokenAddressArray, "0xe538905cf8410324e03a5a23c1c177a474d59b2b") // okb地址

	var tokenBalanceArray []TokenBalance
	tokenBalanceArray, err = GetMultiERC20Balances(client, WalletAddress, tokenAddressArray)
	if err != nil {
		log.Fatal("获取钱包下面代币余额失败")
	}
	fmt.Println("获取到代币余额数组", tokenBalanceArray)

	// 4、获取交易对

	// 5、获取当前区块gas建议费用
	gasPrice, _ := client.SuggestGasPrice(ctx)
	fmt.Println("建议 gas price:", gasPrice)

	// 6、发起交易 返回txHash 作为唯一交易凭证
	//txHash := VerifyAndTradeService(VerifyAndTrade{
	//	privateKeyHex: PrivateKeyHex,
	//	ctx:           ctx,
	//	client:        client,
	//	targetAddress: address,                // 待确认
	//	tradeGasFee:   big.NewInt(1000000),    // 写死待改
	//	tradePrice:    big.NewInt(1000000000), // 写死待改
	//	chainID:       big.NewInt(196),
	//	gasLimit:      uint64(100000000000),
	//})
	//fmt.Println("交易成功获取到的交易凭证txHash：", txHash)
}
