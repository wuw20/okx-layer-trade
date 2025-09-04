package main

import "github.com/ethereum/go-ethereum/ethclient"

type DefaultCoinPriceServer struct{}

// CoinPriceServer 代币查询接口
type CoinPriceServer interface {

	// GetCoinPrice 查询单个代币价格
	GetCoinPrice(client *ethclient.Client, coinAddress string) (map[string]CoinPrice, error)

	// GetCoinArrayPrice 批量查询代币价格
	GetCoinArrayPrice(client *ethclient.Client, coinAddressArray []string) (map[string]CoinPrice, error)
}
