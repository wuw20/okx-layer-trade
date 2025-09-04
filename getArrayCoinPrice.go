package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math"
	"math/big"
	"strings"
)

// GetArrayCoinPrice 批量查询代币价格
func (f *DefaultCoinPriceServer) GetArrayCoinPrice(client *ethclient.Client, coinAddressArray []string) (map[string]CoinPrice, error) {

	// 传入数组为空返回 null
	if len(coinAddressArray) == 0 {
		fmt.Printf("传入查询代币数组为空！！！")
		return nil, nil
	}

	resultMap := make(map[string]CoinPrice)

	// 循环查询 后可以改为并发查
	for _, coinAddress := range coinAddressArray {

		fmt.Printf("当前查询价格代币的代币地址: %s\n", coinAddress)
		pairAddress := common.HexToAddress(coinAddress)
		parsedABI, err := abi.JSON(strings.NewReader(pairABI))
		if err != nil {
			fmt.Printf("解析ABI失败：%v\\n", err)
			continue
		}

		// 调用合约
		callOpts := &bind.CallOpts{Context: context.Background()}
		// 用 BoundContract.Call + struct 接收
		contract := bind.NewBoundContract(pairAddress, parsedABI, client, client, client)

		var r Reserves
		var callResults = []any{&r.Reserve0, &r.Reserve1, &r.BlockTimestampLast}
		if err := contract.Call(callOpts, &callResults, "getReserves"); err != nil {
			log.Printf("getReserves 调用失败 (pair %s): %v\n", coinAddress, err)
			continue
		}
		fmt.Printf("Reserve0: %s\nReserve1: %s\nTs: %d\n", r.Reserve0, r.Reserve1, r.BlockTimestampLast)

		var token0Addr common.Address
		token0AddrAny := []any{&token0Addr}
		if err := contract.Call(callOpts, &token0AddrAny, "token0"); err != nil {
			log.Printf("token0 调用失败 (pair %s): %v\n", coinAddress, err)
			continue
		}

		var token1Addr common.Address
		token1AddrAny := []any{&token1Addr}
		if err := contract.Call(callOpts, &token1AddrAny, "token1"); err != nil {
			log.Printf("token1 调用失败 (pair %s): %v\n", coinAddress, err)
			continue
		}

		decimals0, err := getTokenDecimals(client, token0Addr)
		if err != nil {
			log.Printf("获取token0 decimals失败 (token %s): %v\n", token0Addr.Hex(), err)
			continue
		}
		decimals1, err := getTokenDecimals(client, token1Addr)
		if err != nil {
			log.Printf("获取token1 decimals失败 (token %s): %v\n", token1Addr.Hex(), err)
			continue
		}

		if r.Reserve0 == nil || r.Reserve0.Sign() == 0 {
			fmt.Printf("Reserve0 为 0 或 nil (%s)，无法计算价格\n", coinAddress)
			continue
		}

		// 转换储备量为浮点值，考虑decimals
		reserve0Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve0), big.NewFloat(math.Pow10(int(decimals0))))
		reserve1Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve1), big.NewFloat(math.Pow10(int(decimals1))))

		var price = new(big.Float).Quo(reserve1Float, reserve0Float)
		fmt.Printf("链上价格(估): %s\n", price.Text('f', 18))

		coinPrice := CoinPrice{tokenContractAddress: coinAddress, coinPrice: price, blockTimestampLast: r.BlockTimestampLast}
		resultMap[coinAddress] = coinPrice
	}
	return resultMap, nil
}
