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

// EstimateApproveEthCost 计算approve 手续费
func EstimateApproveEthCost(client *ethclient.Client, from, token, spender common.Address, amount *big.Int) (uint64, *big.Int, *big.Float, error) {

	erc20ABI, _ := abi.JSON(strings.NewReader(`[{"name":"approve","type":"function","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]}]`))
	data, _ := erc20ABI.Pack("approve", spender, amount)

	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		From: from,
		To:   &token,
		Data: data,
	})
	if err != nil {
		return 0, nil, nil, fmt.Errorf("approve estimate: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return 0, nil, nil, fmt.Errorf("gasPrice: %w", err)
	}

	totalWei := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
	totalEth := new(big.Float).Quo(new(big.Float).SetInt(totalWei), big.NewFloat(1e18))
	return gasLimit, gasPrice, totalEth, nil
}
