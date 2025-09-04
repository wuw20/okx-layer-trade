package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strings"
)

// GetMultiERC20Balances 查询账户下面的代币余额
func GetMultiERC20Balances(client *ethclient.Client, walletAddr string, tokenAddresses []string) (map[string]TokenBalance, error) {
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("解析ERC20 ABI失败: %v", err)
	}
	callOpts := &bind.CallOpts{Context: context.Background()}

	resultMap := make(map[string]TokenBalance)
	for _, tokenAddr := range tokenAddresses {
		contract := bind.NewBoundContract(common.HexToAddress(tokenAddr), parsedABI, client, client, client)

		// 1. balanceOf
		var balance *big.Int
		balanceAny := []any{&balance}
		if err := contract.Call(callOpts, &balanceAny, "balanceOf", common.HexToAddress(walletAddr)); err != nil {
			return nil, fmt.Errorf("调用balanceOf失败 (%s): %v", tokenAddr, err)
		}

		// 2. decimals
		var decimals uint8
		decimalsAnt := []any{&decimals}
		if err := contract.Call(callOpts, &decimalsAnt, "decimals"); err != nil {
			return nil, fmt.Errorf("调用decimals失败 (%s): %v", tokenAddr, err)
		}

		// 3. 转换成人类可读
		divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
		balanceFloat := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt(divisor))

		tokenBalance := TokenBalance{
			tokenAddress: tokenAddr,
			balance:      balanceFloat,
		}
		resultMap[tokenAddr] = tokenBalance
	}
	return resultMap, nil
}
