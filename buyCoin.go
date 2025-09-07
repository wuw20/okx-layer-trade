package main

import "github.com/ethereum/go-ethereum/ethclient"

// Buy 买代币
func Buy(tradeCoinParam TradeCoinParam, client *ethclient.Client) (string, error) {

	// 2、获取当前账户下所有的okb eth 代币余额
	tokenAddressMap := make(map[string]string)
	tokenAddressMap["OKB"] = OKBAddress
	tokenAddressMap["ETH"] = ETHAddress

	return "", nil
}
