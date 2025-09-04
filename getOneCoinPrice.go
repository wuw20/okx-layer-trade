package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math"
	"math/big"
	"strings"
)

// GetCoinPrice 查询单个代币价格
func (f *DefaultCoinPriceServer) GetCoinPrice(client *ethclient.Client, coinAddress string) (map[string]CoinBalance, error) {

	if len(coinAddress) == 0 {
		fmt.Printf("传入的代币地址是空！！！")
		return nil, nil
	}

	resultMap := make(map[string]CoinBalance)

	fmt.Printf("当前查询价格代币的代币地址: %s\n", coinAddress)
	pairAddress := common.HexToAddress(coinAddress)
	parsedABI, err := abi.JSON(strings.NewReader(pairABI))
	if err != nil {
		fmt.Printf("解析ABI失败：%v\\n", err)
		return nil, fmt.Errorf("解析ABI失败！！")
	}

	// 调用合约
	callOpts := &bind.CallOpts{Context: context.Background()}
	// 用 BoundContract.Call + struct 接收
	contract := bind.NewBoundContract(pairAddress, parsedABI, client, client, client)

	var r Reserves
	var callResults = []any{&r.Reserve0, &r.Reserve1, &r.BlockTimestampLast}
	if err := contract.Call(callOpts, &callResults, "getReserves"); err != nil {
		fmt.Printf(" (pair %s): %v\n", coinAddress, err)
		return nil, fmt.Errorf("getReserves 调用失败！！！")

	}
	fmt.Printf("Reserve0: %s\nReserve1: %s\nTs: %d\n", r.Reserve0, r.Reserve1, r.BlockTimestampLast)

	var token0Addr, token1Addr common.Address
	token0AddrAny := []any{&token0Addr}
	if err := contract.Call(callOpts, &token0AddrAny, "token0"); err != nil {
		fmt.Printf("token0 调用失败 (pair %s): %v\n", coinAddress, err)
		return nil, fmt.Errorf("token0 调用失败！！！")
	}

	token1AddrAny := []any{&token1Addr}
	if err := contract.Call(callOpts, &token1AddrAny, "token1"); err != nil {
		fmt.Printf("token1 调用失败 (pair %s): %v\n", coinAddress, err)
		return nil, fmt.Errorf("token1 调用失败！！！")
	}

	decimals0, err := getTokenDecimals(client, token0Addr)
	if err != nil {
		fmt.Printf("获取token0 decimals失败 (token %s): %v\n", token0Addr.Hex(), err)
		return nil, fmt.Errorf("获取token0 decimals失败！！！")
	}

	decimals1, err := getTokenDecimals(client, token1Addr)
	if err != nil {
		fmt.Printf("获取token1 decimals失败 (token %s): %v\n", token1Addr.Hex(), err)
		return nil, fmt.Errorf("获取token1 decimals失败！！！")
	}

	if r.Reserve0 == nil || r.Reserve0.Sign() == 0 {
		fmt.Printf("Reserve0 为 0 或 nil (%s)，无法计算价格\n", coinAddress)
		return nil, fmt.Errorf("reserve0 为 0 或 nil！！！")
	}

	// 固定计算代币/OKB 价格
	okbAddress := common.HexToAddress(OKBAddress)

	// 固定计算代币/USDT 价格
	usdtAddress := common.HexToAddress(USDTAddress)

	var price *big.Float
	if token1Addr == okbAddress {
		reserve0Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve0), big.NewFloat(math.Pow10(int(decimals0))))
		reserve1Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve1), big.NewFloat(math.Pow10(int(decimals1))))
		price = new(big.Float).Quo(reserve1Float, reserve0Float)
		fmt.Printf("价格: 1 token0 ≈ %s OKB\n", price.Text('f', 18))
		resultMap[token0Addr.Hex()] = CoinBalance{
			coinAddress:        token0Addr.Hex(),
			coinBalanceInOKB:   price,
			blockTimestampLast: r.BlockTimestampLast,
		}
	} else if token0Addr == okbAddress { // 代币在 token1，OKB 在 token0
		reserve0Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve0), big.NewFloat(math.Pow10(int(decimals0))))
		reserve1Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve1), big.NewFloat(math.Pow10(int(decimals1))))
		price = new(big.Float).Quo(reserve0Float, reserve1Float)
		fmt.Printf("价格: 1 token1 ≈ %s OKB\n", price.Text('f', 18))
		resultMap[token1Addr.Hex()] = CoinBalance{
			coinAddress:        token1Addr.Hex(),
			coinBalanceInOKB:   price,
			blockTimestampLast: r.BlockTimestampLast,
		}
	} else if token1Addr == usdtAddress {
		reserve0Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve0), big.NewFloat(math.Pow10(int(decimals0))))
		reserve1Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve1), big.NewFloat(math.Pow10(int(decimals1))))
		price = new(big.Float).Quo(reserve1Float, reserve0Float)
		fmt.Printf("价格: 1 token0 ≈ %s OKB\n", price.Text('f', 18))
		resultMap[token0Addr.Hex()] = CoinBalance{
			coinAddress:        token0Addr.Hex(),
			coinBalanceInUSDT:  price,
			blockTimestampLast: r.BlockTimestampLast,
		}
	} else if token0Addr == usdtAddress {
		reserve0Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve0), big.NewFloat(math.Pow10(int(decimals0))))
		reserve1Float := new(big.Float).Quo(new(big.Float).SetInt(r.Reserve1), big.NewFloat(math.Pow10(int(decimals1))))
		price = new(big.Float).Quo(reserve0Float, reserve1Float)
		fmt.Printf("价格: 1 token1 ≈ %s OKB\n", price.Text('f', 18))
		resultMap[token1Addr.Hex()] = CoinBalance{
			coinAddress:        token1Addr.Hex(),
			coinBalanceInUSDT:  price,
			blockTimestampLast: r.BlockTimestampLast,
		}
	} else {
		return nil, fmt.Errorf("pair %s 不包含 OKB，无法计算代币/OKB 价格", pairAddress)
	}

	return resultMap, nil
}

// 获取token精度
func getTokenDecimals(client *ethclient.Client, tokenAddress common.Address) (uint8, error) {
	parsedERC20ABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return 0, fmt.Errorf("解析ERC20 ABI失败: %v", err)
	}
	contract := bind.NewBoundContract(tokenAddress, parsedERC20ABI, client, client, client)
	callOpts := &bind.CallOpts{Context: context.Background()}

	var decimals uint8
	decimalsAny := []any{&decimals}
	if err := contract.Call(callOpts, &decimalsAny, "decimals"); err != nil {
		return 0, err
	}
	return decimals, nil
}
