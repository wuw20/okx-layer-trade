package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strings"
)

// EstimateSwapEthCost 计算eth消耗
func EstimateSwapEthCost(client *ethclient.Client, from, router common.Address, path []common.Address, amountIn, amountOutMin, deadline *big.Int) (uint64, *big.Int, *big.Float, error) {
	routerABI, _ := abi.JSON(strings.NewReader(`[{"name":"swapExactTokensForTokens","type":"function","inputs":[{"name":"amountIn","type":"uint256"},{"name":"amountOutMin","type":"uint256"},{"name":"path","type":"address[]"},{"name":"to","type":"address"},{"name":"deadline","type":"uint256"}],"outputs":[{"type":"uint256[]"}]}]`))
	data, _ := routerABI.Pack("swapExactTokensForTokens", amountIn, amountOutMin, path, from, deadline)

	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		From: from,
		To:   &router,
		Data: data,
	})
	if err != nil {
		return 0, nil, nil, fmt.Errorf("swap estimate: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return 0, nil, nil, fmt.Errorf("gasPrice: %w", err)
	}

	totalWei := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
	totalEth := new(big.Float).Quo(new(big.Float).SetInt(totalWei), big.NewFloat(1e18))
	return gasLimit, gasPrice, totalEth, nil
}
